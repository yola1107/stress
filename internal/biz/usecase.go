package biz

import (
	"context"
	"time"

	"stress/internal/biz/chart"
	"stress/internal/biz/game"
	"stress/internal/biz/game/base"
	"stress/internal/biz/member"
	"stress/internal/biz/task"
	"stress/internal/conf"
	"stress/internal/notify"

	"github.com/go-kratos/kratos/v2/log"
)

// 业务常量
const (
	cleanupTimeout      = 10 * time.Minute
	taskRetentionPeriod = 24 * time.Hour // 任务保留时长
	taskCleanupInterval = 1 * time.Hour  // 清理任务执行间隔
)

// DataRepo 数据层接口：成员/订单/清理/任务ID计数
type DataRepo interface {
	// 成员管理
	BatchUpsertMembers(ctx context.Context, members []member.Info) error

	// Redis清理
	CleanRedisBySites(ctx context.Context, sites []string) error

	// 订单表操作
	CleanGameOrderTable(ctx context.Context) error
	DeleteOrdersByScope(ctx context.Context, scope task.OrderScope) (int64, error)

	// 订单统计查询
	GetGameOrderCount(ctx context.Context) (int64, error)
	GetOrderCountByScope(ctx context.Context, scope task.OrderScope) (int64, error)
	GetDetailedOrderAmounts(ctx context.Context, scope task.OrderScope) (totalBet, totalWin, betOrderCount, bonusOrderCount int64, err error)
	QueryGameOrderPoints(ctx context.Context, scope task.OrderScope) ([]chart.Point, error)

	// 任务ID生成
	NextTaskID(ctx context.Context, gameID int64) (string, error)

	// S3 上传
	UploadBytes(ctx context.Context, bucket, key, contentType string, data []byte) (string, error)

	// GameBetSizes
	GetGameBetSize(ctx context.Context, gameIDs []int64) (map[int64][]float64, error)
}

// UseCase 编排层：通过 DataRepo + 领域池（Game/Task/Member）编排业务
type UseCase struct {
	ctx    context.Context
	cancel context.CancelFunc

	repo       DataRepo
	log        *log.Helper
	conf       *conf.Stress
	gamePool   *game.Pool
	taskPool   *task.Pool
	memberPool *member.Pool

	notify notify.Notifier
	chart  chart.IGenerator

	scheduleCh chan struct{} // 调度触发信号
}

// NewUseCase 创建 UseCase
func NewUseCase(repo DataRepo, logger log.Logger, c *conf.Stress, notify notify.Notifier, chart chart.IGenerator) (*UseCase, func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	uc := &UseCase{
		ctx:        ctx,
		cancel:     cancel,
		repo:       repo,
		log:        log.NewHelper(logger),
		conf:       c,
		gamePool:   game.NewPool(repo.GetGameBetSize),
		taskPool:   task.NewTaskPool(),
		memberPool: member.NewMemberPool(),
		notify:     notify,
		chart:      chart,
		scheduleCh: make(chan struct{}, 1),
	}

	// 启动调度器
	go uc.scheduleLoop()

	cleanCtx, cleanCancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cleanCancel()
	if err := repo.CleanRedisBySites(cleanCtx, c.Launch.Sites); err != nil {
		log.NewHelper(logger).Warnf("startup clean Redis: %v", err)
	}
	if err := repo.CleanGameOrderTable(cleanCtx); err != nil {
		log.NewHelper(logger).Warnf("startup clean order table: %v", err)
	}

	go uc.memberPool.StartAutoLoad(ctx, member.LoaderConfig{
		AutoLoads:     c.Member.AutoLoads,
		IntervalSec:   c.Member.IntervalSec,
		MaxLoadTotal:  c.Member.MaxLoadTotal,
		BatchLoadSize: c.Member.BatchLoadSize,
		MemberPrefix:  c.Member.MemberPrefix,
	}, repo, logger, uc.WakeScheduler)

	go uc.taskPool.StartAutoCleanup(ctx, logger, taskRetentionPeriod, taskCleanupInterval)

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

// GetStressConfig 获取压测配置
func (uc *UseCase) GetStressConfig() *conf.Stress {
	return uc.conf
}

// GetMemberStats 玩家池统计
func (uc *UseCase) GetMemberStats() (idle, allocated, total int) {
	return uc.memberPool.Stats()
}
