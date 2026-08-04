package control

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

func SendUDP(ctx context.Context, addr string, payload []byte) (int, time.Duration, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" || strings.HasPrefix(addr, ":") {
		return 0, 0, fmt.Errorf("UDP地址不能为空")
	}
	if len(payload) == 0 {
		return 0, 0, fmt.Errorf("UDP指令内容不能为空")
	}
	start := time.Now()
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "udp", addr)
	if err != nil {
		return 0, time.Since(start), err
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Write(payload)
	return n, time.Since(start), err
}

func payloadHex(payload []byte) string {
	return strings.ToUpper(hex.EncodeToString(payload))
}
