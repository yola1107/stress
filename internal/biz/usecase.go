package biz

import (
	"context"
	"io"
	"strconv"
	"time"

	"stress/internal/biz/game"
	"stress/internal/biz/game/base"
	"stress/internal/biz/member"
	"stress/internal/biz/stats/statistics"
	"stress/internal/biz/task"
	"stress/internal/conf"
	"stress/internal/notify"

	"github.com/go-kratos/kratos/v2/log"
)

// 业务常量
const (
	cleanupTimeout = 10 * time.Minute
)

// DataRepo 数据层接口：成员/订单/清理/任务ID计数
type DataRepo interface {
	// 成员管理
	BatchUpsertMembers(ctx context.Context, members []member.Info) error

	// Redis清理
	CleanRedisBySites(ctx context.Context, sites []string) error

	// 订单表操作
	CleanGameOrderTable(ctx context.Context) error
	DeleteOrdersByScope(ctx context.Context, scope OrderScope) (int64, error)

	// 订单统计查询
	GetGameOrderCount(ctx context.Context) (int64, error)
	GetOrderCountByScope(ctx context.Context, scope OrderScope) (int64, error)
	GetDetailedOrderAmounts(ctx context.Context) (totalBet, totalWin, betOrderCount, bonusOrderCount int64, err error)
	QueryGameOrderPoints(ctx context.Context, scope OrderScope) ([]statistics.Point, error)

	// 任务ID生成
	NextTaskID(ctx context.Context, gameID int64) (string, error)

	// s3
	UploadFile(ctx context.Context, bucket, key, contentType string, body io.Reader) (string, error)
	UploadBytes(ctx context.Context, bucket, key, contentType string, data []byte) (string, error)
	//// s3上传
	//S3Uploader
}

//// S3Uploader S3上传器接口
//type S3Uploader interface {
//	UploadFile(ctx context.Context, bucket, key, contentType string, body io.Reader) (string, error)
//	UploadBytes(ctx context.Context, bucket, key, contentType string, data []byte) (string, error)
//}

// OrderScope 订单查询范围
type OrderScope struct {
	GameID     int64
	Merchant   string
	StartTime  time.Time
	EndTime    time.Time
	ExcludeAmt float64
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

	notify   notify.Notifier
	chartGen *statistics.Generator
	//s3       S3Uploader
}

// NewUseCase 创建 UseCase
func NewUseCase(repo DataRepo, logger log.Logger, c *conf.Stress, notify notify.Notifier) (*UseCase, func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	uc := &UseCase{
		ctx:        ctx,
		cancel:     cancel,
		repo:       repo,
		log:        log.NewHelper(logger),
		conf:       c,
		gamePool:   game.NewPool(),
		taskPool:   task.NewTaskPool(),
		memberPool: member.NewMemberPool(),
		notify:     notify,
		chartGen:   statistics.NewGenerator(""),
		//s3:         s3,
	}

	// 启动时自清理：Redis site:* + 订单表，避免上次压测残留
	uc.cleanOnStartup()

	// 启动成员自动加载
	if c.Member.AutoLoads {
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

// GetStressConfig 获取压测配置
func (uc *UseCase) GetStressConfig() *conf.Stress {
	return uc.conf
}

// GetMemberStats 玩家池统计
func (uc *UseCase) GetMemberStats() (idle, allocated, total int) {
	return uc.memberPool.Stats()
}

// cleanOnStartup 启动时清理 Redis 与订单表
func (uc *UseCase) cleanOnStartup() {
	ctx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
	defer cancel()

	if err := uc.repo.CleanRedisBySites(ctx, uc.conf.Launch.Sites); err != nil {
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
	const (
		memberLoadBatch  = 1000
		memberBalance    = 10000
		memberNameOffset = 1000
	)
	ticker := time.NewTicker(time.Duration(uc.conf.Member.IntervalSec) * time.Second)
	defer ticker.Stop()

	var loaded int32
	for loaded < uc.conf.Member.MaxLoadTotal {
		select {
		case <-uc.ctx.Done():
			return
		case <-ticker.C:
			n := min(memberLoadBatch, uc.conf.Member.MaxLoadTotal-loaded)
			if n <= 0 {
				continue
			}
			batch := make([]member.Info, n)
			for i := int32(0); i < n; i++ {
				loaded++
				batch[i] = member.Info{
					Name:    uc.conf.Member.MemberPrefix + strconv.FormatInt(int64(loaded+memberNameOffset), 10),
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
