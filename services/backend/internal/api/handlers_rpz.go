package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type RPZConfig struct {
	Enabled        bool       `json:"enabled"`
	MasterServers  string     `json:"master_servers"`
	ZoneName       string     `json:"zone_name"`
	LastSync       *time.Time `json:"last_sync"`
	LastSyncStatus string     `json:"last_sync_status"`
	LastSyncError  string     `json:"last_sync_error"`
	DomainCount    int        `json:"domain_count"`
	FileSizeBytes  int64      `json:"file_size_bytes"`
	SyncDurationMs int        `json:"sync_duration_ms"`
}

func (s *Server) getRPZConfig() RPZConfig {
	cfg := RPZConfig{
		MasterServers: "103.154.123.130,139.255.196.202",
		ZoneName:      "trustpositifkominfo",
	}
	ctx := context.Background()
	s.pg.QueryRow(ctx,
		`SELECT enabled, master_servers, zone_name, last_sync, last_sync_status, last_sync_error,
		        domain_count, file_size_bytes, sync_duration_ms
		 FROM rpz_config WHERE id = 1`,
	).Scan(&cfg.Enabled, &cfg.MasterServers, &cfg.ZoneName, &cfg.LastSync, &cfg.LastSyncStatus,
		&cfg.LastSyncError, &cfg.DomainCount, &cfg.FileSizeBytes, &cfg.SyncDurationMs)
	return cfg
}

func (s *Server) handleGetRPZConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.getRPZConfig())
}

func (s *Server) handleUpdateRPZConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled       *bool   `json:"enabled"`
		MasterServers *string `json:"master_servers"`
		ZoneName      *string `json:"zone_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	// Seed config row if not exists
	s.pg.Exec(ctx, `INSERT INTO rpz_config (id) VALUES (1) ON CONFLICT DO NOTHING`)

	if req.Enabled != nil {
		s.pg.Exec(ctx, "UPDATE rpz_config SET enabled = $1, updated_at = NOW() WHERE id = 1", *req.Enabled)

		// If disabling, regenerate kresd config without RPZ and restart
		if !*req.Enabled {
			s.regenerateKresdConfig(false)
			if name := findContainerName("kresd"); name != "" {
				exec.Command("docker", "restart", name).Run()
			}
		}
	}
	if req.MasterServers != nil {
		s.pg.Exec(ctx, "UPDATE rpz_config SET master_servers = $1, updated_at = NOW() WHERE id = 1", *req.MasterServers)
	}
	if req.ZoneName != nil {
		s.pg.Exec(ctx, "UPDATE rpz_config SET zone_name = $1, updated_at = NOW() WHERE id = 1", *req.ZoneName)
	}

	writeJSON(w, map[string]string{"message": "RPZ config updated"})
}

// handleRPZSync triggers an AXFR sync from the master servers
func (s *Server) handleRPZSync(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	sendEvent := func(msg string) {
		fmt.Fprintf(w, "data: %s\n\n", msg)
		flusher.Flush()
	}

	cfg := s.getRPZConfig()
	masters := strings.Split(cfg.MasterServers, ",")
	if len(masters) == 0 {
		sendEvent("[ERROR] No master servers configured")
		fmt.Fprintf(w, "event: error\ndata: no masters\n\n")
		flusher.Flush()
		return
	}

	sendEvent("[INFO] Starting RPZ zone transfer...")
	startTime := time.Now()

	// Try each master server
	var axfrOutput []byte
	var axfrErr error
	var usedMaster string

	for _, master := range masters {
		master = strings.TrimSpace(master)
		if master == "" {
			continue
		}
		sendEvent(fmt.Sprintf("[INFO] Trying AXFR from %s...", master))

		cmd := exec.CommandContext(r.Context(),
			"dig", "AXFR", fmt.Sprintf("@%s", master), cfg.ZoneName, "+noidnout", "+onesoa")
		axfrOutput, axfrErr = cmd.CombinedOutput()
		if axfrErr == nil && len(axfrOutput) > 100 {
			usedMaster = master
			break
		}
		sendEvent(fmt.Sprintf("[WARN] Failed from %s: %v", master, axfrErr))
	}

	if usedMaster == "" {
		errMsg := "all master servers failed"
		if axfrErr != nil {
			errMsg = axfrErr.Error()
		}
		sendEvent(fmt.Sprintf("[ERROR] Zone transfer failed: %s", errMsg))
		s.updateRPZSyncStatus("error", errMsg, 0, 0, 0)
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", errMsg)
		flusher.Flush()
		return
	}

	sendEvent(fmt.Sprintf("[OK] Zone transfer from %s successful", usedMaster))

	// Save raw zone file
	rpzDir := filepath.Join(s.cfg.ProjectDir, "config", "kresd")
	rpzFile := filepath.Join(rpzDir, "rpz.zone")
	os.WriteFile(rpzFile, axfrOutput, 0644)

	// Parse zone to extract blocked domains
	sendEvent("[INFO] Parsing zone file...")
	domains := parseRPZZone(string(axfrOutput))
	sendEvent(fmt.Sprintf("[OK] Found %d blocked domains", len(domains)))

	// Save parsed domains to a simple blocklist file
	blocklistFile := filepath.Join(rpzDir, "rpz-domains.txt")
	var sb strings.Builder
	for _, d := range domains {
		sb.WriteString(d)
		sb.WriteString("\n")
	}
	os.WriteFile(blocklistFile, []byte(sb.String()), 0644)

	duration := time.Since(startTime)
	fileInfo, _ := os.Stat(rpzFile)
	fileSize := int64(0)
	if fileInfo != nil {
		fileSize = fileInfo.Size()
	}

	// Update DB
	s.updateRPZSyncStatus("success", "", len(domains), fileSize, int(duration.Milliseconds()))

	sendEvent(fmt.Sprintf("[OK] Sync complete: %d domains, %.1f MB, %dms",
		len(domains), float64(fileSize)/1024/1024, duration.Milliseconds()))

	// If RPZ is enabled, regenerate kresd config and restart
	if cfg.Enabled {
		sendEvent("[INFO] Applying to DNS resolver...")
		s.regenerateKresdConfig(true)
		if name := findContainerName("kresd"); name != "" {
			exec.Command("docker", "restart", name).Run()
		}
		sendEvent("[OK] DNS resolver restarted with updated RPZ")
	} else {
		sendEvent("[INFO] RPZ is disabled — sync saved but not applied")
	}

	fmt.Fprintf(w, "event: done\ndata: sync complete\n\n")
	flusher.Flush()
}

func (s *Server) handleRPZStats(w http.ResponseWriter, r *http.Request) {
	cfg := s.getRPZConfig()

	// Check if zone file exists
	rpzFile := filepath.Join(s.cfg.ProjectDir, "config", "kresd", "rpz.zone")
	fileExists := false
	if _, err := os.Stat(rpzFile); err == nil {
		fileExists = true
	}

	writeJSON(w, map[string]interface{}{
		"config":      cfg,
		"file_exists": fileExists,
	})
}

func (s *Server) updateRPZSyncStatus(status, errMsg string, domainCount int, fileSize int64, durationMs int) {
	ctx := context.Background()
	s.pg.Exec(ctx, `INSERT INTO rpz_config (id) VALUES (1) ON CONFLICT DO NOTHING`)
	s.pg.Exec(ctx,
		`UPDATE rpz_config SET last_sync = NOW(), last_sync_status = $1, last_sync_error = $2,
		 domain_count = $3, file_size_bytes = $4, sync_duration_ms = $5, updated_at = NOW() WHERE id = 1`,
		status, errMsg, domainCount, fileSize, durationMs)
}

// regenerateKresdConfig rebuilds kresd config with or without RPZ domains
func (s *Server) regenerateKresdConfig(includeRPZ bool) {
	projectDir := s.cfg.ProjectDir
	templatePath := filepath.Join(projectDir, "config/kresd/config.yaml.template")
	configPath := filepath.Join(projectDir, "config/kresd/config.yaml")

	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		log.Printf("Failed to read kresd template: %v", err)
		return
	}

	envVars := loadEnvFile(filepath.Join(projectDir, ".env"))
	serverIP := envVars["SERVER_IP"]
	if serverIP == "" {
		serverIP = "0.0.0.0"
	}
	cacheSize := envVars["CACHE_SIZE"]
	if cacheSize == "" {
		cacheSize = "8G"
	}

	// Build subnet views
	var subnetViews strings.Builder
	if subnets := envVars["ALLOWED_SUBNETS"]; subnets != "" {
		for _, subnet := range strings.Split(subnets, ",") {
			subnet = strings.TrimSpace(subnet)
			if subnet != "" {
				subnetViews.WriteString(fmt.Sprintf("  - subnets: ['%s']\n    answer: allow\n", subnet))
			}
		}
	}

	// Build local-data: custom filter rules + RPZ domains
	var localData strings.Builder
	ctx := context.Background()

	// Custom filter rules from DB
	rows, err := s.pg.Query(ctx,
		"SELECT domain FROM filter_rules WHERE enabled = true AND action = 'block' ORDER BY domain")
	customDomains := []string{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var d string
			rows.Scan(&d)
			customDomains = append(customDomains, d)
		}
	}

	// RPZ domains from file
	rpzDomains := []string{}
	if includeRPZ {
		rpzFile := filepath.Join(projectDir, "config/kresd/rpz-domains.txt")
		if data, err := os.ReadFile(rpzFile); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				d := strings.TrimSpace(scanner.Text())
				if d != "" {
					rpzDomains = append(rpzDomains, d)
				}
			}
		}
	}

	totalDomains := len(customDomains) + len(rpzDomains)
	if totalDomains > 0 {
		localData.WriteString("local-data:\n")
		localData.WriteString("  records:\n")

		writeDomain := func(d string) {
			localData.WriteString(fmt.Sprintf("    - owner: %s.\n      ttl: 60\n      rdata: '%s'\n", d, serverIP))
		}

		for _, d := range customDomains {
			writeDomain(d)
			if !strings.HasPrefix(d, "www.") {
				writeDomain("www." + d)
			}
		}
		for _, d := range rpzDomains {
			writeDomain(d)
		}
	}

	log.Printf("Regenerating kresd config: %d custom + %d RPZ = %d domains",
		len(customDomains), len(rpzDomains), totalDomains)

	config := string(templateData)
	config = strings.ReplaceAll(config, "__CACHE_SIZE__", cacheSize)
	config = strings.ReplaceAll(config, "__SUBNET_VIEWS__", subnetViews.String())
	config = strings.ReplaceAll(config, "__LOCAL_DATA__", localData.String())

	os.WriteFile(configPath, []byte(config), 0644)
}

// parseRPZZone extracts blocked domain names from an RPZ zone file
func parseRPZZone(zone string) []string {
	seen := map[string]bool{}
	var domains []string

	scanner := bufio.NewScanner(strings.NewReader(zone))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "$") {
			continue
		}

		// RPZ format: "domain.com CNAME ." or "domain.com IN CNAME rpz-passthru."
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		domain := strings.ToLower(fields[0])
		domain = strings.TrimSuffix(domain, ".")

		// Skip SOA, NS, and RPZ meta records
		if strings.Contains(domain, "trustpositifkominfo") ||
			domain == "@" || domain == "" {
			continue
		}

		// Skip rpz-passthru entries (whitelisted)
		lineUpper := strings.ToUpper(line)
		if strings.Contains(lineUpper, "RPZ-PASSTHRU") {
			continue
		}

		// Check it looks like a real domain
		if !strings.Contains(domain, ".") {
			continue
		}

		// Remove trailing RPZ zone suffix if present
		if idx := strings.Index(domain, ".trustpositifkominfo"); idx > 0 {
			domain = domain[:idx]
		}

		if !seen[domain] {
			seen[domain] = true
			domains = append(domains, domain)
		}
	}

	return domains
}
