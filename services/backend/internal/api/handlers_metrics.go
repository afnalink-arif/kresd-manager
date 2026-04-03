package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// promQuery executes a PromQL query and returns the raw JSON result
func (s *Server) promQuery(query string, params url.Values) (json.RawMessage, error) {
	u := fmt.Sprintf("%s/api/v1/query_range", s.promURL)
	params.Set("query", query)

	resp, err := s.httpClient.Get(u + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("prometheus query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return json.RawMessage(body), nil
}

// promInstantQuery executes a PromQL instant query
func (s *Server) promInstantQuery(query string) (json.RawMessage, error) {
	u := fmt.Sprintf("%s/api/v1/query", s.promURL)
	params := url.Values{"query": {query}}

	resp, err := s.httpClient.Get(u + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("prometheus query: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return json.RawMessage(body), nil
}

func (s *Server) handleMetricsQPS(w http.ResponseWriter, r *http.Request) {
	params := timeRangeParams(r)
	step := r.URL.Query().Get("step")
	if step == "" {
		step = "15s"
	}
	params.Set("step", step)

	protocol := r.URL.Query().Get("protocol")
	query := `rate(kresd_query_total[1m])`
	if protocol != "" {
		query = fmt.Sprintf(`rate(kresd_query_total{transport="%s"}[1m])`, protocol)
	}

	data, err := s.promQuery(query, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, data)
}

func (s *Server) handleMetricsLatency(w http.ResponseWriter, r *http.Request) {
	params := timeRangeParams(r)
	params.Set("step", queryOrDefault(r, "step", "15s"))

	results := map[string]json.RawMessage{}

	for _, p := range []string{"0.5", "0.95", "0.99"} {
		query := fmt.Sprintf(`histogram_quantile(%s, rate(kresd_answer_duration_seconds_bucket[5m]))`, p)
		data, err := s.promQuery(query, params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		results["p"+p[2:]] = data
	}

	writeJSON(w, results)
}

func (s *Server) handleMetricsCache(w http.ResponseWriter, r *http.Request) {
	params := timeRangeParams(r)
	params.Set("step", queryOrDefault(r, "step", "15s"))

	results := map[string]json.RawMessage{}

	queries := map[string]string{
		"hit_ratio": `rate(kresd_cache_hit_total[5m]) / (rate(kresd_cache_hit_total[5m]) + rate(kresd_cache_miss_total[5m]))`,
		"hits":      `rate(kresd_cache_hit_total[5m])`,
		"misses":    `rate(kresd_cache_miss_total[5m])`,
		"size":      `kresd_cache_size_bytes`,
	}

	for name, query := range queries {
		data, err := s.promQuery(query, params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		results[name] = data
	}

	writeJSON(w, results)
}

func (s *Server) handleMetricsDNSSEC(w http.ResponseWriter, r *http.Request) {
	params := timeRangeParams(r)
	params.Set("step", queryOrDefault(r, "step", "15s"))

	results := map[string]json.RawMessage{}

	queries := map[string]string{
		"secure":        `rate(kresd_answer_total{dnssec="secure"}[5m])`,
		"insecure":      `rate(kresd_answer_total{dnssec="insecure"}[5m])`,
		"bogus":         `rate(kresd_answer_total{dnssec="bogus"}[5m])`,
		"indeterminate": `rate(kresd_answer_total{dnssec="indeterminate"}[5m])`,
	}

	for name, query := range queries {
		data, err := s.promQuery(query, params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		results[name] = data
	}

	writeJSON(w, results)
}

func (s *Server) handleMetricsSystem(w http.ResponseWriter, r *http.Request) {
	results := map[string]json.RawMessage{}

	queries := map[string]string{
		"cpu_usage":      `100 - (avg(rate(node_cpu_seconds_total{mode="idle"}[5m])) * 100)`,
		"memory_used":    `node_memory_MemTotal_bytes - node_memory_MemAvailable_bytes`,
		"memory_total":   `node_memory_MemTotal_bytes`,
		"disk_read":      `rate(node_disk_read_bytes_total[5m])`,
		"disk_write":     `rate(node_disk_written_bytes_total[5m])`,
		"network_rx":     `sum(rate(node_network_receive_bytes_total{device!="lo"}[5m]))`,
		"network_tx":     `sum(rate(node_network_transmit_bytes_total{device!="lo"}[5m]))`,
		"load_average":   `node_load1`,
		"disk_usage_pct": `100 - ((node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"}) * 100)`,
	}

	for name, query := range queries {
		data, err := s.promInstantQuery(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		results[name] = data
	}

	writeJSON(w, results)
}

func (s *Server) handleMetricsUpstreams(w http.ResponseWriter, r *http.Request) {
	results := map[string]json.RawMessage{}

	queries := map[string]string{
		"latency":  `kresd_upstream_latency_seconds`,
		"failures": `rate(kresd_upstream_failures_total[5m])`,
		"queries":  `rate(kresd_upstream_queries_total[5m])`,
	}

	for name, query := range queries {
		data, err := s.promInstantQuery(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		results[name] = data
	}

	writeJSON(w, results)
}

func (s *Server) handleMetricsOverview(w http.ResponseWriter, r *http.Request) {
	results := map[string]json.RawMessage{}

	queries := map[string]string{
		"qps":             `sum(rate(kresd_query_total[1m]))`,
		"avg_latency_ms":  `histogram_quantile(0.5, rate(kresd_answer_duration_seconds_bucket[5m])) * 1000`,
		"cache_hit_ratio": `rate(kresd_cache_hit_total[5m]) / (rate(kresd_cache_hit_total[5m]) + rate(kresd_cache_miss_total[5m]))`,
		"dnssec_secure_pct": `rate(kresd_answer_total{dnssec="secure"}[5m]) / rate(kresd_answer_total[5m]) * 100`,
	}

	for name, query := range queries {
		data, err := s.promInstantQuery(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		results[name] = data
	}

	writeJSON(w, results)
}
