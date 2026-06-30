package redisstore

import (
	"encoding/json"

	"cellular-phagocyte/server/internal/protocol"
)

// SettlementStore 是 settlement.Store 的 Redis 实现。
//   settle:{roomID}:{userID} -> JSON 结算结果
//   settle:latest:{userID}   -> roomID（最近一次）
type SettlementStore struct{ c *Client }

// NewSettlementStore 创建 Redis 结算结果存储。
func NewSettlementStore(c *Client) *SettlementStore { return &SettlementStore{c: c} }

func (s *SettlementStore) SaveResult(res protocol.SettlementResultData) {
	c, cancel := ctx()
	defer cancel()
	raw, _ := json.Marshal(res)
	s.c.rdb.Set(c, "settle:"+res.RoomID+":"+res.UserID, raw, 0)
	s.c.rdb.Set(c, "settle:latest:"+res.UserID, res.RoomID, 0)
}

func (s *SettlementStore) Result(roomID, userID string) (protocol.SettlementResultData, bool) {
	c, cancel := ctx()
	defer cancel()
	raw, err := s.c.rdb.Get(c, "settle:"+roomID+":"+userID).Bytes()
	if err != nil {
		return protocol.SettlementResultData{}, false
	}
	var res protocol.SettlementResultData
	if json.Unmarshal(raw, &res) != nil {
		return protocol.SettlementResultData{}, false
	}
	return res, true
}

func (s *SettlementStore) LatestResult(userID string) (protocol.SettlementResultData, bool) {
	c, cancel := ctx()
	defer cancel()
	roomID, err := s.c.rdb.Get(c, "settle:latest:"+userID).Result()
	if err != nil {
		return protocol.SettlementResultData{}, false
	}
	return s.Result(roomID, userID)
}
