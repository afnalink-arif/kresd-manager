package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type QueryLogEntry struct {
	Timestamp    time.Time `json:"timestamp"`
	ClientIP     string    `json:"client_ip"`
	Qname        string    `json:"qname"`
	Qtype        uint16    `json:"qtype"`
	Rcode        uint8     `json:"rcode"`
	LatencyUS    uint32    `json:"latency_us"`
	Protocol     string    `json:"protocol"`
	DNSSECStatus string    `json:"dnssec_status"`
	UpstreamIP   string    `json:"upstream_ip"`
	Cached       bool      `json:"cached"`
	ResponseSize uint32    `json:"response_size"`
}

type TopDomain struct {
	Qname      string `json:"qname"`
	QueryCount uint64 `json:"query_count"`
}

type Distribution struct {
	Label string `json:"label"`
	Count uint64 `json:"count"`
}

func (s *Server) handleQuerySearch(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	limit, _ := strconv.Atoi(queryOrDefault(r, "limit", "100"))
	offset, _ := strconv.Atoi(queryOrDefault(r, "offset", "0"))
	if limit > 1000 {
		limit = 1000
	}

	// Build WHERE clauses
	var conditions []string
	var args []interface{}
	argIdx := 1

	// Time range
	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")
	if start != "" {
		conditions = append(conditions, fmt.Sprintf("timestamp >= parseDateTimeBestEffort({v%d:String})", argIdx))
		args = append(args, start)
		argIdx++
	}
	if end != "" {
		conditions = append(conditions, fmt.Sprintf("timestamp <= parseDateTimeBestEffort({v%d:String})", argIdx))
		args = append(args, end)
		argIdx++
	}

	// Filters
	if domain := r.URL.Query().Get("domain"); domain != "" {
		conditions = append(conditions, fmt.Sprintf("qname LIKE {v%d:String}", argIdx))
		args = append(args, "%"+domain+"%")
		argIdx++
	}
	if clientIP := r.URL.Query().Get("client_ip"); clientIP != "" {
		conditions = append(conditions, fmt.Sprintf("toString(client_ip) = {v%d:String}", argIdx))
		args = append(args, clientIP)
		argIdx++
	}
	if qtype := r.URL.Query().Get("qtype"); qtype != "" {
		conditions = append(conditions, fmt.Sprintf("qtype = {v%d:UInt16}", argIdx))
		args = append(args, qtype)
		argIdx++
	}
	if rcode := r.URL.Query().Get("rcode"); rcode != "" {
		conditions = append(conditions, fmt.Sprintf("rcode = {v%d:UInt8}", argIdx))
		args = append(args, rcode)
		argIdx++
	}
	if protocol := r.URL.Query().Get("protocol"); protocol != "" {
		conditions = append(conditions, fmt.Sprintf("protocol = {v%d:String}", argIdx))
		args = append(args, protocol)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT timestamp, toString(client_ip), qname, qtype, rcode, latency_us,
			   protocol, dnssec_status, toString(upstream_ip), cached, response_size
		FROM dns_queries
		%s
		ORDER BY timestamp DESC
		LIMIT %d OFFSET %d
	`, where, limit, offset)

	rows, err := s.ch.Query(ctx, query)
	if err != nil {
		http.Error(w, fmt.Sprintf("query error: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var entries []QueryLogEntry
	for rows.Next() {
		var e QueryLogEntry
		if err := rows.Scan(
			&e.Timestamp, &e.ClientIP, &e.Qname, &e.Qtype, &e.Rcode,
			&e.LatencyUS, &e.Protocol, &e.DNSSECStatus, &e.UpstreamIP,
			&e.Cached, &e.ResponseSize,
		); err != nil {
			http.Error(w, fmt.Sprintf("scan error: %v", err), http.StatusInternalServerError)
			return
		}
		entries = append(entries, e)
	}

	if entries == nil {
		entries = []QueryLogEntry{}
	}

	writeJSON(w, map[string]interface{}{
		"data":   entries,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleTopDomains(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	limit, _ := strconv.Atoi(queryOrDefault(r, "limit", "20"))
	hours, _ := strconv.Atoi(queryOrDefault(r, "hours", "1"))

	query := fmt.Sprintf(`
		SELECT qname, sum(query_count) as total
		FROM top_domains_1h
		WHERE timestamp >= now() - INTERVAL %d HOUR
		GROUP BY qname
		ORDER BY total DESC
		LIMIT %d
	`, hours, limit)

	rows, err := s.ch.Query(ctx, query)
	if err != nil {
		http.Error(w, fmt.Sprintf("query error: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var domains []TopDomain
	for rows.Next() {
		var d TopDomain
		if err := rows.Scan(&d.Qname, &d.QueryCount); err != nil {
			http.Error(w, fmt.Sprintf("scan error: %v", err), http.StatusInternalServerError)
			return
		}
		domains = append(domains, d)
	}

	if domains == nil {
		domains = []TopDomain{}
	}
	writeJSON(w, domains)
}

func (s *Server) handleTypeDistribution(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	hours, _ := strconv.Atoi(queryOrDefault(r, "hours", "1"))

	query := fmt.Sprintf(`
		SELECT qtype, sum(query_count) as total
		FROM dns_queries_1m
		WHERE timestamp >= now() - INTERVAL %d HOUR
		GROUP BY qtype
		ORDER BY total DESC
		LIMIT 20
	`, hours)

	rows, err := s.ch.Query(ctx, query)
	if err != nil {
		http.Error(w, fmt.Sprintf("query error: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	qtypeNames := map[uint16]string{
		1: "A", 2: "NS", 5: "CNAME", 6: "SOA", 12: "PTR",
		15: "MX", 16: "TXT", 28: "AAAA", 33: "SRV", 43: "DS",
		46: "RRSIG", 48: "DNSKEY", 65: "HTTPS", 255: "ANY",
	}

	var dist []Distribution
	for rows.Next() {
		var qtype uint16
		var count uint64
		if err := rows.Scan(&qtype, &count); err != nil {
			continue
		}
		label := fmt.Sprintf("TYPE%d", qtype)
		if name, ok := qtypeNames[qtype]; ok {
			label = name
		}
		dist = append(dist, Distribution{Label: label, Count: count})
	}

	if dist == nil {
		dist = []Distribution{}
	}
	writeJSON(w, dist)
}

func (s *Server) handleRcodeDistribution(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	hours, _ := strconv.Atoi(queryOrDefault(r, "hours", "1"))

	rcodeNames := map[uint8]string{
		0: "NOERROR", 1: "FORMERR", 2: "SERVFAIL", 3: "NXDOMAIN",
		4: "NOTIMP", 5: "REFUSED",
	}

	query := fmt.Sprintf(`
		SELECT rcode, sum(query_count) as total
		FROM dns_queries_1m
		WHERE timestamp >= now() - INTERVAL %d HOUR
		GROUP BY rcode
		ORDER BY total DESC
	`, hours)

	rows, err := s.ch.Query(ctx, query)
	if err != nil {
		http.Error(w, fmt.Sprintf("query error: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var dist []Distribution
	for rows.Next() {
		var rcode uint8
		var count uint64
		if err := rows.Scan(&rcode, &count); err != nil {
			continue
		}
		label := fmt.Sprintf("RCODE%d", rcode)
		if name, ok := rcodeNames[rcode]; ok {
			label = name
		}
		dist = append(dist, Distribution{Label: label, Count: count})
	}

	if dist == nil {
		dist = []Distribution{}
	}
	writeJSON(w, dist)
}

func (s *Server) handleProtocolDistribution(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	hours, _ := strconv.Atoi(queryOrDefault(r, "hours", "1"))

	query := fmt.Sprintf(`
		SELECT protocol, sum(query_count) as total
		FROM dns_queries_1m
		WHERE timestamp >= now() - INTERVAL %d HOUR
		GROUP BY protocol
		ORDER BY total DESC
	`, hours)

	rows, err := s.ch.Query(ctx, query)
	if err != nil {
		http.Error(w, fmt.Sprintf("query error: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	protocolLabels := map[string]string{
		"udp": "DNS (UDP)", "tcp": "DNS (TCP)", "dot": "DoT", "doh": "DoH", "doq": "DoQ",
	}

	var dist []Distribution
	for rows.Next() {
		var proto string
		var count uint64
		if err := rows.Scan(&proto, &count); err != nil {
			continue
		}
		label := proto
		if l, ok := protocolLabels[proto]; ok {
			label = l
		}
		dist = append(dist, Distribution{Label: label, Count: count})
	}

	if dist == nil {
		dist = []Distribution{}
	}
	writeJSON(w, dist)
}

func (s *Server) handleQueryTimeline(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	hours, _ := strconv.Atoi(queryOrDefault(r, "hours", "1"))

	query := fmt.Sprintf(`
		SELECT timestamp, sum(query_count) as total, avg(avg_latency) as latency
		FROM dns_queries_1m
		WHERE timestamp >= now() - INTERVAL %d HOUR
		GROUP BY timestamp
		ORDER BY timestamp
	`, hours)

	rows, err := s.ch.Query(ctx, query)
	if err != nil {
		http.Error(w, fmt.Sprintf("query error: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type TimePoint struct {
		Timestamp time.Time `json:"timestamp"`
		Count     uint64    `json:"count"`
		Latency   float64   `json:"latency"`
	}

	var points []TimePoint
	for rows.Next() {
		var p TimePoint
		if err := rows.Scan(&p.Timestamp, &p.Count, &p.Latency); err != nil {
			continue
		}
		points = append(points, p)
	}

	if points == nil {
		points = []TimePoint{}
	}
	writeJSON(w, points)
}
