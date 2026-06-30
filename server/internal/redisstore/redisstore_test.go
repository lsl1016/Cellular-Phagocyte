package redisstore

import (
	"testing"
	"time"

	"cellular-phagocyte/server/internal/game"
	"cellular-phagocyte/server/internal/match"
	"cellular-phagocyte/server/internal/protocol"
	"cellular-phagocyte/server/internal/record"
	"cellular-phagocyte/server/internal/user"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestClient 启动一个 miniredis 并返回连接其上的 Client。
func newTestClient(t *testing.T) *Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewClientWithRedis(rdb)
}

func TestUserStore(t *testing.T) {
	s := NewUserStore(newTestClient(t))

	if seq := s.NextUserSeq(); seq != 1 {
		t.Fatalf("首个 seq 应为 1, got %d", seq)
	}
	if s.NextUserSeq() != 2 {
		t.Fatal("seq 应自增")
	}

	u := &user.User{UserID: "10001", Nickname: "A", UserType: "GUEST", Status: "ACTIVE", DeviceID: "dev1", CreatedAt: 123}
	s.SaveUser(u)
	got, ok := s.GetUser("10001")
	if !ok || got.Nickname != "A" || got.DeviceID != "dev1" || got.CreatedAt != 123 {
		t.Fatalf("用户应完整回读, got %+v ok=%v", got, ok)
	}
	if uid, ok := s.UserIDByDevice("dev1"); !ok || uid != "10001" {
		t.Fatalf("设备索引应指向 10001, got %s ok=%v", uid, ok)
	}

	s.SaveAsset(user.Asset{UserID: "10001", Coin: 50, Exp: 30, Level: 2})
	a, ok := s.GetAsset("10001")
	if !ok || a.Coin != 50 || a.Level != 2 {
		t.Fatalf("资产应回读, got %+v ok=%v", a, ok)
	}

	s.PutToken("tok1", "10001")
	if uid, ok := s.UserIDByToken("tok1"); !ok || uid != "10001" {
		t.Fatalf("令牌应解析为 10001, got %s ok=%v", uid, ok)
	}

	if !s.MarkBiz("b1") {
		t.Fatal("首次标记应返回 true")
	}
	if s.MarkBiz("b1") {
		t.Fatal("重复标记应返回 false（幂等）")
	}

	if _, ok := s.GetUser("nope"); ok {
		t.Fatal("未知用户应返回 false")
	}
}

func TestMatchStore(t *testing.T) {
	s := NewMatchStore(newTestClient(t))

	e := match.Entry{MatchID: "m1", UserID: "u1", Mode: "classic", Status: match.StatusMatching, JoinedAt: time.Now().UnixMilli()}
	s.Save(e)

	if got, ok := s.ByMatch("m1"); !ok || got.UserID != "u1" {
		t.Fatalf("应按 matchID 查到, got %+v ok=%v", got, ok)
	}
	if got, ok := s.ByUser("u1"); !ok || got.MatchID != "m1" {
		t.Fatalf("应按 userID 查到, got %+v ok=%v", got, ok)
	}
	if list := s.ListMatching(); len(list) != 1 {
		t.Fatalf("应有 1 条 MATCHING, got %d", len(list))
	}

	// 标记为 MATCHED 后应移出 matching 集合
	e.Status = match.StatusMatched
	e.RoomID = "r1"
	s.Save(e)
	if list := s.ListMatching(); len(list) != 0 {
		t.Fatalf("MATCHED 后不应出现在 matching, got %d", len(list))
	}
	if got, _ := s.ByMatch("m1"); got.RoomID != "r1" {
		t.Fatalf("应回读 roomId, got %s", got.RoomID)
	}

	s.DeleteUser("u1")
	if _, ok := s.ByUser("u1"); ok {
		t.Fatal("DeleteUser 后不应再查到")
	}
}

func TestTokenStore(t *testing.T) {
	s := NewTokenStore(newTestClient(t))
	exp := time.Now().UnixMilli() + 60000

	s.PutEnter("e1", game.TokenInfo{RoomID: "r1", UserID: "u1", ExpireAt: exp})
	if info, ok := s.GetEnter("e1"); !ok || info.RoomID != "r1" {
		t.Fatalf("入房凭证应回读, got %+v ok=%v", info, ok)
	}
	s.PutRecon("c1", game.TokenInfo{RoomID: "r1", UserID: "u1", ExpireAt: exp})
	if _, ok := s.GetRecon("c1"); !ok {
		t.Fatal("重连凭证应回读")
	}

	// 过期凭证应不可用
	s.PutEnter("e2", game.TokenInfo{RoomID: "r2", UserID: "u2", ExpireAt: time.Now().UnixMilli() - 1})
	if _, ok := s.GetEnter("e2"); ok {
		t.Fatal("过期凭证应返回 false")
	}

	s.DeleteByRoom("r1")
	if _, ok := s.GetEnter("e1"); ok {
		t.Fatal("DeleteByRoom 后入房凭证应清除")
	}
	if _, ok := s.GetRecon("c1"); ok {
		t.Fatal("DeleteByRoom 后重连凭证应清除")
	}
}

func TestSettlementStore(t *testing.T) {
	s := NewSettlementStore(newTestClient(t))
	s.SaveResult(protocol.SettlementResultData{RoomID: "r1", UserID: "u1", Rank: 1, FinalScore: 100})
	s.SaveResult(protocol.SettlementResultData{RoomID: "r2", UserID: "u1", Rank: 2, FinalScore: 80})

	if res, ok := s.Result("r1", "u1"); !ok || res.FinalScore != 100 {
		t.Fatalf("应查到 r1 结果, got %+v ok=%v", res, ok)
	}
	if res, ok := s.LatestResult("u1"); !ok || res.RoomID != "r2" {
		t.Fatalf("最近一次应为 r2, got %+v ok=%v", res, ok)
	}
	if _, ok := s.Result("rX", "u1"); ok {
		t.Fatal("未知房间应返回 false")
	}
}

func TestRecordStore(t *testing.T) {
	s := NewRecordStore(newTestClient(t))
	s.Append("u1", record.Entry{RoomID: "r1", FinalScore: 100})
	s.Append("u1", record.Entry{RoomID: "r2", FinalScore: 80})
	s.Append("u2", record.Entry{RoomID: "r3", FinalScore: 200})

	list := s.ListByUser("u1")
	if len(list) != 2 || list[0].RoomID != "r1" || list[1].RoomID != "r2" {
		t.Fatalf("应按追加顺序返回 r1,r2, got %+v", list)
	}
	if e, ok := s.Get("r1", "u1"); !ok || e.FinalScore != 100 {
		t.Fatalf("应查到 r1/u1 详情, got %+v ok=%v", e, ok)
	}
	if _, ok := s.Get("r1", "u2"); ok {
		t.Fatal("不应越权查到他人战绩")
	}
}

func TestRankStore(t *testing.T) {
	s := NewRankStore(newTestClient(t))
	const key = "rank:daily:test"

	s.SetBrief("u1", "Alice")
	if s.Brief("u1") != "Alice" {
		t.Fatal("昵称应回读")
	}

	s.IncrBy(key, "u1", 100)
	s.IncrBy(key, "u1", 50) // 累加 => 150
	s.IncrBy(key, "u2", 200)

	if sc, ok := s.Score(key, "u1"); !ok || sc != 150 {
		t.Fatalf("u1 应累加为 150, got %d ok=%v", sc, ok)
	}
	if s.Card(key) != 2 {
		t.Fatalf("成员数应为 2, got %d", s.Card(key))
	}

	// 降序：u2(200) 在前，u1(150) 其次
	top := s.Range(key, 0, -1)
	if len(top) != 2 || top[0].UserID != "u2" || top[1].UserID != "u1" {
		t.Fatalf("应按分数降序, got %+v", top)
	}
	if r, ok := s.RevRank(key, "u2"); !ok || r != 0 {
		t.Fatalf("u2 应排第 0, got %d ok=%v", r, ok)
	}
	if r, ok := s.RevRank(key, "u1"); !ok || r != 1 {
		t.Fatalf("u1 应排第 1, got %d ok=%v", r, ok)
	}

	// Max 仅在更大时更新
	const bkey = "rank:best_score"
	s.Max(bkey, "u1", 100)
	s.Max(bkey, "u1", 50) // 不应降低
	if sc, _ := s.Score(bkey, "u1"); sc != 100 {
		t.Fatalf("best_score 应取最大 100, got %d", sc)
	}
	s.Max(bkey, "u1", 300)
	if sc, _ := s.Score(bkey, "u1"); sc != 300 {
		t.Fatalf("best_score 应更新为 300, got %d", sc)
	}

	if _, ok := s.RevRank(key, "nope"); ok {
		t.Fatal("未知成员 RevRank 应返回 false")
	}
}
