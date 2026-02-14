package service

import (
	"context"
	"fmt"
	"stress/pkg/xgo"
	"strings"
	"sync"

	v1 "stress/api/stress/v1"
	"stress/internal/biz"
	"stress/internal/biz/game/base"
	"stress/internal/biz/task"

	"github.com/go-kratos/kratos/v2/log"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	Failed = 1
)

// StressService is a stress test service.
type StressService struct {
	v1.UnimplementedStressServiceServer
	uc  *biz.UseCase
	log *log.Helper
}

// NewStressService new a stress service.
func NewStressService(uc *biz.UseCase, logger log.Logger) *StressService {
	return &StressService{
		uc:  uc,
		log: log.NewHelper(logger),
	}
}

// PingReq implements stress.PingReq.
func (s *StressService) PingReq(ctx context.Context, in *v1.PingRequest) (*v1.PingReply, error) {
	return &v1.PingReply{Message: "Hello " + in.Name}, nil
}

// ListGames 获取游戏列表
func (s *StressService) ListGames(ctx context.Context, in *v1.ListGamesRequest) (*v1.ListGamesResponse, error) {
	all := s.uc.ListGames()
	games := make([]*v1.Game, len(all))
	for i, g := range all {
		games[i] = &v1.Game{
			GameId:      g.GameID(),
			GameName:    g.Name(),
			Description: g.Name(),
			BetSize:     g.BetSize(),
		}
	}
	return &v1.ListGamesResponse{Games: games, Total: int32(len(games))}, nil
}

// ListTasks 获取任务列表
func (s *StressService) ListTasks(ctx context.Context, in *v1.ListTasksRequest) (*v1.ListTasksResponse, error) {
	all := s.uc.ListTasks()
	tasks := make([]*v1.Task, 0, len(all))

	for _, t := range all {
		if t == nil {
			continue
		}
		if protoTask := t.ToProto(); protoTask != nil {
			tasks = append(tasks, protoTask)
		}
	}
	return &v1.ListTasksResponse{Tasks: tasks, Total: int32(len(tasks))}, nil
}

// CreateTask 创建压测任务
func (s *StressService) CreateTask(ctx context.Context, in *v1.CreateTaskRequest) (*v1.CreateTaskResponse, error) {
	if in == nil || in.Config == nil || in.Config.BetOrder == nil {
		return &v1.CreateTaskResponse{Code: Failed, Message: "req.Config is nil"}, nil
	}

	g, ok := s.uc.GetGame(in.Config.GameId)
	if !ok {
		s.log.Warnf("CreateTask Faild. game not found: %d", in.Config.GameId)
		return &v1.CreateTaskResponse{Code: Failed, Message: fmt.Sprintf("game not found: %d", in.Config.GameId)}, nil
	}

	if !g.ValidBetMoney(in.Config.BetOrder.BaseMoney) {
		msg := fmt.Sprintf("CreateTask Faild. game_id=%d, invalid bet money: %.2f, betsize: %v",
			in.Config.GameId, in.Config.BetOrder.BaseMoney, g.BetSize())
		s.log.Warnf("%s", msg)
		return &v1.CreateTaskResponse{Code: Failed, Message: msg}, nil
	}

	t, err := s.uc.CreateTask(ctx, g, in.Config)
	if err != nil {
		s.log.Warnf("CreateTask failed: %v", err)
		return &v1.CreateTaskResponse{Code: Failed, Message: err.Error()}, nil
	}
	return &v1.CreateTaskResponse{Task: t.ToProto()}, nil
}

// TaskInfo 获取任务详情
func (s *StressService) TaskInfo(ctx context.Context, in *v1.TaskInfoRequest) (*v1.TaskInfoResponse, error) {
	t, err := s.getTask(in.TaskId)
	if err != nil {
		return &v1.TaskInfoResponse{Code: Failed, Message: err.Error()}, nil
	}
	return &v1.TaskInfoResponse{Task: t.ToProto()}, nil
}

// CancelTask 取消任务
func (s *StressService) CancelTask(ctx context.Context, in *v1.CancelTaskRequest) (*v1.CancelTaskResponse, error) {
	if err := s.uc.CancelTask(in.TaskId); err != nil {
		return &v1.CancelTaskResponse{Code: Failed, Message: err.Error()}, nil
	}
	return &v1.CancelTaskResponse{}, nil
}

// DeleteTask 删除任务
func (s *StressService) DeleteTask(ctx context.Context, in *v1.DeleteTaskRequest) (*emptypb.Empty, error) {
	if err := s.uc.DeleteTask(in.TaskId); err != nil {
		s.log.Warnf("DeleteTask failed: %v", err)
		return &emptypb.Empty{}, nil
	}
	return &emptypb.Empty{}, nil
}

// GetRecord 获取任务结果
func (s *StressService) GetRecord(ctx context.Context, in *v1.RecordRequest) (*v1.RecordResponse, error) {
	t, err := s.getTask(in.TaskId)
	if err != nil {
		return &v1.RecordResponse{Code: Failed, Message: err.Error()}, nil
	}
	return &v1.RecordResponse{Url: t.GetRecordUrl()}, nil
}

func (s *StressService) getTask(taskID string) (*task.Task, error) {
	if taskID = strings.TrimSpace(taskID); taskID == "" {
		return nil, fmt.Errorf("task id is empty")
	}
	if t, ok := s.uc.GetTask(taskID); ok && t != nil {
		return t, nil
	}
	return nil, fmt.Errorf("task not found")
}

// ================================================================

// Bench 批量压测启动
func (s *StressService) Bench(ctx context.Context, in *v1.BenchRequest) (*v1.BenchResponse, error) {
	games := s.uc.ListGames()
	if len(games) == 0 {
		return &v1.BenchResponse{Code: Failed, Message: "no games available"}, nil
	}

	// 过滤目标游戏
	targets := games
	if len(in.GameIds) > 0 {
		gameMap := make(map[int64]base.IGame, len(games))
		for _, g := range games {
			gameMap[g.GameID()] = g
		}

		targets = make([]base.IGame, 0, len(in.GameIds))
		for _, id := range in.GameIds {
			if g, ok := gameMap[id]; ok {
				targets = append(targets, g)
			}
		}
	}

	if len(targets) == 0 {
		return &v1.BenchResponse{Code: Failed, Message: "no matching games"}, nil
	}

	var (
		mu      sync.Mutex
		taskIDs = make([]string, 0, len(targets))
		fails   = make([]string, 0)
	)

	eg, egCtx := errgroup.WithContext(ctx)

	for _, g := range targets {
		g := g

		eg.Go(func() error {
			cfg := &v1.TaskConfig{
				GameId:         g.GameID(),
				Description:    "bench",
				MemberCount:    in.MemberCount,
				TimesPerMember: in.TimesPerMember,
				BetOrder: &v1.BetOrderConfig{
					BaseMoney: pickBaseMoney(g.BetSize()),
					Multiple:  1,
				},
				BetBonus: &v1.BetBonusConfig{
					Enable:        true,
					BonusSequence: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
				},
			}

			t, err := s.uc.CreateTask(egCtx, g, cfg)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fails = append(fails, fmt.Sprintf("%d:%s", g.GameID(), err.Error()))
				return nil
			}
			taskIDs = append(taskIDs, t.GetID())
			return nil
		})
	}

	_ = eg.Wait()

	return &v1.BenchResponse{TaskIds: xgo.ToJSON(taskIDs), Fails: fails}, nil
}

func pickBaseMoney(sizes []float64) float64 {
	if len(sizes) > 1 {
		return sizes[1]
	}
	if len(sizes) > 0 {
		return sizes[0]
	}
	return 0.1
}
