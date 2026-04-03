package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// handleKresdMetrics scrapes kresd management API JSON metrics
// and converts them to Prometheus text format
func (s *Server) handleKresdMetrics(w http.ResponseWriter, r *http.Request) {
	kresdURL := envOr("KRESD_METRICS_URL", "http://kresd:5000/metrics")

	resp, err := s.httpClient.Get(kresdURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("kresd scrape error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read error: %v", err), http.StatusBadGateway)
		return
	}

	// Parse kresd JSON metrics: {"kresd:kresd0": {"query": {...}, "answer": {...}, "request": {...}}, ...}
	var workers map[string]json.RawMessage
	if err := json.Unmarshal(body, &workers); err != nil {
		http.Error(w, fmt.Sprintf("parse error: %v", err), http.StatusBadGateway)
		return
	}

	var out strings.Builder

	// Aggregated counters across all workers
	totals := map[string]float64{}

	for workerName, raw := range workers {
		var sections map[string]json.RawMessage
		if err := json.Unmarshal(raw, &sections); err != nil {
			continue
		}

		worker := strings.TrimPrefix(workerName, "kresd:")

		for sectionName, sectionRaw := range sections {
			var counters map[string]float64
			if err := json.Unmarshal(sectionRaw, &counters); err != nil {
				continue
			}

			for key, val := range counters {
				metricName := fmt.Sprintf("kresd_%s_%s", sectionName, key)
				metricName = sanitizeMetricName(metricName)

				// Per-worker metric
				fmt.Fprintf(&out, "%s{worker=\"%s\"} %g\n", metricName, worker, val)

				// Aggregate
				totals[metricName] += val
			}
		}

		_ = worker
	}

	// Write aggregated totals
	out.WriteString("\n# Aggregated totals across all workers\n")
	for metric, val := range totals {
		fmt.Fprintf(&out, "%s_total %g\n", metric, val)
	}

	// Write well-known Prometheus metrics for dashboard compatibility
	writeNamedMetrics(&out, totals)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Write([]byte(out.String()))
}

func writeNamedMetrics(out *strings.Builder, totals map[string]float64) {
	// QPS-relevant
	if v, ok := totals["kresd_answer_total"]; ok {
		fmt.Fprintf(out, "\n# HELP kresd_query_total Total queries answered\n")
		fmt.Fprintf(out, "# TYPE kresd_query_total counter\n")
		fmt.Fprintf(out, "kresd_query_total %g\n", v)
	}

	// Cache metrics
	if v, ok := totals["kresd_answer_cached"]; ok {
		fmt.Fprintf(out, "# HELP kresd_cache_hit_total Total cache hits\n")
		fmt.Fprintf(out, "# TYPE kresd_cache_hit_total counter\n")
		fmt.Fprintf(out, "kresd_cache_hit_total %g\n", v)
	}

	totalAnswers := totals["kresd_answer_total"]
	cachedAnswers := totals["kresd_answer_cached"]
	if totalAnswers > 0 {
		fmt.Fprintf(out, "kresd_cache_miss_total %g\n", totalAnswers-cachedAnswers)
	}

	// Response codes
	for _, rcode := range []string{"noerror", "nxdomain", "servfail"} {
		key := "kresd_answer_" + rcode
		if v, ok := totals[key]; ok {
			fmt.Fprintf(out, "kresd_answer_rcode{rcode=\"%s\"} %g\n", rcode, v)
		}
	}

	// Protocol distribution
	for _, proto := range []string{"udp", "tcp", "dot", "doh", "doq"} {
		key := "kresd_request_" + proto
		if v, ok := totals[key]; ok {
			fmt.Fprintf(out, "kresd_request_protocol{protocol=\"%s\"} %g\n", proto, v)
		}
	}

	// Latency buckets (kresd reports cumulative count per bucket)
	latencyBuckets := []struct {
		name string
		le   string
	}{
		{"kresd_answer_1ms", "0.001"},
		{"kresd_answer_10ms", "0.01"},
		{"kresd_answer_50ms", "0.05"},
		{"kresd_answer_100ms", "0.1"},
		{"kresd_answer_250ms", "0.25"},
		{"kresd_answer_500ms", "0.5"},
		{"kresd_answer_1000ms", "1.0"},
		{"kresd_answer_1500ms", "1.5"},
		{"kresd_answer_slow", "+Inf"},
	}

	fmt.Fprintf(out, "\n# HELP kresd_answer_duration_seconds_bucket Answer latency histogram\n")
	fmt.Fprintf(out, "# TYPE kresd_answer_duration_seconds_bucket histogram\n")
	cumulative := 0.0
	for _, b := range latencyBuckets {
		if v, ok := totals[b.name]; ok {
			cumulative += v
		}
		fmt.Fprintf(out, "kresd_answer_duration_seconds_bucket{le=\"%s\"} %g\n", b.le, cumulative)
	}
	fmt.Fprintf(out, "kresd_answer_duration_seconds_count %g\n", totals["kresd_answer_total"])
	fmt.Fprintf(out, "kresd_answer_duration_seconds_sum %g\n", totals["kresd_answer_sum_ms"]/1000.0)

	// DNSSEC
	if v, ok := totals["kresd_query_dnssec"]; ok {
		fmt.Fprintf(out, "kresd_query_dnssec_total %g\n", v)
	}
}

func sanitizeMetricName(name string) string {
	// Replace non-alphanumeric chars with underscore
	var b strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			b.WriteRune(c)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
