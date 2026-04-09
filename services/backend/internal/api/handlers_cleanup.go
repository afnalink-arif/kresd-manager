package api

import (
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

type CleanupInfo struct {
	DanglingImages         int    `json:"dangling_images"`
	DanglingImagesSize     string `json:"dangling_images_size"`
	BuildCacheSize         string `json:"build_cache_size"`
	BuildCacheReclaimable  string `json:"build_cache_reclaimable"`
	BuildCacheEntries      int    `json:"build_cache_entries"`
	TotalReclaimable       string `json:"total_reclaimable"`
}

func (s *Server) handleGetCleanupInfo(w http.ResponseWriter, r *http.Request) {
	info := CleanupInfo{}

	// Count dangling images
	out, err := exec.CommandContext(r.Context(), "docker", "images", "--filter", "dangling=true", "--format", "{{.Size}}").Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		if lines[0] != "" {
			info.DanglingImages = len(lines)
		}
		info.DanglingImagesSize = sumDockerSizes(lines)
	}

	// Build cache info via docker system df
	out, err = exec.CommandContext(r.Context(), "docker", "system", "df", "--format", "{{.Type}}\t{{.TotalCount}}\t{{.Size}}\t{{.Reclaimable}}").Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			fields := strings.Split(line, "\t")
			if len(fields) < 4 {
				continue
			}
			switch fields[0] {
			case "Build Cache":
				info.BuildCacheEntries, _ = strconv.Atoi(fields[1])
				info.BuildCacheSize = fields[2]
				info.BuildCacheReclaimable = fields[3]
			}
		}
	}

	// Total reclaimable = dangling images + build cache reclaimable only
	info.TotalReclaimable = sumDockerSizes([]string{info.DanglingImagesSize, info.BuildCacheReclaimable})

	writeJSON(w, info)
}

func (s *Server) handleRunCleanup(w http.ResponseWriter, r *http.Request) {
	results := []string{}

	// Prune dangling images
	out, err := exec.CommandContext(r.Context(), "docker", "image", "prune", "-f").CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Total reclaimed space:") {
				results = append(results, "Images: "+strings.TrimPrefix(line, "Total reclaimed space: "))
			}
		}
	}

	// Prune build cache
	out, err = exec.CommandContext(r.Context(), "docker", "builder", "prune", "-f").CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Total:") {
				results = append(results, "Build cache: "+strings.TrimSpace(strings.TrimPrefix(line, "Total:")))
			}
		}
	}

	writeJSON(w, map[string]interface{}{
		"message": "Cleanup complete",
		"details": results,
	})
}

// sumDockerSizes sums human-readable sizes like "1.2GB", "345MB", "12.5kB"
func sumDockerSizes(lines []string) string {
	totalBytes := float64(0)
	for _, s := range lines {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		// Strip parenthesized percentage like "(100%)"
		if idx := strings.Index(s, " ("); idx > 0 {
			s = s[:idx]
		}
		val, unit := parseDockerSize(s)
		totalBytes += val * unit
	}
	return formatBytes(totalBytes)
}

func parseDockerSize(s string) (float64, float64) {
	s = strings.TrimSpace(s)
	multipliers := []struct {
		suffix string
		mult   float64
	}{
		{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1},
	}
	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			numStr := strings.TrimSuffix(s, m.suffix)
			val, err := strconv.ParseFloat(numStr, 64)
			if err == nil {
				return val, m.mult
			}
		}
	}
	return 0, 1
}

func formatBytes(b float64) string {
	if b >= 1e9 {
		return fmt.Sprintf("%.1f GB", b/1e9)
	}
	if b >= 1e6 {
		return fmt.Sprintf("%.1f MB", b/1e6)
	}
	if b >= 1e3 {
		return fmt.Sprintf("%.1f KB", b/1e3)
	}
	return fmt.Sprintf("%.0f B", b)
}
