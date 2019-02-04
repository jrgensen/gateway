package main

import "testing"

func TestSplitHHP(t *testing.T) {
	ps := &ProxyServer{}

	src, dst, port, _ := ps.splitHostHostPort("srcdst")
	if src != "srcdst" || dst != "srcdst" || port != 80 {
		t.Errorf("Source host should equal destination host and have default port, got: %s, %s, %d.", src, dst, port)
	}
	src, dst, port, _ = ps.splitHostHostPort("src:dst")
	if src != "src" || dst != "dst" || port != 80 {
		t.Errorf("Source host and destination host shoudl be different and have default port, got: %s, %s, %d.", src, dst, port)
	}
	src, dst, port, _ = ps.splitHostHostPort("srcdst:81")
	if src != "srcdst" || dst != "srcdst" || port != 81 {
		t.Errorf("Source host should equal destination host and have specific port, got: %s, %s, %d.", src, dst, port)
	}
	src, dst, port, _ = ps.splitHostHostPort("src:dst:82")
	if src != "src" || dst != "dst" || port != 82 {
		t.Errorf("Source host and destination host should be different and have specific port, got: %s, %s, %d.", src, dst, port)
	}
}
