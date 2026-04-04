package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

var managedServices = []string{
	"kresd", "dnstap-ingester", "prometheus", "node-exporter",
	"clickhouse", "redis", "postgres", "frontend", "caddy", "backend",
}

type ServiceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Health string `json:"health,omitempty"`
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	composePath := s.resolveComposePath()
	services := []ServiceStatus{}

	for _, svc := range managedServices {
		st := ServiceStatus{Name: svc, Status: "unknown"}

		out, err := exec.CommandContext(r.Context(),
			"docker", "compose", "-f", composePath+"/docker-compose.yml",
			"--project-directory", composePath,
			"ps", "--format", "json", svc,
		).Output()
		if err == nil && len(out) > 0 {
			// docker compose ps --format json outputs one JSON object per line
			var info struct {
				State  string `json:"State"`
				Health string `json:"Health"`
			}
			if json.Unmarshal(out, &info) == nil {
				st.Status = info.State
				st.Health = info.Health
			}
		}

		services = append(services, st)
	}

	writeJSON(w, services)
}

func (s *Server) handleRestartService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Service string `json:"service"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Service == "" {
		http.Error(w, `{"error":"service name required"}`, http.StatusBadRequest)
		return
	}

	// Validate service name
	valid := false
	for _, svc := range managedServices {
		if svc == req.Service {
			valid = true
			break
		}
	}
	if !valid {
		http.Error(w, `{"error":"invalid service name"}`, http.StatusBadRequest)
		return
	}

	composePath := s.resolveComposePath()

	// For backend, restart in background since it kills itself
	if req.Service == "backend" {
		go func() {
			exec.Command(
				"docker", "compose", "-f", composePath+"/docker-compose.yml",
				"--project-directory", composePath,
				"restart", "backend",
			).Run()
		}()
		writeJSON(w, map[string]string{"message": "backend restarting"})
		return
	}

	out, err := exec.CommandContext(r.Context(),
		"docker", "compose", "-f", composePath+"/docker-compose.yml",
		"--project-directory", composePath,
		"restart", req.Service,
	).CombinedOutput()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s","output":"%s"}`,
			err.Error(), strings.TrimSpace(string(out))), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]string{"message": req.Service + " restarted"})
}

func (s *Server) handleRestartAll(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	composePath := s.resolveComposePath()

	// Restart order (same as update.sh)
	groups := []struct {
		label    string
		services []string
	}{
		{"Infrastructure", []string{"clickhouse", "redis", "postgres"}},
		{"DNS pipeline", []string{"dnstap-ingester", "kresd"}},
		{"Monitoring", []string{"prometheus", "node-exporter"}},
		{"Frontend", []string{"frontend", "caddy"}},
	}

	for _, g := range groups {
		fmt.Fprintf(w, "data: Restarting %s...\n\n", g.label)
		flusher.Flush()

		for _, svc := range g.services {
			out, err := exec.CommandContext(r.Context(),
				"docker", "compose", "-f", composePath+"/docker-compose.yml",
				"--project-directory", composePath,
				"restart", svc,
			).CombinedOutput()
			if err != nil {
				fmt.Fprintf(w, "data: [ERROR] %s: %s\n\n", svc, strings.TrimSpace(string(out)))
			} else {
				fmt.Fprintf(w, "data: [OK] %s restarted\n\n", svc)
			}
			flusher.Flush()
		}
	}

	// Restart backend last (kills this container)
	fmt.Fprintf(w, "data: Restarting backend...\n\n")
	flusher.Flush()

	go func() {
		exec.Command(
			"docker", "compose", "-f", composePath+"/docker-compose.yml",
			"--project-directory", composePath,
			"restart", "backend",
		).Run()
	}()

	fmt.Fprintf(w, "event: done\ndata: All services restarted\n\n")
	flusher.Flush()
}

// resolveComposePath returns the host project path for docker compose commands.
func (s *Server) resolveComposePath() string {
	// Try to detect host path when running inside a container
	hostPath, err := exec.Command(
		"docker", "inspect", getHostname(),
		"--format", `{{range .Mounts}}{{if eq .Destination "/project"}}{{.Source}}{{end}}{{end}}`,
	).Output()
	if err == nil && len(strings.TrimSpace(string(hostPath))) > 0 {
		return strings.TrimSpace(string(hostPath))
	}
	// Fallback to project dir (works when running on host)
	return s.cfg.ProjectDir
}

func getHostname() string {
	out, err := exec.Command("hostname").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
