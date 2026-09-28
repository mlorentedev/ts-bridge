package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"ts-bridge/internal/config"
	"ts-bridge/internal/telemetry"
)

const (
	socksVersion5     = 0x05
	socksNoAuth       = 0x00
	socksNoAcceptable = 0xff
	socksConnect      = 0x01
	socksIPv4         = 0x01
	socksDomain       = 0x03
	socksIPv6         = 0x04
)

var ErrSOCKSTargetDenied = errors.New("SOCKS5 target denied by route allow-list")

type SOCKSTargetResolver func(host string, port uint16) (string, error)

type socksRequestError struct {
	reply byte
	cause error
}

func (e *socksRequestError) Error() string { return e.cause.Error() }
func (e *socksRequestError) Unwrap() error { return e.cause }

func directSOCKSTarget(host string, port uint16) (string, error) {
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func DirectSOCKSTarget() SOCKSTargetResolver {
	return directSOCKSTarget
}

func NewAllowlistSOCKSTarget(routes map[string]string) (SOCKSTargetResolver, error) {
	normalized := make(map[string]string, len(routes))
	for source, destination := range routes {
		sourceKey, err := normalizeSOCKSAddress(source)
		if err != nil {
			return nil, fmt.Errorf("invalid SOCKS5 route source %q: %w", source, err)
		}
		if _, err := normalizeSOCKSAddress(destination); err != nil {
			return nil, fmt.Errorf("invalid SOCKS5 route destination %q: %w", destination, err)
		}
		normalized[sourceKey] = destination
	}
	return func(host string, port uint16) (string, error) {
		key := net.JoinHostPort(strings.ToLower(host), strconv.Itoa(int(port)))
		target, ok := normalized[key]
		if !ok {
			return "", fmt.Errorf("%w: %s", ErrSOCKSTargetDenied, key)
		}
		return target, nil
	}, nil
}

func normalizeSOCKSAddress(address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}
	if host == "" {
		return "", fmt.Errorf("host cannot be empty")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", fmt.Errorf("invalid port %q", port)
	}
	return net.JoinHostPort(strings.ToLower(host), strconv.Itoa(portNumber)), nil
}

func AcceptSOCKS5Loop(
	listener net.Listener,
	dialer Dialer,
	cfg config.Config,
	resolve SOCKSTargetResolver,
	wg *sync.WaitGroup,
	cancel context.CancelCauseFunc,
	logger *slog.Logger,
) error {
	if resolve == nil {
		return fmt.Errorf("SOCKS5 target resolver is required")
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return fmt.Errorf("accept SOCKS5 connection: %w", err)
		}
		if !telemetry.TryClaimConnection(cfg.MaxConnections) {
			telemetry.AddRejectedConn()
			_ = conn.Close()
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer telemetry.AddActiveConnection(-1)
			telemetry.AddTotalConnection()
			if err := handleSOCKS5Conn(conn, dialer, resolve, cfg, cancel, logger); err != nil {
				telemetry.AddError()
				logger.Warn("SOCKS5 connection failed", "error", err)
			}
		}()
	}
}

func handleSOCKS5Conn(
	client net.Conn,
	dialer Dialer,
	resolve SOCKSTargetResolver,
	cfg config.Config,
	cancel context.CancelCauseFunc,
	logger *slog.Logger,
) error {
	defer client.Close()
	if cfg.DialTimeout > 0 {
		_ = client.SetDeadline(time.Now().Add(cfg.DialTimeout))
		defer func() { _ = client.SetDeadline(time.Time{}) }()
	}
	if err := negotiateSOCKS5(client); err != nil {
		return err
	}

	host, port, err := readSOCKS5Connect(client)
	if err != nil {
		reply := byte(0x01)
		var requestError *socksRequestError
		if errors.As(err, &requestError) {
			reply = requestError.reply
		}
		_ = writeSOCKS5Reply(client, reply)
		return err
	}
	target, err := resolve(host, port)
	if err != nil {
		reply := byte(0x01)
		if errors.Is(err, ErrSOCKSTargetDenied) {
			reply = 0x02
		}
		_ = writeSOCKS5Reply(client, reply)
		return fmt.Errorf("resolve SOCKS5 target: %w", err)
	}
	_ = client.SetDeadline(time.Time{})

	dialContext := context.Background()
	dialCancel := func() {}
	if cfg.DialTimeout > 0 {
		dialContext, dialCancel = context.WithTimeout(dialContext, cfg.DialTimeout)
	}
	defer dialCancel()
	remote, err := dialer.Dial(dialContext, "tcp", target)
	if err != nil {
		_ = writeSOCKS5Reply(client, 0x05)
		var terminalError *TerminalDialError
		if errors.As(err, &terminalError) && cancel != nil {
			cancel(err)
		}
		return fmt.Errorf("dial SOCKS5 target %s: %w", target, err)
	}
	if err := writeSOCKS5Reply(client, 0x00); err != nil {
		_ = remote.Close()
		return err
	}

	client = withIdleTimeout(client, cfg.IdleTimeout)
	remote = withIdleTimeout(remote, cfg.IdleTimeout)
	bytesTx, bytesRx := proxyConnections(client, remote, client.RemoteAddr().String(), logger)
	telemetry.AddBytesTx(bytesTx)
	telemetry.AddBytesRx(bytesRx)
	return nil
}

func negotiateSOCKS5(conn net.Conn) error {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("read SOCKS5 greeting: %w", err)
	}
	if header[0] != socksVersion5 {
		return fmt.Errorf("unsupported SOCKS version: %d", header[0])
	}
	methods := make([]byte, int(header[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return fmt.Errorf("read SOCKS5 methods: %w", err)
	}
	for _, method := range methods {
		if method == socksNoAuth {
			_, err := conn.Write([]byte{socksVersion5, socksNoAuth})
			return err
		}
	}
	_, _ = conn.Write([]byte{socksVersion5, socksNoAcceptable})
	return fmt.Errorf("SOCKS5 client did not offer no-authentication")
}

func readSOCKS5Connect(conn net.Conn) (string, uint16, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", 0, fmt.Errorf("read SOCKS5 request: %w", err)
	}
	if header[0] != socksVersion5 {
		return "", 0, &socksRequestError{reply: 0x01, cause: fmt.Errorf("unsupported SOCKS version: %d", header[0])}
	}
	if header[1] != socksConnect {
		return "", 0, &socksRequestError{reply: 0x07, cause: fmt.Errorf("unsupported SOCKS5 command: %d", header[1])}
	}
	if header[3] != socksIPv4 && header[3] != socksDomain && header[3] != socksIPv6 {
		return "", 0, &socksRequestError{reply: 0x08, cause: fmt.Errorf("unsupported SOCKS5 address type: %d", header[3])}
	}
	address, err := readSOCKS5Address(conn, header[3])
	if err != nil {
		return "", 0, err
	}
	port := uint16(address[len(address)-2])<<8 | uint16(address[len(address)-1])
	return addressHost(header[3], address[:len(address)-2]), port, nil
}

func readSOCKS5Address(conn net.Conn, addressType byte) ([]byte, error) {
	length := 0
	switch addressType {
	case socksIPv4:
		length = net.IPv4len
	case socksIPv6:
		length = net.IPv6len
	case socksDomain:
		size := make([]byte, 1)
		if _, err := io.ReadFull(conn, size); err != nil {
			return nil, fmt.Errorf("read SOCKS5 domain length: %w", err)
		}
		length = int(size[0])
	default:
		return nil, fmt.Errorf("unsupported SOCKS5 address type: %d", addressType)
	}
	address := make([]byte, length+2)
	if _, err := io.ReadFull(conn, address); err != nil {
		return nil, fmt.Errorf("read SOCKS5 address: %w", err)
	}
	return address, nil
}

func addressHost(addressType byte, address []byte) string {
	if addressType == socksDomain {
		return string(address)
	}
	return net.IP(address).String()
}

func writeSOCKS5Reply(conn net.Conn, reply byte) error {
	_, err := conn.Write([]byte{socksVersion5, reply, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if err != nil {
		return fmt.Errorf("write SOCKS5 reply: %w", err)
	}
	return nil
}
