package config

import (
	"net"
	"strconv"
	"testing"
)

func TestSelectAvailablePortRejectsOccupiedOnlyPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port

	if _, err := selectAvailablePortImpl("seed", port, port); err == nil {
		t.Fatalf("selectAvailablePortImpl(%d-%d) expected occupied-range error", port, port)
	}
}

func TestDeriveAutoLocalAddrUsesInjectedPortSelector(t *testing.T) {
	original := selectAvailablePort
	t.Cleanup(func() { selectAvailablePort = original })

	var gotStart, gotEnd int
	selectAvailablePort = func(_ string, start, end int) (int, error) {
		gotStart, gotEnd = start, end
		return end, nil
	}
	got, err := deriveAutoLocalAddr("host:22", "office", "41000-41010")
	if err != nil {
		t.Fatal(err)
	}
	if got != net.JoinHostPort("127.0.0.1", strconv.Itoa(41010)) {
		t.Fatalf("deriveAutoLocalAddr() = %q", got)
	}
	if gotStart != 41000 || gotEnd != 41010 {
		t.Fatalf("selector range = %d-%d, want 41000-41010", gotStart, gotEnd)
	}
}
