package biz

import (
	"context"
	"fmt"
	"stress/internal/biz/game"
	"stress/internal/biz/game/base"
	"stress/internal/biz/member"
	"stress/internal/biz/task"
	"stress/internal/conf"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// DataRepo 数据层接口：成员/订单/清理/任务ID计数
type DataRepo interface {
	BatchUpsertMembers(ctx context.Context, members []member.Info) error
	GetMemberCount(ctx context.Context) (int64, error)
	CleanRedisBySite(ctx context.Context, site, merchant string) error
	CleanRedisBySites(ctx context.Context, sites []string) error
	CleanGameOrderTable(ctx context.Context) error
	GetGameOrderCount(ctx context.Context) (int64, error)
	GetOrderAmounts(ctx context.Context) (totalBet, totalWin int64, err error)
	GetDetailedOrderAmounts(ctx context.Context) (totalBet, totalWin, betOrderCount, bonusOrderCount int64, err error)

	NextTaskID(ctx context.Context, gameID int64) (string, error)
}

// UseCase 编排层：通过 DataRepo + 领域池（Game/Task/Member）编排业务
type UseCase struct {
	repo       DataRepo
	log        *log.Helper
	c          *conf.Launch
	gamePool   *game.Pool
	taskPool   *task.Pool
	memberPool *member.Pool
}

// NewUseCase 创建 UseCase
func NewUseCase(repo DataRepo, logger log.Logger, c *conf.Launch) (*UseCase, func(), error) {
	uc := &UseCase{
		repo:       repo,
		log:        log.NewHelper(logger),
		c:          c,
		gamePool:   game.NewPool(),
		taskPool:   task.NewTaskPool(),
		memberPool: member.NewMemberPool(),
	}

	// 启动时自清理：Redis site:* + 订单表，避免上次压测残留
	uc.cleanOnStartup()

	cleanup := func() {}
	if c.AutoLoads {
		loaderCtx, cancel := context.WithCancel(context.Background())
		var loaderWg sync.WaitGroup
		loaderWg.Add(1)
		go func() {
			defer loaderWg.Done()
			runMemberLoader(loaderCtx, repo, uc.memberPool, uc.log, uc.c, uc.Schedule)
		}()
		cleanup = func() {
			cancel()
			loaderWg.Wait()
		}
	}
	return uc, cleanup, nil
}

// cleanOnStartup 启动时清理 Redis 与订单表
func (uc *UseCase) cleanOnStartup() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := uc.cleanupRedis(ctx); err != nil {
		uc.log.Warnf("startup clean Redis: %v", err)
	}

	if err := uc.repo.CleanGameOrderTable(ctx); err != nil {
		uc.log.Warnf("startup clean order table: %v", err)
	} else {
		uc.log.Info("startup clean order table done")
	}
}

// GetGame 按 gameID 获取游戏
func (uc *UseCase) GetGame(gameID int64) (base.IGame, bool) {
	return uc.gamePool.Get(gameID)
}

// ListGames 返回游戏列表副本（按 GameID 升序）
func (uc *UseCase) ListGames() []base.IGame {
	return uc.gamePool.List()
}

// runMemberLoader 按间隔生成成员、落库、加入空闲池，每批后调用 onLoaded（如 Schedule）
func runMemberLoader(ctx context.Context, repo DataRepo, pool *member.Pool, logHelper *log.Helper, c *conf.Launch, onLoaded func()) {
	ticker := time.NewTicker(time.Duration(c.IntervalSec) * time.Second)
	defer ticker.Stop()
	loaded := int32(0)
	for loaded < c.MaxLoadTotal {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n := c.BatchLoadSize
			if left := c.MaxLoadTotal - loaded; left < n {
				n = left
			}
			batch := make([]member.Info, n)
			for i := int32(0); i < n; i++ {
				loaded++
				batch[i] = member.Info{
					Name:    fmt.Sprintf("%s%d", member.DefaultMemberNamePrefix, loaded+1000),
					Balance: 10000,
				}
			}
			if err := repo.BatchUpsertMembers(ctx, batch); err != nil {
				if logHelper != nil {
					logHelper.Errorf("BatchUpsertMembers: %v", err)
				}
				loaded -= n
				continue
			}
			pool.AddIdle(batch)
			if logHelper != nil {
				_, _, total := pool.Stats()
				logHelper.Infof("Loaded %d members, total: %d", len(batch), total)
			}
			if onLoaded != nil {
				onLoaded()
			}
		}
	}
	if logHelper != nil {
		logHelper.Info("Member loading completed.")
	}
}
