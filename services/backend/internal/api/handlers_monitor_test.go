package api

import "testing"

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
