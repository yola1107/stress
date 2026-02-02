package biz

import (
	"context"
	"strconv"
	"time"

	"stress/internal/biz/game"
	"stress/internal/biz/game/base"
	"stress/internal/biz/member"
	"stress/internal/biz/stats"
	"stress/internal/biz/task"
	"stress/internal/conf"
	"stress/internal/notify"

	"github.com/go-kratos/kratos/v2/log"
)

// 业务常量
const (
	cleanupTimeout   = 10 * time.Minute
	memberLoadBatch  = 1000
	memberBalance    = 10000
	memberNameOffset = 1000
)

// OrderScope 订单查询范围
type OrderScope struct {
	GameID     int64
	Merchant   string
	StartTime  time.Time
	EndTime    time.Time
	ExcludeAmt float64
}

// QueryFilter 订单查询过滤器
type QueryFilter struct {
	GameID        int
	Merchant      string
	Member        string
	StartTime     string
	EndTime       string
	ExcludeAmount float64
}

// DataRepo 数据层接口：成员/订单/清理/任务ID计数
type DataRepo interface {
	BatchUpsertMembers(ctx context.Context, members []member.Info) error
	CleanRedisBySites(ctx context.Context, sites []string) error
	CleanGameOrderTable(ctx context.Context) error
	GetGameOrderCount(ctx context.Context) (int64, error)
	GetOrderCountByScope(ctx context.Context, scope OrderScope) (int64, error)
	DeleteOrdersByScope(ctx context.Context, scope OrderScope) (int64, error)
	GetDetailedOrderAmounts(ctx context.Context) (totalBet, totalWin, betOrderCount, bonusOrderCount int64, err error)
	NextTaskID(ctx context.Context, gameID int64) (string, error)
	QueryGameOrderPoints(ctx context.Context, filter QueryFilter) ([]stats.Point, error)
}

// UseCase 编排层：通过 DataRepo + 领域池（Game/Task/Member）编排业务
type UseCase struct {
	ctx    context.Context
	cancel context.CancelFunc

	repo       DataRepo
	log        *log.Helper
	c          *conf.Launch
	gamePool   *game.Pool
	taskPool   *task.Pool
	memberPool *member.Pool

	notify notify.Notifier
	stats  *stats.Component
}

// NewUseCase 创建 UseCase
func NewUseCase(repo DataRepo, logger log.Logger, c *conf.Launch, notify notify.Notifier) (*UseCase, func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	uc := &UseCase{
		ctx:        ctx,
		cancel:     cancel,
		repo:       repo,
		log:        log.NewHelper(logger),
		c:          c,
		gamePool:   game.NewPool(),
		taskPool:   task.NewTaskPool(),
		memberPool: member.NewMemberPool(),
		notify:     notify,
		stats:      stats.New(),
	}

	// 启动时自清理：Redis site:* + 订单表，避免上次压测残留
	uc.cleanOnStartup()

	// 启动成员自动加载
	if c.AutoLoads {
		go uc.runMemberLoader()
	}

	cleanup := func() { uc.cancel() }
	return uc, cleanup, nil
}

// GetGame 按 gameID 获取游戏
func (uc *UseCase) GetGame(gameID int64) (base.IGame, bool) {
	return uc.gamePool.Get(gameID)
}

// ListGames 返回游戏列表副本（按 GameID 升序）
func (uc *UseCase) ListGames() []base.IGame {
	return uc.gamePool.List()
}

// GetTask 按 ID 获取任务
func (uc *UseCase) GetTask(id string) (*task.Task, bool) {
	return uc.taskPool.Get(id)
}

// ListTasks 返回所有任务（已按创建时间倒序）
func (uc *UseCase) ListTasks() []*task.Task {
	return uc.taskPool.List()
}

// GetMemberStats 玩家池统计
func (uc *UseCase) GetMemberStats() (idle, allocated, total int) {
	return uc.memberPool.Stats()
}

// cleanOnStartup 启动时清理 Redis 与订单表
func (uc *UseCase) cleanOnStartup() {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	if err := uc.repo.CleanRedisBySites(ctx, uc.c.Sites); err != nil {
		uc.log.Warnf("startup clean Redis: %v", err)
	}
	if err := uc.repo.CleanGameOrderTable(ctx); err != nil {
		uc.log.Warnf("startup clean order table: %v", err)
	} else {
		uc.log.Info("startup clean order table done")
	}
}

// runMemberLoader 按间隔生成成员、落库、加入空闲池
func (uc *UseCase) runMemberLoader() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	var loaded int32
	for loaded < uc.c.MaxLoadTotal {
		select {
		case <-uc.ctx.Done():
			return
		case <-ticker.C:
			n := min32(memberLoadBatch, uc.c.MaxLoadTotal-loaded)
			if n <= 0 {
				continue
			}
			batch := make([]member.Info, n)
			for i := int32(0); i < n; i++ {
				loaded++
				batch[i] = member.Info{
					Name:    member.DefaultMemberNamePrefix + strconv.FormatInt(int64(loaded+memberNameOffset), 10),
					Balance: memberBalance,
				}
			}
			if err := uc.repo.BatchUpsertMembers(uc.ctx, batch); err != nil {
				uc.log.Errorf("BatchUpsertMembers: %v", err)
				loaded -= n
				continue
			}
			uc.memberPool.AddIdle(batch)
			_, _, total := uc.memberPool.Stats()
			uc.log.Infof("Loaded %d members, total: %d", len(batch), total)
			uc.Schedule()
		}
	}
	uc.log.Info("Member loading completed")
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}
