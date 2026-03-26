package biz

import (
	"context"
	"sync"
	"time"

	"stress/internal/biz/chart"
	"stress/internal/biz/game"
	"stress/internal/biz/game/base"
	"stress/internal/biz/member"
	"stress/internal/biz/notify"
	"stress/internal/biz/task"
	"stress/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
)

// 业务常量
const (
	cleanupTimeout      = 10 * time.Minute
	taskRetentionPeriod = 24 * time.Hour // 任务保留时长
	taskCleanupInterval = 1 * time.Hour  // 清理任务执行间隔
)

// DataRepo 数据层接口，嵌入 task.Repo（任务执行期子集）避免方法重复声明
type DataRepo interface {
	task.Repo

	// BatchUpsertMembers 批量创建或更新压测成员
	BatchUpsertMembers(ctx context.Context, members []member.Info) error
	// DeleteOrdersByScope 按范围删除订单，返回删除行数
	DeleteOrdersByScope(ctx context.Context, scope task.OrderScope) (int64, error)
	// GetOrderCountByScope 按范围统计订单数
	GetOrderCountByScope(ctx context.Context, scope task.OrderScope) (int64, error)
	// NextTaskID 生成下一个任务 ID（Redis 自增）
	NextTaskID(ctx context.Context, gameID int64) (string, error)
	// GetGameBetSize 从 DB 获取游戏下注档位
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

	// 清理残余资源
	_, _ = uc.Cleanup(ctx)

	// 启动调度器
	go uc.scheduleLoop()

	go uc.memberPool.StartAutoLoad(ctx, c.Member, repo, logger, uc.WakeScheduler)

	go uc.taskPool.StartAutoCleanup(ctx, logger, taskRetentionPeriod, taskCleanupInterval)

	cleanup := func() { uc.cancel() }
	return uc, cleanup, nil
}

// GetGame 按 gameID 获取游戏
func (uc *UseCase) GetGame(gameID int64) (base.IGame, bool) {
	return uc.gamePool.Get(gameID)
}

// EnsureBetSize 确保游戏有 betsize，如果没有则从数据库动态获取
func (uc *UseCase) EnsureBetSize(ctx context.Context, gameID int64) error {
	return uc.gamePool.EnsureGameBetSize(ctx, gameID)
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

// Cleanup 清理 Redis 和 MySQL 订单数据
func (uc *UseCase) Cleanup(ctx context.Context) (redisErr, mysqlErr error) {
	cleanCtx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if redisErr = uc.repo.CleanRedisBySites(cleanCtx, uc.conf.Launch.Sites); redisErr != nil {
			log.Warnf("startup clean Redis: %v", redisErr)
		}
	}()

	go func() {
		defer wg.Done()
		if mysqlErr = uc.repo.CleanGameOrderTable(cleanCtx); mysqlErr != nil {
			log.Warnf("startup clean MySQL: %v", mysqlErr)
		}
	}()

	wg.Wait()
	return
}
