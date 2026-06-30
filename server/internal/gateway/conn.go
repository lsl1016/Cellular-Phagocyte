package gateway

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"cellular-phagocyte/server/internal/protocol"
)

const (
	sendBuffer   = 256
	writeWait    = 5 * time.Second
	pongWait     = 60 * time.Second
	pingInterval = 25 * time.Second
)

// wsConn 把 gorilla 的 websocket 适配成 game.Conn 接口。Send 是非阻塞的：
// 当缓冲已满时丢弃消息，从而保证 Tick 循环不会被慢客户端拖住。
type wsConn struct {
	ws        *websocket.Conn
	send      chan protocol.Envelope
	done      chan struct{}
	closeOnce sync.Once
}

func newWSConn(ws *websocket.Conn) *wsConn {
	return &wsConn{
		ws:   ws,
		send: make(chan protocol.Envelope, sendBuffer),
		done: make(chan struct{}),
	}
}

// Send 将信封排队等待发送，若缓冲已满则丢弃。
func (c *wsConn) Send(env protocol.Envelope) {
	select {
	case c.send <- env:
	case <-c.done:
	default:
		// 缓冲已满：丢弃（该客户端处理本次快照太慢）
	}
}

// Close 仅终止连接一次。
func (c *wsConn) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.ws.Close()
	})
}

// writePump 将排队的信封写入套接字，并周期性发送 ping。
func (c *wsConn) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()
	for {
		select {
		case env := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteJSON(env); err != nil {
				c.Close()
				return
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.Close()
				return
			}
		case <-c.done:
			return
		}
	}
}
