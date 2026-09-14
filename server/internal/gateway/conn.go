package gateway

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"cellular-phagocyte/server/internal/protocol"
)

const (
	reliableBuffer = 256
	writeWait      = 5 * time.Second
	pongWait       = 60 * time.Second
	pingInterval   = 25 * time.Second
)

// wsConn 把 gorilla websocket 适配成 game.Conn。
//
// 消息分为两类：
//   - Send: 可靠控制消息。保持顺序；队列溢出时主动断开慢客户端，由重连机制恢复，绝不静默丢弃。
//   - SendSnapshot: 高频 ROOM_SNAPSHOT。只保留尚未写出的最新一帧，避免慢客户端反压 Tick。
type wsConn struct {
	ws *websocket.Conn

	reliable chan protocol.Envelope

	snapshotMu    sync.Mutex
	latestSnapshot protocol.Envelope
	hasSnapshot    bool
	snapshotReady  chan struct{}

	done      chan struct{}
	closeOnce sync.Once
}

func newWSConn(ws *websocket.Conn) *wsConn {
	return &wsConn{
		ws:            ws,
		reliable:      make(chan protocol.Envelope, reliableBuffer),
		snapshotReady: make(chan struct{}, 1),
		done:          make(chan struct{}),
	}
}

// Send 非阻塞地排队可靠控制消息。
// 如果客户端持续写不出去导致可靠队列耗尽，直接关闭连接，避免把 GAME_END、
// SETTLEMENT_RESULT、RECONNECT_RESULT 等关键消息静默丢掉。
func (c *wsConn) Send(env protocol.Envelope) {
	select {
	case <-c.done:
		return
	default:
	}

	select {
	case c.reliable <- env:
	case <-c.done:
		return
	default:
		c.Close()
	}
}

// SendSnapshot 非阻塞地更新待发送的最新快照。
// 如果旧快照尚未写出，会被新快照覆盖；客户端最终只需要追上最新权威状态。
func (c *wsConn) SendSnapshot(env protocol.Envelope) {
	select {
	case <-c.done:
		return
	default:
	}

	c.snapshotMu.Lock()
	c.latestSnapshot = env
	c.hasSnapshot = true
	c.snapshotMu.Unlock()

	// 通知只需要一个：真正写出时会读取当时最新的快照。
	select {
	case c.snapshotReady <- struct{}{}:
	default:
	}
}

func (c *wsConn) takeLatestSnapshot() (protocol.Envelope, bool) {
	c.snapshotMu.Lock()
	defer c.snapshotMu.Unlock()
	if !c.hasSnapshot {
		return protocol.Envelope{}, false
	}
	env := c.latestSnapshot
	c.hasSnapshot = false
	return env, true
}

// Close 仅终止连接一次。
func (c *wsConn) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		if c.ws != nil {
			_ = c.ws.Close()
		}
	})
}

func (c *wsConn) writeEnvelope(env protocol.Envelope) bool {
	_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
	if err := c.ws.WriteJSON(env); err != nil {
		c.Close()
		return false
	}
	return true
}

// writePump 串行写 WebSocket。可靠控制消息优先于可覆盖的快照消息。
func (c *wsConn) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		// 先快速排空一个可靠消息，避免快照持续就绪时抢占控制消息。
		select {
		case <-c.done:
			return
		case env := <-c.reliable:
			if !c.writeEnvelope(env) {
				return
			}
			continue
		default:
		}

		select {
		case <-c.done:
			return
		case env := <-c.reliable:
			if !c.writeEnvelope(env) {
				return
			}
		case <-c.snapshotReady:
			if env, ok := c.takeLatestSnapshot(); ok {
				if !c.writeEnvelope(env) {
					return
				}
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.Close()
				return
			}
		}
	}
}
