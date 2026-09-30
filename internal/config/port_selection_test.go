package config

import (
	"hash/fnv"
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

func TestSelectAvailablePortSkipsOccupiedPortAndReturnsFreeCandidate(t *testing.T) {
	occupied, freePort := consecutiveAvailablePorts(t)
	defer occupied.Close()
	occupiedPort := occupied.Addr().(*net.TCPAddr).Port

	got, err := selectAvailablePortImpl(seedForOffset(2, 0), occupiedPort, freePort)
	if err != nil {
		t.Fatalf("selectAvailablePortImpl() error = %v", err)
	}
	if got != freePort {
		t.Fatalf("selectAvailablePortImpl() = %d, want free candidate %d", got, freePort)
	}
}

func seedForOffset(span, want int) string {
	for candidate := 0; ; candidate++ {
		seed := strconv.Itoa(candidate)
		hasher := fnv.New32a()
		_, _ = hasher.Write([]byte(seed))
		if int(int64(hasher.Sum32())%int64(span)) == want {
			return seed
		}
	}
}

func consecutiveAvailablePorts(t *testing.T) (net.Listener, int) {
	t.Helper()
	for port := 20000; port < 60000; port++ {
		first, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			continue
		}
		second, secondErr := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port+1)))
		if secondErr == nil {
			_ = second.Close()
			return first, port + 1
		}
		_ = first.Close()
	}
	t.Fatal("could not reserve consecutive loopback ports")
	return nil, 0
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
