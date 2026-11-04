package api

// handlers_monitor.go — one read-only status document for an external monitor
// (the ISP billing platform, nu.afna.link) to poll.
//
// WHY A SEPARATE ENDPOINT. The cluster agent API only answers on nodes whose
// role is "agent" — the controller (216) cannot be asked — and its data is
// spread over a dozen calls in Prometheus' raw shape. An outside watcher needs
// the same document from every node, cheap to fetch, already reduced to the
// numbers that decide "are customers getting answers" and "is the filter on".
//
// The 4 Nov 2026 outage is the reason ruledb is here: both replicas died with
// MDB_MAP_FULL hours after the zone grew, and the only number that would have
// warned in advance — allocated ruledb bytes against the LMDB mapsize — was
// visible to nobody. `ls` shows the nominal size (= mapsize, sparse file); the
// truth is in st_blocks.
//
// Auth: X-Monitor-Token must equal MONITOR_TOKEN in the project .env. No token
// configured = endpoint off (404). Nothing here writes or restarts anything.

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type monitorStatus struct {
	GeneratedAt time.Time `json:"generated_at"`
	Node        struct {
		Name      string `json:"name"`
		Domain    string `json:"domain"`
		ServerIP  string `json:"server_ip"`
		Role      string `json:"role"`
		Version   string `json:"version"`
		GitCommit string `json:"git_commit"`
	} `json:"node"`
	Resolver struct {
		// Answering = dnsdist on :53 (the path customers use) resolved the control name.
		Answering    bool     `json:"answering"`
		KresdDirect  bool     `json:"kresd_direct"`
		ProbeMs      int64    `json:"probe_ms"`
		QPS          *float64 `json:"qps"`
		LatencyP50Ms *float64 `json:"latency_p50_ms"`
		CacheHitPct  *float64 `json:"cache_hit_pct"`
	} `json:"resolver"`
	Kresd struct {
		Status       string `json:"status"`
		RestartCount int    `json:"restart_count"`
		StartedAt    string `json:"started_at"`
		// MapFull15m counts MDB_MAP_FULL lines in the last 15 minutes of kresd logs.
		MapFull15m int `json:"map_full_15m"`
	} `json:"kresd"`
	Ruledb struct {
		AllocatedBytes int64 `json:"allocated_bytes"`
		// MapsizeBytes = mapsize the ruledb was last written with, read from the
		// LMDB meta page (mm_mapsize of the newer meta). Neither file size nor the
		// template is it: st_size is the high-water mark (216: 12 GiB sparse; 212
		// after a clean reload: 3 GiB for a 16 GiB map), and the template only
		// applies at the next kresd restart.
		MapsizeBytes           int64    `json:"mapsize_bytes"`
		MapsizeConfiguredBytes int64    `json:"mapsize_configured_bytes"`
		UsedPct                *float64 `json:"used_pct"`
		WarmMarker             bool     `json:"warm_marker"`
	} `json:"ruledb"`
	RPZ struct {
		Enabled          bool       `json:"enabled"`
		ZoneSerial       uint64     `json:"zone_serial"`
		DomainCount      int        `json:"domain_count"`
		LastSync         *time.Time `json:"last_sync"`
		LastSyncStatus   string     `json:"last_sync_status"`
		LastSyncError    string     `json:"last_sync_error"`
		AutoSyncHour     int        `json:"auto_sync_hour"`
		LastVerifyAt     *time.Time `json:"last_verify_at"`
		LastVerifyStatus string     `json:"last_verify_status"`
		LastVerifyDetail string     `json:"last_verify_detail"`
	} `json:"rpz"`
	Services []monitorService `json:"services"`
	System   struct {
		CPUPct      *float64 `json:"cpu_pct"`
		Load1       *float64 `json:"load1"`
		MemUsedB    *float64 `json:"mem_used_bytes"`
		MemTotalB   *float64 `json:"mem_total_bytes"`
		DiskUsedPct *float64 `json:"disk_used_pct"`
	} `json:"system"`
	Alerts []monitorAlert `json:"alerts"`
	Probes []probeItem    `json:"probes"`
}

type monitorService struct {
	Name         string `json:"name"`
	State        string `json:"state"`
	Health       string `json:"health,omitempty"`
	RestartCount int    `json:"restart_count"`
	StartedAt    string `json:"started_at"`
}

type monitorAlert struct {
	Kind  string    `json:"kind"`
	Since time.Time `json:"since"`
}

// The document costs a few docker/dig/prometheus calls; several pollers (the
// platform's worker, a dashboard tab) must not multiply that.
const monitorCacheTTL = 15 * time.Second

var monitorCache struct {
	mu  sync.Mutex
	at  time.Time
	doc []byte
}

var mapsizeRe = regexp.MustCompile(`kr_rules_init\(\s*nil\s*,\s*(\d+)`)

func (s *Server) monitorTokenOK(r *http.Request) (configured, ok bool) {
	want := loadEnvFile(filepath.Join(s.cfg.ProjectDir, ".env"))["MONITOR_TOKEN"]
	if want == "" {
		return false, false
	}
	got := r.Header.Get("X-Monitor-Token")
	return true, got != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (s *Server) handleMonitorStatus(w http.ResponseWriter, r *http.Request) {
	configured, ok := s.monitorTokenOK(r)
	if !configured {
		http.Error(w, `{"error":"monitor API not configured"}`, http.StatusNotFound)
		return
	}
	if !ok {
		http.Error(w, `{"error":"invalid monitor token"}`, http.StatusForbidden)
		return
	}

	monitorCache.mu.Lock()
	defer monitorCache.mu.Unlock()
	if monitorCache.doc == nil || time.Since(monitorCache.at) > monitorCacheTTL {
		doc, err := json.Marshal(s.buildMonitorStatus(r.Context()))
		if err != nil {
			http.Error(w, `{"error":"encode"}`, http.StatusInternalServerError)
			return
		}
		monitorCache.doc, monitorCache.at = doc, time.Now()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(monitorCache.doc)
}

func (s *Server) buildMonitorStatus(ctx context.Context) monitorStatus {
	var st monitorStatus
	st.GeneratedAt = time.Now().UTC()
	env := loadEnvFile(filepath.Join(s.cfg.ProjectDir, ".env"))

	st.Node.Name = env["NODE_NAME"]
	if st.Node.Name == "" {
		st.Node.Name = s.cfg.NodeName
	}
	st.Node.Domain = env["DOMAIN"]
	st.Node.ServerIP = env["SERVER_IP"]
	st.Node.Role = s.getRole()
	st.Node.Version = s.cfg.Version
	if out, err := exec.CommandContext(ctx, "git", "-C", s.cfg.ProjectDir, "rev-parse", "--short", "HEAD").Output(); err == nil {
		st.Node.GitCommit = strings.TrimSpace(string(out))
	}

	// Resolver: dnsdist is what customers hit; kresd direct tells a broken
	// dnsdist (stale backend IP, 12 Aug) apart from a dead resolver.
	t0 := time.Now()
	st.Resolver.Answering, _ = digAnswers("dnsdist", "google.com")
	st.Resolver.ProbeMs = time.Since(t0).Milliseconds()
	st.Resolver.KresdDirect, _ = digAnswers("kresd", "google.com")
	st.Resolver.QPS = s.promScalar(`sum(rate(kresd_query_total[1m]))`)
	st.Resolver.LatencyP50Ms = s.promScalar(`histogram_quantile(0.5, sum(rate(kresd_answer_duration_seconds_bucket[5m])) by (le)) * 1000`)
	if v := s.promScalar(`sum(rate(kresd_cache_hit_total[5m])) / (sum(rate(kresd_cache_hit_total[5m])) + sum(rate(kresd_cache_miss_total[5m]))) * 100`); v != nil {
		st.Resolver.CacheHitPct = v
	}

	st.Services = listMonitorServices(ctx)
	for _, svc := range st.Services {
		if svc.Name == "kresd" {
			st.Kresd.Status = svc.State
			st.Kresd.RestartCount = svc.RestartCount
			st.Kresd.StartedAt = svc.StartedAt
		}
	}
	st.Kresd.MapFull15m = countKresdLog(ctx, "MDB_MAP_FULL", "15m")

	// Ruledb: allocated blocks, not st_size — the file is sparse and its
	// nominal size is always the mapsize.
	var fi syscall.Stat_t
	if err := syscall.Stat("/ruledb/data.mdb", &fi); err == nil {
		st.Ruledb.AllocatedBytes = fi.Blocks * 512
	}
	st.Ruledb.MapsizeBytes = lmdbMapsize("/ruledb/data.mdb")
	if _, err := os.Stat("/ruledb/.zone-ok"); err == nil {
		st.Ruledb.WarmMarker = true
	}
	// Configured value (kresd.lua.j2; policy-loader.lua.j2 must match).
	if b, err := os.ReadFile(filepath.Join(s.cfg.ProjectDir, "config", "kresd", "kresd.lua.j2")); err == nil {
		if m := mapsizeRe.FindSubmatch(b); m != nil {
			st.Ruledb.MapsizeConfiguredBytes, _ = strconv.ParseInt(string(m[1]), 10, 64)
		}
	}
	if st.Ruledb.MapsizeBytes == 0 {
		st.Ruledb.MapsizeBytes = st.Ruledb.MapsizeConfiguredBytes
	}
	if st.Ruledb.MapsizeBytes > 0 && st.Ruledb.AllocatedBytes > 0 {
		p := float64(st.Ruledb.AllocatedBytes) / float64(st.Ruledb.MapsizeBytes) * 100
		st.Ruledb.UsedPct = &p
	}

	cfg := s.getRPZConfig()
	st.RPZ.Enabled = cfg.Enabled
	st.RPZ.ZoneSerial = cfg.ZoneSerial
	st.RPZ.DomainCount = cfg.DomainCount
	st.RPZ.LastSync = cfg.LastSync
	st.RPZ.LastSyncStatus = cfg.LastSyncStatus
	st.RPZ.LastSyncError = cfg.LastSyncError
	st.RPZ.AutoSyncHour = cfg.AutoSyncHour
	st.RPZ.LastVerifyAt = cfg.LastVerifyAt
	st.RPZ.LastVerifyStatus = cfg.LastVerifyStatus
	st.RPZ.LastVerifyDetail = cfg.LastVerifyDetail

	st.System.CPUPct = s.promScalar(`100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`)
	st.System.Load1 = s.promScalar(`node_load1`)
	st.System.MemUsedB = s.promScalar(`node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes`)
	st.System.MemTotalB = s.promScalar(`node_memory_MemTotal_bytes`)
	st.System.DiskUsedPct = s.promScalar(`100 - ((node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"}) * 100)`)

	sysAlerts.mu.Lock()
	for kind, since := range sysAlerts.firing {
		st.Alerts = append(st.Alerts, monitorAlert{Kind: kind, Since: since})
	}
	sysAlerts.mu.Unlock()
	if st.Alerts == nil {
		st.Alerts = []monitorAlert{}
	}
	st.Probes = fleetProbe.snapshot()
	return st
}

// promScalar returns the first sample of an instant query, or nil when
// Prometheus is down or the series is empty — "unknown" must not read as 0.
func (s *Server) promScalar(query string) *float64 {
	raw, err := s.promInstantQuery(query)
	if err != nil {
		return nil
	}
	var resp struct {
		Data struct {
			Result []struct {
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &resp) != nil || len(resp.Data.Result) == 0 || len(resp.Data.Result[0].Value) < 2 {
		return nil
	}
	str, _ := resp.Data.Result[0].Value[1].(string)
	v, err := strconv.ParseFloat(str, 64)
	if err != nil || v != v { // NaN: 0/0 with no traffic
		return nil
	}
	return &v
}

// listMonitorServices: state + restart count per compose service. A container
// that is "Up 4 minutes" with RestartCount 40 is crash-looping — the exact
// face kresd showed on 4 Nov — so the count matters more than the state.
func listMonitorServices(ctx context.Context) []monitorService {
	args := []string{"ps", "-a", "--format", `{{.Label "com.docker.compose.service"}}|{{.ID}}|{{.State}}|{{.Status}}`}
	if p := detectProjectName(); p != "" {
		args = append(args, "--filter", "label=com.docker.compose.project="+p)
	}
	out, err := exec.CommandContext(ctx, "docker", args...).Output()
	byName := map[string]*monitorService{}
	var ids []string
	idToName := map[string]string{}
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			parts := strings.SplitN(line, "|", 4)
			if len(parts) < 4 || parts[0] == "" {
				continue
			}
			svc := &monitorService{Name: parts[0], State: strings.ToLower(parts[2])}
			detail := strings.ToLower(parts[3])
			if strings.Contains(detail, "(healthy)") {
				svc.Health = "healthy"
			} else if strings.Contains(detail, "(unhealthy)") {
				svc.Health = "unhealthy"
			}
			byName[parts[0]] = svc
			ids = append(ids, parts[1])
			idToName[parts[1]] = parts[0]
		}
	}
	if len(ids) > 0 {
		iargs := append([]string{"inspect", "--format", "{{.Id}}|{{.RestartCount}}|{{.State.StartedAt}}"}, ids...)
		if out, err := exec.CommandContext(ctx, "docker", iargs...).Output(); err == nil {
			for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				parts := strings.SplitN(line, "|", 3)
				if len(parts) < 3 {
					continue
				}
				for short, name := range idToName {
					if strings.HasPrefix(parts[0], short) {
						byName[name].RestartCount, _ = strconv.Atoi(parts[1])
						byName[name].StartedAt = parts[2]
					}
				}
			}
		}
	}
	res := make([]monitorService, 0, len(managedServices))
	for _, name := range managedServices {
		if svc, ok := byName[name]; ok {
			res = append(res, *svc)
		} else {
			res = append(res, monitorService{Name: name, State: "missing"})
		}
	}
	return res
}

// countKresdLog counts log lines containing needle within the window.
func countKresdLog(ctx context.Context, needle, since string) int {
	args := []string{"ps", "-q", "--filter", "label=com.docker.compose.service=kresd"}
	if p := detectProjectName(); p != "" {
		args = append(args, "--filter", "label=com.docker.compose.project="+p)
	}
	idOut, err := exec.CommandContext(ctx, "docker", args...).Output()
	id := strings.TrimSpace(strings.Split(strings.TrimSpace(string(idOut)), "\n")[0])
	if err != nil || id == "" {
		return 0
	}
	lctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(lctx, "docker", "logs", "--since", since, id)
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return 0
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return 0
	}
	n := 0
	sc := bufio.NewScanner(pipe)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		if strings.Contains(sc.Text(), needle) {
			n++
		}
	}
	cmd.Wait()
	return n
}

// lmdbMapsize reads mm_mapsize from the newer of LMDB's two meta pages.
// Layout (LMDB 0.9, 64-bit): page header 16 B, then MDB_meta: magic u32,
// version u32, address 8, mapsize 8 @32, dbs[2] 96, last_pg 8, txnid 8 @144.
// Returns 0 when the file is missing or not LMDB.
func lmdbMapsize(path string) int64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	var best, bestTxn uint64
	buf := make([]byte, 152)
	for _, off := range []int64{0, 4096} {
		if _, err := f.ReadAt(buf, off); err != nil {
			continue
		}
		if binary.LittleEndian.Uint32(buf[16:]) != 0xBEEFC0DE {
			continue
		}
		mapsize := binary.LittleEndian.Uint64(buf[32:])
		txn := binary.LittleEndian.Uint64(buf[144:])
		if best == 0 || txn > bestTxn {
			best, bestTxn = mapsize, txn
		}
	}
	return int64(best)
}
