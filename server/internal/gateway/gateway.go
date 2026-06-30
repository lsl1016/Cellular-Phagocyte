// Package gateway 承载 WebSocket 端点：通过入房凭证认证玩家、将连接绑定到房间，
// 并路由客户端输入。
package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"cellular-phagocyte/server/internal/game"
	"cellular-phagocyte/server/internal/protocol"
)

// Gateway 负责升级 HTTP 连接并把 WebSocket 消息路由到房间。
type Gateway struct {
	mgr      *game.Manager
	log      *slog.Logger
	upgrader websocket.Upgrader
}

// New 创建一个网关。
func New(mgr *game.Manager, log *slog.Logger) *Gateway {
	return &Gateway{
		mgr: mgr,
		log: log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(*http.Request) bool { return true },
		},
	}
}

// Handle 是 WebSocket 路径的 http.HandlerFunc。
func (g *Gateway) Handle(w http.ResponseWriter, r *http.Request) {
	ws, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.log.Warn("ws_upgrade_failed", "err", err)
		return
	}
	conn := newWSConn(ws)
	go conn.writePump()
	g.serve(conn)
}

// serve 处理 ENTER_ROOM 握手，随后路由后续消息。
func (g *Gateway) serve(conn *wsConn) {
	defer conn.Close()

	ws := conn.ws
	_ = ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	// 第一条消息必须是 ENTER_ROOM 或 RECONNECT。
	var first protocol.Envelope
	if err := ws.ReadJSON(&first); err != nil {
		return
	}

	switch first.Type {
	case protocol.TypeEnterRoom:
		g.serveEnter(conn, first)
	case protocol.TypeReconnect:
		g.serveReconnect(conn, first)
	default:
		conn.Send(errEnvelope(40002, "expected ENTER_ROOM or RECONNECT"))
	}
}

func (g *Gateway) serveEnter(conn *wsConn, first protocol.Envelope) {
	var ed protocol.EnterRoomData
	_ = json.Unmarshal(first.Data, &ed)

	room, ok := g.mgr.ValidateEnterToken(ed.EnterToken, ed.RoomID, ed.UserID)
	if !ok {
		conn.Send(protocol.Envelope{
			Type: protocol.TypeEnterRoomResult,
			Seq:  first.Seq,
			Data: protocol.MustMarshal(protocol.EnterRoomResultData{
				Success: false, ErrorCode: 30011, Message: "入房凭证已过期，请重新匹配",
			}),
		})
		return
	}

	result, ok := room.AttachConn(ed.UserID, conn)
	conn.Send(protocol.Envelope{
		Type: protocol.TypeEnterRoomResult,
		Seq:  first.Seq,
		Data: protocol.MustMarshal(result),
	})
	if !ok {
		return
	}
	g.log.Info("ws_auth_success", "roomId", ed.RoomID, "userId", ed.UserID)

	defer room.Disconnect(ed.UserID, conn)
	g.readLoop(conn, room, ed.UserID)
}

func (g *Gateway) serveReconnect(conn *wsConn, first protocol.Envelope) {
	var rd protocol.ReconnectData
	_ = json.Unmarshal(first.Data, &rd)

	room, ok := g.mgr.ValidateReconnectToken(rd.ReconnectToken, rd.RoomID, rd.UserID)
	if !ok {
		conn.Send(protocol.Envelope{
			Type: protocol.TypeReconnectResult,
			Seq:  first.Seq,
			Data: protocol.MustMarshal(protocol.ReconnectResultData{
				Success: false, Reason: "TOKEN_INVALID", Message: "重连凭证无效或已过期",
			}),
		})
		return
	}

	result, recoverSnap, ok := room.Reconnect(rd.UserID, conn)
	conn.Send(protocol.Envelope{
		Type: protocol.TypeReconnectResult,
		Seq:  first.Seq,
		Data: protocol.MustMarshal(result),
	})
	if !ok {
		return
	}
	if recoverSnap != nil {
		conn.Send(protocol.Envelope{
			Type:       protocol.TypeRoomRecover,
			ServerTime: time.Now().UnixMilli(),
			Data:       protocol.MustMarshal(recoverSnap),
		})
	}
	g.log.Info("reconnect_success", "roomId", rd.RoomID, "userId", rd.UserID)

	defer room.Disconnect(rd.UserID, conn)
	g.readLoop(conn, room, rd.UserID)
}

func (g *Gateway) readLoop(conn *wsConn, room *game.Room, userID string) {
	ws := conn.ws
	for {
		var env protocol.Envelope
		if err := ws.ReadJSON(&env); err != nil {
			g.log.Info("ws_disconnected", "userId", userID, "err", err.Error())
			return
		}
		switch env.Type {
		case protocol.TypeReady:
			room.MarkReady(userID)
		case protocol.TypeMove:
			var in protocol.InputData
			if err := json.Unmarshal(env.Data, &in); err == nil {
				room.SubmitInput(userID, env.Seq, in.Direction)
			}
		case protocol.TypeSplit:
			var in protocol.InputData
			if err := json.Unmarshal(env.Data, &in); err == nil {
				room.Split(userID, in.Direction)
			}
		case protocol.TypeEject:
			var in protocol.InputData
			if err := json.Unmarshal(env.Data, &in); err == nil {
				room.Eject(userID, in.Direction)
			}
		case protocol.TypePing:
			conn.Send(protocol.Envelope{Type: protocol.TypePong, ServerTime: time.Now().UnixMilli()})
		}
	}
}

func errEnvelope(code int, msg string) protocol.Envelope {
	return protocol.Envelope{
		Type: protocol.TypeError,
		Data: protocol.MustMarshal(protocol.ErrorData{Code: code, Message: msg}),
	}
}
