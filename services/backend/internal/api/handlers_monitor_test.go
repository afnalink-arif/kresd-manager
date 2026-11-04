package api

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestMapsizeRe(t *testing.T) {
	src := []byte("-- 16 GiB\nkr_rules_init(nil, 17179869184, false)\n")
	m := mapsizeRe.FindSubmatch(src)
	if m == nil || string(m[1]) != "17179869184" {
		t.Fatalf("mapsize tidak terbaca: %q", m)
	}
	if mapsizeRe.FindSubmatch([]byte("kr_rules_init(nil, N, true)")) != nil {
		t.Fatal("placeholder non-angka tak boleh cocok")
	}
}

func TestLmdbMapsize(t *testing.T) {
	buf := make([]byte, 8192)
	meta := func(off int, mapsize, txn uint64) {
		binary.LittleEndian.PutUint32(buf[off+16:], 0xBEEFC0DE)
		binary.LittleEndian.PutUint64(buf[off+32:], mapsize)
		binary.LittleEndian.PutUint64(buf[off+144:], txn)
	}
	meta(0, 12<<30, 88)
	meta(4096, 16<<30, 89) // newer meta wins
	p := filepath.Join(t.TempDir(), "data.mdb")
	if err := os.WriteFile(p, buf, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := lmdbMapsize(p); got != 16<<30 {
		t.Fatalf("mapsize %d, want 16 GiB", got)
	}
	if lmdbMapsize(filepath.Join(t.TempDir(), "absent")) != 0 {
		t.Fatal("missing file must read 0")
	}
}
