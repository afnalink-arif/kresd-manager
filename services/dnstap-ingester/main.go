package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Frame Streams control frame types
const (
	fsControlAccept      = 0x01
	fsControlStart       = 0x02
	fsControlStop        = 0x03
	fsControlReady       = 0x04
	fsControlFinish      = 0x05
	fsFieldContentType   = 0x01
	dnstapContentType    = "protobuf:dnstap.Dnstap"
)

type QueryRow struct {
	Timestamp    time.Time
	ClientIP     string
	Qname        string
	Qtype        uint16
	Rcode        uint8
	LatencyUS    uint32
	Protocol     string
	DNSSECStatus string
	Cached       bool
	ResponseSize uint32
}

func main() {
	socketPath := envOrDefault("DNSTAP_SOCKET", "/var/run/dnstap/dnstap.sock")
	clickhouseAddr := envOrDefault("CLICKHOUSE_URL", "http://clickhouse:8123")
	clickhouseDB := envOrDefault("CLICKHOUSE_DB", "dnsmonitor")
	batchSize := envIntOrDefault("BATCH_SIZE", 500)
	flushInterval := envDurationOrDefault("FLUSH_INTERVAL", time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	chInsertURL := fmt.Sprintf("%s/?database=%s&query=%s",
		strings.TrimRight(clickhouseAddr, "/"),
		clickhouseDB,
		"INSERT+INTO+dns_queries+(timestamp,client_ip,qname,qtype,rcode,latency_us,protocol,dnssec_status,upstream_ip,cached,response_size)+FORMAT+TabSeparated")

	log.Printf("ClickHouse: %s/%s", clickhouseAddr, clickhouseDB)
	log.Printf("Dnstap socket: %s", socketPath)

	var rows []QueryRow
	var mu sync.Mutex
	flushCh := make(chan struct{}, 1)

	// Periodic flusher
	go func() {
		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case flushCh <- struct{}{}:
				default:
				}
			}
		}
	}()

	// Batch writer
	go func() {
		for {
			select {
			case <-ctx.Done():
				mu.Lock()
				if len(rows) > 0 {
					flushBatchHTTP(chInsertURL, rows)
				}
				mu.Unlock()
				return
			case <-flushCh:
				mu.Lock()
				if len(rows) > 0 {
					batch := make([]QueryRow, len(rows))
					copy(batch, rows)
					rows = rows[:0]
					mu.Unlock()
					flushBatchHTTP(chInsertURL, batch)
				} else {
					mu.Unlock()
				}
			}
		}
	}()

	// Remove stale socket, then listen
	os.Remove(socketPath)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Failed to listen on %s: %v", socketPath, err)
	}
	defer listener.Close()
	os.Chmod(socketPath, 0777)
	log.Printf("Listening for Frame Streams connections on %s", socketPath)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("Accept error: %v", err)
			continue
		}

		log.Printf("New connection from kresd")
		go handleFSConnection(ctx, conn, &mu, &rows, batchSize, flushCh)
	}
}

// handleFSConnection implements the Frame Streams bidirectional handshake and data reading
func handleFSConnection(ctx context.Context, conn net.Conn, mu *sync.Mutex, rows *[]QueryRow, batchSize int, flushCh chan struct{}) {
	defer conn.Close()

	// ---- Frame Streams bidirectional handshake ----
	// 1. Client (kresd) sends READY control frame
	// 2. Server (us) sends ACCEPT control frame
	// 3. Client sends START control frame
	// 4. Client sends data frames
	// 5. Client sends STOP control frame
	// 6. Server sends FINISH control frame

	// Step 1: Read READY frame from kresd
	ctrlType, _, err := readControlFrame(conn)
	if err != nil {
		log.Printf("Failed to read READY frame: %v", err)
		return
	}
	if ctrlType != fsControlReady {
		log.Printf("Expected READY (0x04), got 0x%02x", ctrlType)
		return
	}
	log.Printf("Received READY frame from kresd")

	// Step 2: Send ACCEPT frame
	if err := writeControlFrame(conn, fsControlAccept); err != nil {
		log.Printf("Failed to send ACCEPT frame: %v", err)
		return
	}
	log.Printf("Sent ACCEPT frame")

	// Step 3: Read START frame
	ctrlType, _, err = readControlFrame(conn)
	if err != nil {
		log.Printf("Failed to read START frame: %v", err)
		return
	}
	if ctrlType != fsControlStart {
		log.Printf("Expected START (0x02), got 0x%02x", ctrlType)
		return
	}
	log.Printf("Received START frame — now reading dnstap data")

	// Step 4: Read data frames until STOP
	count := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Read 4-byte frame length
		var frameLen uint32
		if err := binary.Read(conn, binary.BigEndian, &frameLen); err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if err == io.EOF {
				log.Printf("Connection closed (read %d frames)", count)
			} else {
				log.Printf("Read error after %d frames: %v", count, err)
			}
			return
		}

		// frameLen == 0 means control frame
		if frameLen == 0 {
			ctrlType, _, err := readControlFrame(conn)
			if err != nil {
				log.Printf("Control frame read error: %v", err)
				return
			}
			if ctrlType == fsControlStop {
				log.Printf("Received STOP frame after %d data frames", count)
				// Send FINISH
				writeControlFrame(conn, fsControlFinish)
				return
			}
			log.Printf("Unexpected control frame 0x%02x during data", ctrlType)
			continue
		}

		// Sanity check
		if frameLen > 1048576 { // 1MB max
			log.Printf("Frame too large: %d bytes", frameLen)
			return
		}

		// Read data frame
		frame := make([]byte, frameLen)
		if _, err := io.ReadFull(conn, frame); err != nil {
			log.Printf("Read data frame error: %v", err)
			return
		}

		count++

		// Parse dnstap protobuf
		row := parseDnstapFrame(frame)
		if row == nil || row.Qname == "" {
			continue
		}

		mu.Lock()
		*rows = append(*rows, *row)
		shouldFlush := len(*rows) >= batchSize
		mu.Unlock()

		if shouldFlush {
			select {
			case flushCh <- struct{}{}:
			default:
			}
		}
	}
}

// readControlFrame reads a control frame (after the initial 0x00000000 has already been read,
// OR reads the 0x00000000 marker + control frame body)
func readControlFrame(conn net.Conn) (uint32, []byte, error) {
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// Read escape sequence (4 bytes of 0x00000000)
	var escape uint32
	if err := binary.Read(conn, binary.BigEndian, &escape); err != nil {
		return 0, nil, fmt.Errorf("read escape: %w", err)
	}
	if escape != 0 {
		return 0, nil, fmt.Errorf("expected escape 0x00000000, got 0x%08x", escape)
	}

	// Read control frame length
	var ctrlLen uint32
	if err := binary.Read(conn, binary.BigEndian, &ctrlLen); err != nil {
		return 0, nil, fmt.Errorf("read control length: %w", err)
	}
	if ctrlLen < 4 || ctrlLen > 65536 {
		return 0, nil, fmt.Errorf("invalid control frame length: %d", ctrlLen)
	}

	// Read control frame body
	body := make([]byte, ctrlLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return 0, nil, fmt.Errorf("read control body: %w", err)
	}

	// First 4 bytes = control type
	ctrlType := binary.BigEndian.Uint32(body[:4])
	return ctrlType, body[4:], nil
}

// writeControlFrame writes a Frame Streams control frame
func writeControlFrame(conn net.Conn, ctrlType uint32) error {
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	ct := []byte(dnstapContentType)
	// Control frame: escape(4) + length(4) + type(4) + field_type(4) + field_len(4) + content_type
	ctrlLen := uint32(4 + 4 + 4 + len(ct)) // type + field_type + field_len + ct
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, uint32(0))          // escape
	binary.Write(&buf, binary.BigEndian, ctrlLen)             // control frame length
	binary.Write(&buf, binary.BigEndian, ctrlType)            // control type
	binary.Write(&buf, binary.BigEndian, uint32(fsFieldContentType)) // field type
	binary.Write(&buf, binary.BigEndian, uint32(len(ct)))     // field length
	buf.Write(ct)                                              // content type string

	_, err := conn.Write(buf.Bytes())
	return err
}

// parseDnstapFrame parses a dnstap protobuf message
func parseDnstapFrame(data []byte) *QueryRow {
	msg := extractField(data, 14) // field 14 = Message
	if msg == nil {
		return nil
	}

	row := &QueryRow{
		Timestamp:    time.Now().UTC(),
		Protocol:     "udp",
		DNSSECStatus: "unknown",
		ClientIP:     "::",
	}

	if proto := extractVarint(msg, 3); proto > 0 {
		switch proto {
		case 1:
			row.Protocol = "udp"
		case 2:
			row.Protocol = "tcp"
		case 3:
			row.Protocol = "dot"
		case 4:
			row.Protocol = "doh"
		case 5, 6:
			row.Protocol = "doq"
		}
	}

	if addr := extractField(msg, 4); addr != nil {
		row.ClientIP = net.IP(addr).String()
	}

	if qmsg := extractField(msg, 10); len(qmsg) > 12 {
		row.Qname, row.Qtype = parseSimpleDNS(qmsg)
	}

	if rmsg := extractField(msg, 13); len(rmsg) >= 4 {
		row.Rcode = uint8(rmsg[3] & 0x0F)
		row.ResponseSize = uint32(len(rmsg))
	}

	qt := extractVarint(msg, 8)
	rt := extractVarint(msg, 11)
	if qt > 0 && rt >= qt {
		row.LatencyUS = uint32((rt - qt) * 1_000_000)
		qns := extractFixed32(msg, 9)
		rns := extractFixed32(msg, 12)
		if rns >= qns {
			row.LatencyUS += (rns - qns) / 1000
		}
	}

	return row
}

// --- Protobuf helpers ---

func extractField(data []byte, fieldNum uint64) []byte {
	i := 0
	for i < len(data) {
		tag, n := decodeVarint(data[i:])
		if n <= 0 { break }
		i += n
		wt := tag & 0x07
		num := tag >> 3
		switch wt {
		case 0:
			_, n = decodeVarint(data[i:])
			if n <= 0 { return nil }
			i += n
		case 1:
			i += 8
		case 2:
			length, n := decodeVarint(data[i:])
			if n <= 0 { return nil }
			i += n
			end := i + int(length)
			if end > len(data) { return nil }
			if num == fieldNum { return data[i:end] }
			i = end
		case 5:
			i += 4
		default:
			return nil
		}
	}
	return nil
}

func extractVarint(data []byte, fieldNum uint64) uint64 {
	i := 0
	for i < len(data) {
		tag, n := decodeVarint(data[i:])
		if n <= 0 { break }
		i += n
		wt := tag & 0x07
		num := tag >> 3
		switch wt {
		case 0:
			val, n := decodeVarint(data[i:])
			if n <= 0 { return 0 }
			if num == fieldNum { return val }
			i += n
		case 1: i += 8
		case 2:
			l, n := decodeVarint(data[i:])
			if n <= 0 { return 0 }
			i += n + int(l)
		case 5: i += 4
		default: return 0
		}
	}
	return 0
}

func extractFixed32(data []byte, fieldNum uint64) uint32 {
	i := 0
	for i < len(data) {
		tag, n := decodeVarint(data[i:])
		if n <= 0 { break }
		i += n
		wt := tag & 0x07
		num := tag >> 3
		switch wt {
		case 0:
			_, n := decodeVarint(data[i:])
			if n <= 0 { return 0 }
			i += n
		case 1: i += 8
		case 2:
			l, n := decodeVarint(data[i:])
			if n <= 0 { return 0 }
			i += n + int(l)
		case 5:
			if num == fieldNum && i+4 <= len(data) {
				return binary.LittleEndian.Uint32(data[i : i+4])
			}
			i += 4
		default: return 0
		}
	}
	return 0
}

func decodeVarint(buf []byte) (uint64, int) {
	var x uint64
	var s uint
	for i, b := range buf {
		if i >= 10 { return 0, -1 }
		if b < 0x80 { return x | uint64(b)<<s, i + 1 }
		x |= uint64(b&0x7f) << s
		s += 7
	}
	return 0, -1
}

func parseSimpleDNS(msg []byte) (string, uint16) {
	if len(msg) < 13 { return "", 0 }
	pos := 12
	var labels []string
	for pos < len(msg) {
		length := int(msg[pos])
		if length == 0 { pos++; break }
		if pos+1+length > len(msg) { return "", 0 }
		labels = append(labels, string(msg[pos+1:pos+1+length]))
		pos += 1 + length
	}
	qname := strings.Join(labels, ".")
	var qtype uint16
	if pos+2 <= len(msg) {
		qtype = uint16(msg[pos])<<8 | uint16(msg[pos+1])
	}
	return qname, qtype
}

// --- ClickHouse HTTP insert ---

func flushBatchHTTP(chURL string, rows []QueryRow) {
	var body strings.Builder
	for _, r := range rows {
		clientIP := r.ClientIP
		if clientIP == "" || clientIP == "<nil>" {
			clientIP = "::"
		}
		// Map IPv4 to IPv6-mapped format for ClickHouse IPv6 type
		ip := net.ParseIP(clientIP)
		if ip != nil {
			if ip4 := ip.To4(); ip4 != nil {
				clientIP = "::ffff:" + ip4.String()
			} else {
				clientIP = ip.String()
			}
		}

		cached := uint8(0)
		if r.Cached { cached = 1 }
		fmt.Fprintf(&body, "%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\t::\t%d\t%d\n",
			r.Timestamp.Format("2006-01-02 15:04:05.000"),
			clientIP,
			r.Qname,
			r.Qtype,
			r.Rcode,
			r.LatencyUS,
			r.Protocol,
			r.DNSSECStatus,
			cached,
			r.ResponseSize,
		)
	}

	resp, err := http.Post(chURL, "text/plain", strings.NewReader(body.String()))
	if err != nil {
		log.Printf("ClickHouse insert error: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("ClickHouse error %d: %s", resp.StatusCode, string(respBody))
		return
	}
	log.Printf("Flushed %d rows to ClickHouse", len(rows))
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}

func envIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil { return n }
	}
	return def
}

func envDurationOrDefault(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil { return d }
	}
	return def
}
