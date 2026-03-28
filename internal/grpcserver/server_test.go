package grpcserver_test

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"grael/internal/api"
	"grael/internal/grpcserver"
	"grael/internal/grpcserver/pb"
	rt "grael/internal/runtime"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestGraelServerSupportsWorkerLifecycleAndEventStreaming(t *testing.T) {
	t.Parallel()

	client, cleanup := newTestClient(t)
	defer cleanup()

	ctx := context.Background()
	startResp, err := client.StartRun(ctx, &pb.StartRunRequest{
		Workflow: &pb.WorkflowDefinition{
			Name: "grpc-demo",
			Nodes: []*pb.NodeDefinition{
				{
					Id:           "collect",
					ActivityType: string(rt.ActivityTypeNoop),
					Input:        mustStruct(t, map[string]any{"question": "What is the customer impact?"}),
				},
			},
		},
		Input: mustStruct(t, map[string]any{"brief": "morning incident"}),
	})
	if err != nil {
		t.Fatalf("start run: %v", err)
	}
	runID := startResp.GetRunId()
	if runID == "" {
		t.Fatal("expected run id")
	}

	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()
	stream, err := client.StreamEvents(streamCtx, &pb.StreamEventsRequest{RunId: runID, FromSeq: 0})
	if err != nil {
		t.Fatalf("stream events: %v", err)
	}

	firstEvent, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv first event: %v", err)
	}
	if firstEvent.GetType() != string(rt.EventWorkflowStarted) {
		t.Fatalf("expected first event WorkflowStarted, got %s", firstEvent.GetType())
	}
	var startedPayload map[string]any
	if err := json.Unmarshal(firstEvent.GetPayload(), &startedPayload); err != nil {
		t.Fatalf("decode first payload: %v", err)
	}
	if _, ok := startedPayload["workflow"]; !ok {
		t.Fatalf("expected workflow in start payload, got %v", startedPayload)
	}

	if _, err := client.RegisterWorker(ctx, &pb.RegisterWorkerRequest{
		WorkerId:      "grpc-worker-1",
		ActivityTypes: []string{string(rt.ActivityTypeNoop)},
	}); err != nil {
		t.Fatalf("register worker: %v", err)
	}

	pollResp, err := client.PollTask(ctx, &pb.PollTaskRequest{
		WorkerId:  "grpc-worker-1",
		TimeoutMs: 100,
	})
	if err != nil {
		t.Fatalf("poll task: %v", err)
	}
	if pollResp.GetTask() == nil {
		t.Fatal("expected task")
	}
	if pollResp.GetTask().GetRunId() != runID {
		t.Fatalf("expected task run %s, got %s", runID, pollResp.GetTask().GetRunId())
	}
	if got := pollResp.GetTask().GetWorkflowInput().AsMap()["brief"]; got != "morning incident" {
		t.Fatalf("expected workflow input over grpc, got %v", pollResp.GetTask().GetWorkflowInput().AsMap())
	}
	if got := pollResp.GetTask().GetNodeInput().AsMap()["question"]; got != "What is the customer impact?" {
		t.Fatalf("expected node input over grpc, got %v", pollResp.GetTask().GetNodeInput().AsMap())
	}

	if _, err := client.CompleteTask(ctx, &pb.CompleteTaskRequest{
		WorkerId: "grpc-worker-1",
		RunId:    runID,
		NodeId:   pollResp.GetTask().GetNodeId(),
		Attempt:  pollResp.GetTask().GetAttempt(),
		Output:   mustStruct(t, map[string]any{"status": "done"}),
	}); err != nil {
		t.Fatalf("complete task: %v", err)
	}

	view, err := client.GetRun(ctx, &pb.GetRunRequest{RunId: runID})
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if view.GetState() != string(rt.RunStateCompleted) {
		t.Fatalf("expected completed run, got %s", view.GetState())
	}

	seenCompleted := false
	deadline := time.After(2 * time.Second)
	for !seenCompleted {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for workflow completed event")
		default:
		}

		event, err := stream.Recv()
		if err != nil {
			t.Fatalf("recv streamed event: %v", err)
		}
		if event.GetType() == string(rt.EventWorkflowCompleted) {
			seenCompleted = true
		}
	}

	eventsResp, err := client.ListEvents(ctx, &pb.ListEventsRequest{RunId: runID})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(eventsResp.GetEvents()) < 5 {
		t.Fatalf("expected worker lifecycle events, got %d", len(eventsResp.GetEvents()))
	}
}

func newTestClient(t *testing.T) (pb.GraelClient, func()) {
	t.Helper()

	listener := bufconn.Listen(1024 * 1024)
	service := api.New(t.TempDir())
	server := grpc.NewServer()
	pb.RegisterGraelServer(server, grpcserver.New(service))

	go func() {
		if err := server.Serve(listener); err != nil {
			t.Logf("grpc server stopped: %v", err)
		}
	}()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial grpc bufconn: %v", err)
	}

	cleanup := func() {
		_ = conn.Close()
		server.Stop()
		_ = listener.Close()
		service.Close()
	}
	return pb.NewGraelClient(conn), cleanup
}

func mustStruct(t *testing.T, value map[string]any) *structpb.Struct {
	t.Helper()
	out, err := structpb.NewStruct(value)
	if err != nil {
		t.Fatalf("structpb.NewStruct: %v", err)
	}
	return out
}
