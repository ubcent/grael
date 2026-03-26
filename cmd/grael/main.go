package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"grael/internal/api"
	rt "grael/internal/runtime"
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
		def, err = builtInExample(*example)
	} else {
		def, err = loadWorkflow(*workflowFile)
	}
	if err != nil {
		return err
	}

	svc := api.New(*dataDir)
	runID, err := svc.StartRun(def, nil)
	if err != nil {
		return err
	}
	if _, err := svc.WaitForQuiescence(runID, *waitTimeout); err != nil {
		return err
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
	info, err := svc.SnapshotInfo(*runID)
	if err != nil {
		return err
	}
	return printJSON(info)
}

func loadWorkflow(path string) (rt.WorkflowDefinition, error) {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return rt.WorkflowDefinition{}, fmt.Errorf("read workflow file: %w", err)
	}
	var def rt.WorkflowDefinition
	if err := json.Unmarshal(content, &def); err != nil {
		return rt.WorkflowDefinition{}, fmt.Errorf("decode workflow file: %w", err)
	}
	if def.Name == "" {
		return rt.WorkflowDefinition{}, errors.New("workflow name is required")
	}
	if len(def.Nodes) == 0 {
		return rt.WorkflowDefinition{}, errors.New("workflow must contain at least one node")
	}
	for _, node := range def.Nodes {
		if node.ID == "" {
			return rt.WorkflowDefinition{}, errors.New("every node must have an id")
		}
		if node.ActivityType == "" {
			return rt.WorkflowDefinition{}, fmt.Errorf("node %q must define activity_type", node.ID)
		}
	}
	return def, nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printUsage() {
	fmt.Println(`grael

Commands:
  start  (-workflow <file> | -example <name>) [-data-dir <dir>] [-wait-timeout <duration>]
  status -run-id <id> [-data-dir <dir>]
  events -run-id <id> [-data-dir <dir>]
  snapshot -run-id <id> [-data-dir <dir>]

Workflow JSON example:
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

func builtInExample(name string) (rt.WorkflowDefinition, error) {
	switch name {
	case "linear-noop":
		return rt.WorkflowDefinition{
			Name: "linear-noop",
			Nodes: []rt.NodeDefinition{
				{ID: "A", ActivityType: rt.ActivityTypeNoop},
				{ID: "B", ActivityType: rt.ActivityTypeNoop, DependsOn: []string{"A"}},
				{ID: "C", ActivityType: rt.ActivityTypeNoop, DependsOn: []string{"B"}},
			},
		}, nil
	default:
		return rt.WorkflowDefinition{}, fmt.Errorf("unknown built-in example %q", name)
	}
}
