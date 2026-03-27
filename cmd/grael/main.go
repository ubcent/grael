package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"grael/demo/harness"
	"grael/internal/api"
	grpcserver "grael/internal/grpcserver"
	"grael/internal/grpcserver/pb"
	rt "grael/internal/runtime"
	"grael/internal/workflowdef"

	"google.golang.org/grpc"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	switch args[0] {
	case "start":
		return startRun(args[1:])
	case "status":
		return getRun(args[1:])
	case "events":
		return listEvents(args[1:])
	case "snapshot":
		return snapshotInfo(args[1:])
	case "serve":
		return serve(args[1:])
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func startRun(args []string) error {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	dataDir := fs.String("data-dir", ".grael-data", "directory for Grael WAL data")
	workflowFile := fs.String("workflow", "", "path to workflow JSON file")
	example := fs.String("example", "", "name of built-in example workflow")
	demoWorker := fs.Bool("demo-worker", false, "start an in-process demo worker for the workflow activity types")
	demoProfileName := fs.String("demo-profile", string(harness.ProfileShowcase), "demo pacing profile for -demo-worker: showcase or fast")
	waitTimeout := fs.Duration("wait-timeout", 2*time.Second, "maximum time to wait for the initial run pass before exiting")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *workflowFile == "" && *example == "" {
		return errors.New("missing required -workflow or -example")
	}
	if *workflowFile != "" && *example != "" {
		return errors.New("use only one of -workflow or -example")
	}

	var (
		def rt.WorkflowDefinition
		err error
	)
	if *example != "" {
		def, err = workflowdef.BuiltIn(*example)
	} else {
		def, err = workflowdef.LoadJSON(*workflowFile)
	}
	if err != nil {
		return err
	}
	profile, err := harness.ParseProfile(*demoProfileName)
	if err != nil {
		return err
	}
	resolvedWaitTimeout := *waitTimeout
	if !flagExplicitlySet(fs, "wait-timeout") && *demoWorker {
		resolvedWaitTimeout = harness.DefaultWaitTimeout(profile)
	}

	svc := api.New(*dataDir)
	defer svc.Close()
	runID, err := svc.StartRun(def, nil)
	if err != nil {
		return err
	}
	if *demoWorker {
		if err := startDemoWorkerWithProfile(svc, runID, def, profile); err != nil {
			return err
		}
	}
	if *demoWorker {
		if err := waitForTerminalRun(svc, runID, resolvedWaitTimeout); err != nil {
			return err
		}
	} else {
		if _, err := svc.WaitForQuiescence(runID, resolvedWaitTimeout); err != nil {
			return err
		}
	}

	fmt.Println(runID)
	return nil
}

func getRun(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	dataDir := fs.String("data-dir", ".grael-data", "directory for Grael WAL data")
	runID := fs.String("run-id", "", "run identifier")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return errors.New("missing required -run-id")
	}

	svc := api.New(*dataDir)
	defer svc.Close()
	view, err := svc.GetRun(*runID)
	if err != nil {
		return err
	}
	return printJSON(view)
}

func listEvents(args []string) error {
	fs := flag.NewFlagSet("events", flag.ContinueOnError)
	dataDir := fs.String("data-dir", ".grael-data", "directory for Grael WAL data")
	runID := fs.String("run-id", "", "run identifier")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return errors.New("missing required -run-id")
	}

	svc := api.New(*dataDir)
	defer svc.Close()
	events, err := svc.ListEvents(*runID)
	if err != nil {
		return err
	}
	return printJSON(events)
}

func snapshotInfo(args []string) error {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	dataDir := fs.String("data-dir", ".grael-data", "directory for Grael WAL data")
	runID := fs.String("run-id", "", "run identifier")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *runID == "" {
		return errors.New("missing required -run-id")
	}

	svc := api.New(*dataDir)
	defer svc.Close()
	info, err := svc.SnapshotInfo(*runID)
	if err != nil {
		return err
	}
	return printJSON(info)
}

func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	dataDir := fs.String("data-dir", ".grael-data", "directory for Grael WAL data")
	grpcAddr := fs.String("grpc-addr", ":50051", "gRPC listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}

	svc := api.New(*dataDir)
	defer svc.Close()

	listener, err := net.Listen("tcp", *grpcAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	server := grpc.NewServer()
	pb.RegisterGraelServer(server, grpcserver.New(svc))
	return server.Serve(listener)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printUsage() {
	fmt.Println(`grael

Commands:
  start  (-workflow <file> | -example <name>) [-demo-worker] [-demo-profile <showcase|fast>] [-data-dir <dir>] [-wait-timeout <duration>]
  status -run-id <id> [-data-dir <dir>]
  events -run-id <id> [-data-dir <dir>]
  snapshot -run-id <id> [-data-dir <dir>]
  serve [-grpc-addr <addr>] [-data-dir <dir>]

Workflow definition example (JSON ingress format):
{
  "name": "linear-noop",
  "nodes": [
    {"id": "A", "activity_type": "noop"},
    {"id": "B", "activity_type": "noop", "depends_on": ["A"]}
  ]
}

Built-in examples:
  linear-noop`)
}

func startDemoWorker(svc *api.Service, runID string, def rt.WorkflowDefinition) error {
	return startDemoWorkerWithProfile(svc, runID, def, harness.ProfileShowcase)
}

func startDemoWorkerWithProfile(svc *api.Service, runID string, def rt.WorkflowDefinition, profile harness.Profile) error {
	return startDemoWorkerWithProfileAndContext(context.Background(), svc, runID, def, profile)
}

func startDemoWorkerWithContext(ctx context.Context, svc *api.Service, runID string, def rt.WorkflowDefinition) error {
	return startDemoWorkerWithProfileAndContext(ctx, svc, runID, def, harness.ProfileFast)
}

func startDemoWorkerWithProfileAndContext(ctx context.Context, svc *api.Service, runID string, def rt.WorkflowDefinition, profile harness.Profile) error {
	return startDemoWorkerWithID(ctx, svc, runID, def, "demo-worker", profile)
}

func startDemoWorkerWithID(ctx context.Context, svc *api.Service, runID string, def rt.WorkflowDefinition, workerID string, profile harness.Profile) error {
	return harness.StartWithPrefix(ctx, svc, runID, def, profile, workerID)
}

func flagExplicitlySet(fs *flag.FlagSet, name string) bool {
	explicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			explicit = true
		}
	})
	return explicit
}

func waitForTerminalRun(svc *api.Service, runID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		view, err := svc.GetRun(runID)
		if err != nil {
			return err
		}
		if isTerminalRunState(view.State) {
			return nil
		}
		if timeout > 0 && time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for run %s to reach terminal state", runID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func isTerminalRunState(state rt.RunState) bool {
	return state == rt.RunStateCompleted || state == rt.RunStateFailed || state == rt.RunStateCancelled || state == rt.RunStateCompensated
}

func stdoutString(fn func() error) (string, error) {
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	defer func() {
		os.Stdout = original
	}()

	callErr := fn()
	if err := w.Close(); err != nil && callErr == nil {
		callErr = err
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil && callErr == nil {
		callErr = err
	}
	if err := r.Close(); err != nil && callErr == nil {
		callErr = err
	}

	return buf.String(), callErr
}
