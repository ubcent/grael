package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"grael/internal/api"
	rt "grael/internal/runtime"
	"grael/internal/workflowdef"
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
		def, err = workflowdef.BuiltIn(*example)
	} else {
		def, err = workflowdef.LoadJSON(*workflowFile)
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
