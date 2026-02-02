package service

import (
	"context"
	"strings"

	v1 "stress/api/stress/v1"
	"stress/internal/biz"
	"stress/internal/biz/task"

	"github.com/go-kratos/kratos/v2/errors"
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

// ListGames 获取游戏列表
func (s *StressService) ListGames(ctx context.Context, in *v1.ListGamesRequest) (*v1.ListGamesResponse, error) {
	all := s.uc.ListGames()
	games := make([]*v1.Game, len(all))
	for i, g := range all {
		games[i] = &v1.Game{
			GameId:      g.GameID(),
			GameName:    g.Name(),
			Description: g.Name(),
		}
	}
	return &v1.ListGamesResponse{Games: games, Total: int32(len(games))}, nil
}

// ListTasks 获取任务列表
func (s *StressService) ListTasks(ctx context.Context, in *v1.ListTasksRequest) (*v1.ListTasksResponse, error) {
	all := s.uc.ListTasks()
	tasks := make([]*v1.Task, 0, len(all))

	status := v1.TaskStatus_TASK_UNSPECIFIED
	if in != nil {
		status = in.Status
	}

	for _, t := range all {
		if status != v1.TaskStatus_TASK_UNSPECIFIED && t.GetStatus() != status {
			continue
		}
		tasks = append(tasks, s.buildTask(t))
	}
	return &v1.ListTasksResponse{Tasks: tasks, Total: int32(len(tasks))}, nil
}

// CreateTask 创建压测任务
func (s *StressService) CreateTask(ctx context.Context, in *v1.CreateTaskRequest) (*v1.CreateTaskResponse, error) {
	if in == nil || in.Config == nil {
		return nil, errors.BadRequest("INVALID_ARGUMENT", "request config is required")
	}

	g, ok := s.uc.GetGame(in.Config.GameId)
	if !ok {
		s.log.Warnf("CreateTask game not found: %d", in.Config.GameId)
		return nil, errors.NotFound("GAME_NOT_FOUND", "game not found")
	}

	t, err := s.uc.CreateTask(ctx, g, in.Config)
	if err != nil {
		s.log.Errorf("CreateTask failed: %v", err)
		return nil, errors.InternalServer("CREATE_TASK_FAILED", err.Error())
	}

	return &v1.CreateTaskResponse{
		Code:    0,
		Message: "success",
		Task:    s.buildTask(t),
	}, nil
}

// TaskInfo 获取任务详情
func (s *StressService) TaskInfo(ctx context.Context, in *v1.TaskInfoRequest) (*v1.TaskInfoResponse, error) {
	t, err := s.getTask(in.TaskId)
	if err != nil {
		return nil, err
	}

	return &v1.TaskInfoResponse{
		Code:    0,
		Message: "success",
		Task:    s.buildTask(t),
	}, nil
}

// CancelTask 取消任务
func (s *StressService) CancelTask(ctx context.Context, in *v1.CancelTaskRequest) (*v1.CancelTaskResponse, error) {
	t, err := s.getTask(in.TaskId)
	if err != nil {
		return nil, err
	}

	if err = s.uc.CancelTask(t.GetID()); err != nil {
		return nil, errors.InternalServer("CANCEL_TASK_FAILED", err.Error())
	}

	return &v1.CancelTaskResponse{
		Code:    0,
		Message: "success",
	}, nil
}

// DeleteTask 删除任务
func (s *StressService) DeleteTask(ctx context.Context, in *v1.DeleteTaskRequest) (*emptypb.Empty, error) {
	t, err := s.getTask(in.TaskId)
	if err != nil {
		return nil, errors.Wrapf(err, "delete task failed")
	}

	if err := s.uc.DeleteTask(t.GetID()); err != nil {
		return nil, errors.Wrapf(err, "delete task failed")
	}

	return &emptypb.Empty{}, nil
}

// GetRecord 获取任务结果
func (s *StressService) GetRecord(ctx context.Context, in *v1.RecordRequest) (*v1.RecordResponse, error) {
	t, err := s.getTask(in.TaskId)
	if err != nil {
		return nil, err
	}

	return &v1.RecordResponse{
		Code:    0,
		Message: "success",
		Url:     "/api/stress/tasks/" + t.GetID() + "/record",
	}, nil
}

func (s *StressService) getTask(taskID string) (*task.Task, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, errors.BadRequest("INVALID_ARGUMENT", "task id is required")
	}

	t, ok := s.uc.GetTask(taskID)
	if !ok {
		return nil, errors.NotFound("TASK_NOT_FOUND", "task not found")
	}

	return t, nil
}

func (s *StressService) buildTask(t *task.Task) *v1.Task {
	taskID := t.GetID()
	return &v1.Task{
		TaskId:    taskID,
		Status:    t.GetStatus(),
		Config:    t.GetConfig(),
		RecordUrl: "/api/stress/tasks/" + taskID + "/record",
		CreatedAt: timestamppb.New(t.GetCreatedAt()),
		UpdatedAt: timestamppb.Now(),
	}
}
