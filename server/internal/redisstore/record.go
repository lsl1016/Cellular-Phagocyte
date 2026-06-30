package redisstore

import (
	"encoding/json"

	"cellular-phagocyte/server/internal/record"
)

// RecordStore 是 record.Store 的 Redis 实现。
//   record:user:{userID}        -> LIST，按时间升序追加的 JSON 战绩
//   record:room:{roomID}:{user} -> JSON 单局详情
type RecordStore struct{ c *Client }

// NewRecordStore 创建 Redis 战绩存储。
func NewRecordStore(c *Client) *RecordStore { return &RecordStore{c: c} }

func (s *RecordStore) Append(userID string, e record.Entry) {
	c, cancel := ctx()
	defer cancel()
	raw, _ := json.Marshal(e)
	s.c.rdb.RPush(c, "record:user:"+userID, raw)
	s.c.rdb.Set(c, "record:room:"+e.RoomID+":"+userID, raw, 0)
}

func (s *RecordStore) ListByUser(userID string) []record.Entry {
	c, cancel := ctx()
	defer cancel()
	raws, err := s.c.rdb.LRange(c, "record:user:"+userID, 0, -1).Result()
	if err != nil {
		return nil
	}
	out := make([]record.Entry, 0, len(raws))
	for _, raw := range raws {
		var e record.Entry
		if json.Unmarshal([]byte(raw), &e) == nil {
			out = append(out, e)
		}
	}
	return out
}

func (s *RecordStore) Get(roomID, userID string) (record.Entry, bool) {
	c, cancel := ctx()
	defer cancel()
	raw, err := s.c.rdb.Get(c, "record:room:"+roomID+":"+userID).Bytes()
	if err != nil {
		return record.Entry{}, false
	}
	var e record.Entry
	if json.Unmarshal(raw, &e) != nil {
		return record.Entry{}, false
	}
	return e, true
}
