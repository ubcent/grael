package grpcserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"grael/internal/api"
	"grael/internal/engine"
	"grael/internal/grpcserver/pb"
	rt "grael/internal/runtime"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	pb.UnimplementedGraelServer
	service *api.Service
}

func New(service *api.Service) *Server {
	return &Server{service: service}
}

func (s *Server) StartRun(ctx context.Context, req *pb.StartRunRequest) (*pb.StartRunResponse, error) {
	def, err := workflowDefinitionFromProto(req.GetWorkflow())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	input, err := structToMap(req.GetInput())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	runID, err := s.service.StartRun(def, input)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.StartRunResponse{RunId: runID}, nil
}

func (s *Server) CancelRun(ctx context.Context, req *pb.CancelRunRequest) (*emptypb.Empty, error) {
	if err := s.service.CancelRun(req.GetRunId()); err != nil {
		return nil, mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ApproveCheckpoint(ctx context.Context, req *pb.ApproveCheckpointRequest) (*emptypb.Empty, error) {
	if err := s.service.ApproveCheckpoint(req.GetRunId(), req.GetNodeId()); err != nil {
		return nil, mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) GetRun(ctx context.Context, req *pb.GetRunRequest) (*pb.RunView, error) {
	view, err := s.service.GetRun(req.GetRunId())
	if err != nil {
		return nil, mapError(err)
	}
	return runViewToProto(view), nil
}

func (s *Server) StreamEvents(req *pb.StreamEventsRequest, stream pb.Grael_StreamEventsServer) error {
	events, err := s.service.SubscribeEvents(stream.Context(), req.GetRunId(), req.GetFromSeq())
	if err != nil {
		return mapError(err)
	}
	for event := range events {
		mapped, err := eventToProto(event)
		if err != nil {
			return status.Errorf(codes.Internal, "marshal event payload: %v", err)
		}
		if err := stream.Send(mapped); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) RegisterWorker(ctx context.Context, req *pb.RegisterWorkerRequest) (*emptypb.Empty, error) {
	activities := make([]rt.ActivityType, 0, len(req.GetActivityTypes()))
	for _, activity := range req.GetActivityTypes() {
		activities = append(activities, rt.ActivityType(activity))
	}
	if err := s.service.RegisterWorker(req.GetWorkerId(), activities); err != nil {
		return nil, mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) PollTask(ctx context.Context, req *pb.PollTaskRequest) (*pb.PollTaskResponse, error) {
	timeout := time.Duration(req.GetTimeoutMs()) * time.Millisecond
	task, ok, err := s.service.PollTask(req.GetWorkerId(), timeout)
	if err != nil {
		return nil, mapError(err)
	}
	if !ok {
		return &pb.PollTaskResponse{}, nil
	}
	mapped, err := workerTaskToProto(task)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.PollTaskResponse{Task: mapped}, nil
}

func (s *Server) CompleteTask(ctx context.Context, req *pb.CompleteTaskRequest) (*emptypb.Empty, error) {
	mapped, err := completeTaskFromProto(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err := s.service.CompleteTask(mapped); err != nil {
		return nil, mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) FailTask(ctx context.Context, req *pb.FailTaskRequest) (*emptypb.Empty, error) {
	if err := s.service.FailTask(rt.FailTaskRequest{
		WorkerID:     req.GetWorkerId(),
		RunID:        req.GetRunId(),
		NodeID:       req.GetNodeId(),
		Attempt:      req.GetAttempt(),
		Compensation: req.GetCompensation(),
		Message:      req.GetMessage(),
		Cancelled:    req.GetCancelled(),
		Retryable:    req.GetRetryable(),
	}); err != nil {
		return nil, mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*emptypb.Empty, error) {
	if err := s.service.Heartbeat(req.GetWorkerId()); err != nil {
		return nil, mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListEvents(ctx context.Context, req *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	events, err := s.service.ListEvents(req.GetRunId())
	if err != nil {
		return nil, mapError(err)
	}
	mapped := make([]*pb.Event, 0, len(events))
	for _, event := range events {
		protoEvent, err := eventToProto(event)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal event payload: %v", err)
		}
		mapped = append(mapped, protoEvent)
	}
	return &pb.ListEventsResponse{Events: mapped}, nil
}

func workflowDefinitionFromProto(def *pb.WorkflowDefinition) (rt.WorkflowDefinition, error) {
	if def == nil {
		return rt.WorkflowDefinition{}, errors.New("workflow is required")
	}

	nodes := make([]rt.NodeDefinition, 0, len(def.GetNodes()))
	for _, node := range def.GetNodes() {
		mapped, err := nodeDefinitionFromProto(node)
		if err != nil {
			return rt.WorkflowDefinition{}, fmt.Errorf("node %q: %w", node.GetId(), err)
		}
		nodes = append(nodes, mapped)
	}
	return rt.WorkflowDefinition{
		Name:  def.GetName(),
		Nodes: nodes,
	}, nil
}

func nodeDefinitionFromProto(node *pb.NodeDefinition) (rt.NodeDefinition, error) {
	if node == nil {
		return rt.NodeDefinition{}, errors.New("node definition is required")
	}
	input, err := structToMap(node.GetInput())
	if err != nil {
		return rt.NodeDefinition{}, fmt.Errorf("input: %w", err)
	}
	checkpointTimeout, err := durationFromProto(node.GetCheckpointTimeout())
	if err != nil {
		return rt.NodeDefinition{}, fmt.Errorf("checkpoint_timeout: %w", err)
	}
	executionDeadline, err := durationFromProto(node.GetExecutionDeadline())
	if err != nil {
		return rt.NodeDefinition{}, fmt.Errorf("execution_deadline: %w", err)
	}
	absoluteDeadline, err := durationFromProto(node.GetAbsoluteDeadline())
	if err != nil {
		return rt.NodeDefinition{}, fmt.Errorf("absolute_deadline: %w", err)
	}

	return rt.NodeDefinition{
		ID:                   node.GetId(),
		ActivityType:         rt.ActivityType(node.GetActivityType()),
		Input:                input,
		DependsOn:            append([]string(nil), node.GetDependsOn()...),
		RetryPolicy:          retryPolicyFromProto(node.GetRetryPolicy()),
		CompensationActivity: rt.ActivityType(node.GetCompensationActivity()),
		CheckpointTimeout:    checkpointTimeout,
		ExecutionDeadline:    executionDeadline,
		AbsoluteDeadline:     absoluteDeadline,
	}, nil
}

func retryPolicyFromProto(policy *pb.RetryPolicy) *rt.RetryPolicy {
	if policy == nil {
		return nil
	}
	backoff, _ := durationFromProto(policy.GetBackoff())
	return &rt.RetryPolicy{
		MaxAttempts: int(policy.GetMaxAttempts()),
		Backoff:     backoff,
	}
}

func durationFromProto(value *durationpb.Duration) (time.Duration, error) {
	if value == nil {
		return 0, nil
	}
	return value.AsDuration(), value.CheckValid()
}

func structToMap(value *structpb.Struct) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}
	return value.AsMap(), nil
}

func runViewToProto(view rt.RunView) *pb.RunView {
	nodes := make(map[string]*pb.NodeView, len(view.Nodes))
	keys := make([]string, 0, len(view.Nodes))
	for nodeID := range view.Nodes {
		keys = append(keys, nodeID)
	}
	sort.Strings(keys)
	for _, nodeID := range keys {
		node := view.Nodes[nodeID]
		nodes[nodeID] = &pb.NodeView{
			Id:           node.ID,
			ActivityType: string(node.ActivityType),
			State:        string(node.State),
			DependsOn:    append([]string(nil), node.DependsOn...),
			Attempt:      node.Attempt,
			WorkerId:     node.WorkerID,
			LastError:    node.LastError,
		}
	}

	resp := &pb.RunView{
		RunId:          view.RunID,
		Workflow:       view.Workflow,
		DefinitionHash: view.DefinitionHash,
		State:          string(view.State),
		LastSeq:        view.LastSeq,
		Nodes:          nodes,
		CreatedAt:      timestamppb.New(view.CreatedAt),
	}
	if view.FinishedAt != nil {
		resp.FinishedAt = timestamppb.New(*view.FinishedAt)
	}
	return resp
}

func workerTaskToProto(task rt.WorkerTask) (*pb.WorkerTask, error) {
	input, err := mapToStruct(task.WorkflowInput)
	if err != nil {
		return nil, fmt.Errorf("workflow input: %w", err)
	}
	nodeInput, err := mapToStruct(task.NodeInput)
	if err != nil {
		return nil, fmt.Errorf("node input: %w", err)
	}
	return &pb.WorkerTask{
		RunId:         task.RunID,
		NodeId:        task.NodeID,
		ActivityType:  string(task.ActivityType),
		Attempt:       task.Attempt,
		Compensation:  task.Compensation,
		Workflow:      task.Workflow,
		WorkflowInput: input,
		NodeInput:     nodeInput,
	}, nil
}

func completeTaskFromProto(req *pb.CompleteTaskRequest) (rt.CompleteTaskRequest, error) {
	output, err := structToMap(req.GetOutput())
	if err != nil {
		return rt.CompleteTaskRequest{}, fmt.Errorf("output: %w", err)
	}
	spawned := make([]rt.NodeDefinition, 0, len(req.GetSpawnedNodes()))
	for _, node := range req.GetSpawnedNodes() {
		mapped, err := nodeDefinitionFromProto(node)
		if err != nil {
			return rt.CompleteTaskRequest{}, fmt.Errorf("spawned_nodes: %w", err)
		}
		spawned = append(spawned, mapped)
	}

	out := rt.CompleteTaskRequest{
		WorkerID:     req.GetWorkerId(),
		RunID:        req.GetRunId(),
		NodeID:       req.GetNodeId(),
		Attempt:      req.GetAttempt(),
		Compensation: req.GetCompensation(),
		Output:       output,
		SpawnedNodes: spawned,
	}
	if checkpoint := req.GetCheckpoint(); checkpoint != nil {
		out.Checkpoint = &rt.CheckpointRequest{Reason: checkpoint.GetReason()}
	}
	return out, nil
}

func mapToStruct(value map[string]any) (*structpb.Struct, error) {
	if value == nil {
		return nil, nil
	}
	return structpb.NewStruct(value)
}

func eventToProto(event rt.Event) (*pb.Event, error) {
	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return nil, err
	}
	return &pb.Event{
		Seq:       event.Seq,
		RunId:     event.RunID,
		Type:      string(event.Type),
		Timestamp: timestamppb.New(event.Timestamp),
		Payload:   payload,
	}, nil
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, engine.ErrRunNotFound), errors.Is(err, engine.ErrTaskNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, engine.ErrWorkerUnavailable), errors.Is(err, engine.ErrAttemptMismatch),
		errors.Is(err, engine.ErrLeaseExpired), errors.Is(err, engine.ErrCheckpointNotWaiting),
		errors.Is(err, engine.ErrRunCancelled):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, engine.ErrCapacityExceeded):
		return status.Error(codes.ResourceExhausted, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
