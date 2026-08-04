package foxglove

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	foxgloveSubprotocol = "foxglove.websocket.v1"
	binaryDataOp        = byte(1)
)

type FetchOptions struct {
	WSURL   string
	Topic   string
	Timeout time.Duration
}

type Channel struct {
	ID         uint32 `json:"id"`
	Topic      string `json:"topic"`
	SchemaName string `json:"schemaName"`
	Encoding   string `json:"encoding"`
}

type BinaryMessage struct {
	Op             byte   `json:"op"`
	SubscriptionID uint32 `json:"subscription_id"`
	Timestamp      uint64 `json:"timestamp"`
	Payload        []byte `json:"-"`
}

type PoseMessage struct {
	WSURL            string           `json:"ws_url"`
	Topic            string           `json:"topic"`
	SchemaName       string           `json:"schema_name"`
	Encoding         string           `json:"encoding"`
	SubscriptionID   uint32           `json:"subscription_id"`
	Timestamp        uint64           `json:"timestamp"`
	ReceivedAt       time.Time        `json:"received_at"`
	Decoded          *Odometry        `json:"decoded"`
	NavCustomPayload NavCustomPayload `json:"navCustomPayload"`
}

func FetchLatestOdometry(ctx context.Context, opts FetchOptions) (*PoseMessage, error) {
	cfg := LoadConfig()
	if opts.WSURL == "" {
		opts.WSURL = cfg.FoxgloveWSURL
	}
	if opts.Topic == "" {
		opts.Topic = cfg.FoxgloveTopic
	}
	if opts.Timeout <= 0 {
		opts.Timeout = cfg.Timeout()
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: opts.Timeout,
		Subprotocols:     []string{foxgloveSubprotocol},
	}
	conn, _, err := dialer.DialContext(ctx, opts.WSURL, nil)
	if err != nil {
		return nil, fmt.Errorf("连接Foxglove WS失败: %w", err)
	}
	defer conn.Close()

	subscriptionID := uint32(1)
	channels := map[uint32]Channel{}
	subscribedChannelID := uint32(0)

	for {
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetReadDeadline(deadline)
		}
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("读取Foxglove WS消息失败: %w", err)
		}
		switch msgType {
		case websocket.TextMessage:
			channel, ok, err := handleTextMessage(data, opts.Topic, channels)
			if err != nil {
				return nil, err
			}
			if ok && subscribedChannelID == 0 {
				if err := subscribe(conn, subscriptionID, channel.ID); err != nil {
					return nil, err
				}
				channels[channel.ID] = channel
				subscribedChannelID = channel.ID
			}
		case websocket.BinaryMessage:
			msg, err := parseBinaryMessage(data)
			if err != nil {
				return nil, err
			}
			if msg.SubscriptionID != subscriptionID || subscribedChannelID == 0 {
				continue
			}
			channel := channels[subscribedChannelID]
			decoded, err := DecodeOdometry(msg.Payload)
			if err != nil {
				return nil, err
			}
			return &PoseMessage{
				WSURL:            opts.WSURL,
				Topic:            channel.Topic,
				SchemaName:       channel.SchemaName,
				Encoding:         channel.Encoding,
				SubscriptionID:   msg.SubscriptionID,
				Timestamp:        msg.Timestamp,
				ReceivedAt:       time.Now(),
				Decoded:          decoded,
				NavCustomPayload: decoded.NavCustomPayload,
			}, nil
		}
	}
}

func handleTextMessage(data []byte, topic string, channels map[uint32]Channel) (Channel, bool, error) {
	var envelope struct {
		Op       string    `json:"op"`
		Channels []Channel `json:"channels"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Channel{}, false, fmt.Errorf("解析Foxglove文本消息失败: %w", err)
	}
	switch envelope.Op {
	case "advertise":
		for _, channel := range envelope.Channels {
			channels[channel.ID] = channel
			if channel.Topic == topic {
				return channel, true, nil
			}
		}
	case "unadvertise":
		for _, channel := range envelope.Channels {
			delete(channels, channel.ID)
		}
	}
	return Channel{}, false, nil
}

func subscribe(conn *websocket.Conn, subscriptionID uint32, channelID uint32) error {
	payload := map[string]interface{}{
		"op": "subscribe",
		"subscriptions": []map[string]uint32{{
			"id":        subscriptionID,
			"channelId": channelID,
		}},
	}
	if err := conn.WriteJSON(payload); err != nil {
		return fmt.Errorf("订阅Foxglove channel失败: %w", err)
	}
	return nil
}

func parseBinaryMessage(data []byte) (*BinaryMessage, error) {
	if len(data) < 13 {
		return nil, fmt.Errorf("Foxglove二进制消息长度不足: %d", len(data))
	}
	op := data[0]
	if op != binaryDataOp {
		return nil, fmt.Errorf("不支持的Foxglove二进制op: %d", op)
	}
	return &BinaryMessage{
		Op:             op,
		SubscriptionID: binary.LittleEndian.Uint32(data[1:5]),
		Timestamp:      binary.LittleEndian.Uint64(data[5:13]),
		Payload:        data[13:],
	}, nil
}
