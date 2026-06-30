package settlement

import (
	"fmt"
	"log/slog"
	"sync"

	"cellular-phagocyte/server/internal/game"
	"cellular-phagocyte/server/internal/protocol"
	"cellular-phagocyte/server/internal/user"
)

// Service 计算奖励、（幂等地）发放资产，并保存每个玩家的结算结果以供后续查询。
// 它实现了 game.Settler 接口。
type Service struct {
	cfg   RewardConfig
	users *user.Service
	log   *slog.Logger

	mu     sync.RWMutex
	byRoom map[string]map[string]protocol.SettlementResultData // roomId -> userId -> 结果
	latest map[string]string                                   // userId -> 最近一次的 roomId
}

// NewService 创建一个结算服务。
func NewService(users *user.Service, log *slog.Logger) *Service {
	return &Service{
		cfg:    DefaultRewardConfig(),
		users:  users,
		log:    log,
		byRoom: make(map[string]map[string]protocol.SettlementResultData),
		latest: make(map[string]string),
	}
}

// Settle 处理一个已结束的房间：为真人玩家计算并发放奖励，返回他们的结算结果
//（不含机器人）。
func (s *Service) Settle(req *game.SettleRequest) []protocol.SettlementResultData {
	out := make([]protocol.SettlementResultData, 0)
	for _, p := range req.Players {
		if p.IsBot {
			continue
		}
		reward := CalcReward(s.cfg, PlayerResult{
			Rank:           p.Rank,
			EatPlayerCount: p.EatPlayerCount,
			EatFoodCount:   p.EatFoodCount,
			AliveSeconds:   p.AliveSeconds,
		})

		bizID := fmt.Sprintf("settlement:%s:%s", req.RoomID, p.UserID)
		if _, err := s.users.GrantAssets(p.UserID, bizID, []user.GrantItem{
			{AssetType: "COIN", Amount: reward.Coin},
			{AssetType: "EXP", Amount: reward.Exp},
		}); err != nil {
			s.log.Error("settlement_failed", "roomId", req.RoomID, "userId", p.UserID, "err", err)
			continue
		}

		res := protocol.SettlementResultData{
			RoomID:         req.RoomID,
			UserID:         p.UserID,
			Rank:           p.Rank,
			TotalPlayers:   req.TotalPlayers,
			FinalScore:     p.FinalScore,
			MaxMass:        p.MaxMass,
			EatPlayerCount: p.EatPlayerCount,
			EatFoodCount:   p.EatFoodCount,
			AliveSeconds:   p.AliveSeconds,
			CoinReward:     reward.Coin,
			ExpReward:      reward.Exp,
			IsBestScore:    true,
			Status:         "SUCCESS",
		}
		s.storeResult(res)
		out = append(out, res)
		s.log.Info("settlement_success", "roomId", req.RoomID, "userId", p.UserID,
			"rank", p.Rank, "coin", reward.Coin, "exp", reward.Exp)
	}
	return out
}

func (s *Service) storeResult(res protocol.SettlementResultData) {
	s.mu.Lock()
	defer s.mu.Unlock()
	room := s.byRoom[res.RoomID]
	if room == nil {
		room = make(map[string]protocol.SettlementResultData)
		s.byRoom[res.RoomID] = room
	}
	room[res.UserID] = res
	s.latest[res.UserID] = res.RoomID
}

// Result 返回某玩家在某房间的结算结果。
func (s *Service) Result(roomID, userID string) (protocol.SettlementResultData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	room, ok := s.byRoom[roomID]
	if !ok {
		return protocol.SettlementResultData{}, false
	}
	res, ok := room[userID]
	return res, ok
}

// LatestResult 返回某玩家最近一次的结算结果。
func (s *Service) LatestResult(userID string) (protocol.SettlementResultData, bool) {
	s.mu.RLock()
	roomID, ok := s.latest[userID]
	s.mu.RUnlock()
	if !ok {
		return protocol.SettlementResultData{}, false
	}
	return s.Result(roomID, userID)
}
