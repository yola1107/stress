package service

import (
	"context"
	"fmt"
	"strings"

	v1 "stress/api/stress/v1"
	"stress/internal/biz"
	"stress/internal/biz/task"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
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
func (s *StressService) ListGames(ctx context.Context, req *v1.ListGamesRequest) (*v1.ListGamesResponse, error) {
	all := s.uc.ListGames()
	filtered := make([]*v1.Game, 0, len(all))
	for _, g := range all {
		filtered = append(filtered, &v1.Game{
			GameId:      g.GameID(),
			GameName:    g.Name(),
			Description: fmt.Sprintf("Stress test: %s", g.Name()),
			IsActive:    true,
		})
	}
	s.log.Infof("ListGames returned %d/%d games", len(filtered), len(all))
	return &v1.ListGamesResponse{
		Games: filtered,
		Total: int32(len(filtered)),
	}, nil
}

// CreateTask 创建压测任务
func (s *StressService) CreateTask(ctx context.Context, req *v1.CreateTaskRequest) (*v1.CreateTaskResponse, error) {
	s.log.Infof("CreateTask request: %+v", req)
	if req == nil || req.Config == nil {
		return nil, kerrors.BadRequest("BAD_REQUEST", "request or config required")
	}
	config := req.Config
	if _, ok := s.uc.GetGame(config.GameId); !ok {
		log.Errorf("game not found: %d", config.GameId)
		return nil, kerrors.BadRequest("GAME_NOT_FOUND", fmt.Sprintf("game not found: %d", config.GameId))
	}
	t, err := s.uc.CreateTask(ctx, req.Description, req.Config)
	if err != nil {
		s.log.Errorf("create task failed: %v", err)
		return nil, kerrors.InternalServer("CREATE_FAILED", err.Error())
	}
	return &v1.CreateTaskResponse{Task: s.buildTaskProto(t)}, nil
}

// GetTask 获取任务详情
func (s *StressService) GetTask(ctx context.Context, req *v1.TaskRequest) (*v1.GetTaskResponse, error) {
	pb, err := s.getTaskProtoOrError(req)
	if err != nil {
		return nil, err
	}
	return &v1.GetTaskResponse{Task: pb}, nil
}

// GetTaskUserIds 获取任务分配到的用户ID列表
func (s *StressService) GetTaskUserIds(ctx context.Context, req *v1.TaskRequest) (*v1.GetTaskUserIdsResponse, error) {
	taskID, err := s.requireTaskRequest(req)
	if err != nil {
		return nil, err
	}
	t, err := s.getTaskOrError(taskID)
	if err != nil {
		return nil, err
	}
	ids := t.GetUserIDs()
	return &v1.GetTaskUserIdsResponse{UserIds: ids, Total: int32(len(ids))}, nil
}

// ListTasks 获取任务列表
func (s *StressService) ListTasks(ctx context.Context, req *v1.ListTasksRequest) (*v1.ListTasksResponse, error) {
	all := s.uc.ListTasks()
	tasks := make([]*v1.Task, 0, len(all))

	filter := ""
	status := v1.TaskStatus_TASK_UNSPECIFIED
	if req != nil {
		filter = strings.ToLower(strings.TrimSpace(req.Filter))
		status = req.Status
	}

	for _, t := range all {
		// 1. 状态过滤
		if status != v1.TaskStatus_TASK_UNSPECIFIED && t.GetStatus() != status {
			continue
		}
		// 2. 关键字过滤（按 description）
		if filter != "" && !strings.Contains(strings.ToLower(t.GetDescription()), filter) {
			continue
		}
		tasks = append(tasks, s.buildTaskProto(t))
	}

	return &v1.ListTasksResponse{
		Tasks: tasks,
		Total: int32(len(tasks)),
	}, nil
}

// GetTaskProgress 获取任务进度
func (s *StressService) GetTaskProgress(ctx context.Context, req *v1.TaskRequest) (*v1.GetTaskProgressResponse, error) {
	pb, err := s.getTaskProtoOrError(req)
	if err != nil {
		return nil, err
	}
	return &v1.GetTaskProgressResponse{Progress: pb.Progress}, nil
}

// GetTaskStatistics 获取任务统计
func (s *StressService) GetTaskStatistics(ctx context.Context, req *v1.TaskRequest) (*v1.GetTaskStatisticsResponse, error) {
	pb, err := s.getTaskProtoOrError(req)
	if err != nil {
		return nil, err
	}
	return &v1.GetTaskStatisticsResponse{Statistics: pb.Statistics}, nil
}

// GetSystemStatistics 获取系统统计
func (s *StressService) GetSystemStatistics(ctx context.Context, req *v1.GetSystemStatisticsRequest) (*v1.GetSystemStatisticsResponse, error) {
	_, allocated, total := s.uc.GetMemberStats()
	games := s.uc.ListGames()
	tasks := s.uc.ListTasks()
	nGames := int32(len(games))
	var running, pending int32
	for _, t := range tasks {
		switch t.GetStatus() {
		case v1.TaskStatus_TASK_RUNNING:
			running++
		case v1.TaskStatus_TASK_PENDING:
			pending++
		}
	}
	return &v1.GetSystemStatisticsResponse{
		Statistics: &v1.SystemStatistics{
			TotalGames:   nGames,
			ActiveGames:  nGames,
			TotalUsers:   int32(total),
			ActiveUsers:  int32(allocated),
			RunningTasks: running,
			PendingTasks: pending,
			CollectedAt:  timestamppb.Now(),
		},
	}, nil
}

// CancelTask 取消任务
func (s *StressService) CancelTask(ctx context.Context, req *v1.TaskRequest) (*emptypb.Empty, error) {
	return s.withTask(ctx, req, "CANCEL_FAILED", func(t *task.Task) error {
		return s.uc.CancelTask(t.GetID())
	})
}

// DeleteTask 删除任务
func (s *StressService) DeleteTask(ctx context.Context, req *v1.TaskRequest) (*emptypb.Empty, error) {
	return s.withTask(ctx, req, "DELETE_FAILED", func(t *task.Task) error {
		return s.uc.DeleteTask(t.GetID())
	})
}

// ============================================================================
// 辅助函数
// ============================================================================

func (s *StressService) requireTaskRequest(req *v1.TaskRequest) (string, error) {
	if req == nil {
		return "", kerrors.BadRequest("BAD_REQUEST", "request required")
	}
	taskID := strings.TrimSpace(req.TaskId)
	if taskID == "" {
		return "", kerrors.BadRequest("BAD_REQUEST", "task_id required")
	}
	return taskID, nil
}

func (s *StressService) getTaskOrError(taskID string) (*task.Task, error) {
	t, ok := s.uc.GetTask(taskID)
	if !ok {
		return nil, kerrors.NotFound("TASK_NOT_FOUND", "task not found")
	}
	return t, nil
}

func (s *StressService) getTaskProtoOrError(req *v1.TaskRequest) (*v1.Task, error) {
	taskID, err := s.requireTaskRequest(req)
	if err != nil {
		return nil, err
	}
	t, err := s.getTaskOrError(taskID)
	if err != nil {
		return nil, err
	}
	return s.buildTaskProto(t), nil
}

func (s *StressService) withTask(ctx context.Context, req *v1.TaskRequest, errCode string, fn func(*task.Task) error) (*emptypb.Empty, error) {
	taskID, err := s.requireTaskRequest(req)
	if err != nil {
		return nil, err
	}
	t, err := s.getTaskOrError(taskID)
	if err != nil {
		return nil, err
	}
	if err := fn(t); err != nil {
		if errCode != "" {
			return nil, kerrors.InternalServer(errCode, err.Error())
		}
		return nil, kerrors.InternalServer("TASK_OP_FAILED", err.Error())
	}
	return &emptypb.Empty{}, nil
}

func (s *StressService) fillTaskStatsFromSnapshot(snap *task.StatsSnapshot, taskProto *v1.Task) {
	var memberCount int32
	if snap.Config != nil {
		memberCount = snap.Config.MemberCount
	}
	taskProto.Progress = &v1.TaskProgress{
		TotalMembers:      memberCount,
		CompletedMembers:  int32(snap.CompletedMembers),
		ActiveMembers:     int32(snap.ActiveMembers),
		FailedMembers:     int32(snap.FailedMembers),
		TotalRequests:     snap.Target,
		CompletedRequests: snap.Process,
		FailedRequests:    snap.FailedRequests,
	}
	if snap.Target > 0 {
		taskProto.Progress.CompletionRate = float64(snap.Process) / float64(snap.Target)
	}
	sr := snap.SuccessRate()
	taskProto.Statistics = &v1.TaskStatistics{
		StartTime:         timestamppb.New(snap.CreatedAt),
		TotalDurationMs:   snap.TotalDuration.Milliseconds(),
		TotalBetOrders:    snap.BetOrders,
		TotalBetBonuses:   snap.BetBonuses,
		Qps:               snap.QPS(),
		AvgResponseTimeMs: snap.AvgLatencyMs(),
		SuccessRate:       sr,
		ErrorCounts:       snap.ErrorCounts,
	}
	if !snap.FinishedAt.IsZero() {
		taskProto.Statistics.EndTime = timestamppb.New(snap.FinishedAt)
	}
}

func (s *StressService) buildTaskProto(t *task.Task) *v1.Task {
	snap := t.GetStats().StatsSnapshot()
	taskProto := &v1.Task{
		TaskId:      snap.ID,
		Description: snap.Description,
		Status:      snap.Status,
		Config:      snap.Config,
		UserCount:   int32(snap.UserIDCount),
		CreatedAt:   timestamppb.New(snap.CreatedAt),
		UpdatedAt:   timestamppb.New(snap.CreatedAt),
	}
	s.fillTaskStatsFromSnapshot(&snap, taskProto)
	return taskProto
}
