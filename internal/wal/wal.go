package wal

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"time"

	rt "grael/internal/runtime"
)

var ErrCorruptTail = errors.New("wal: corrupt tail")

type Store struct {
	baseDir string
	mu      sync.Mutex
}

func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

type diskRecord struct {
	Seq       uint64          `json:"seq"`
	RunID     string          `json:"run_id"`
	Type      rt.EventType    `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
	CRC32     uint32          `json:"crc32"`
}

// Append persists exactly one event at the next durable sequence number.
func (s *Store) Append(event rt.Event) (rt.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return rt.Event{}, fmt.Errorf("create wal dir: %w", err)
	}

	// Sprint 1 keeps sequence assignment simple and deterministic by deriving the
	// next sequence from the already persisted valid prefix of the run WAL.
	events, _, err := s.scanLocked(event.RunID)
	if err != nil && !errors.Is(err, ErrCorruptTail) {
		return rt.Event{}, err
	}
	event.Seq = uint64(len(events) + 1)
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}

	payload, err := json.Marshal(event.Payload)
	if err != nil {
		return rt.Event{}, fmt.Errorf("marshal payload: %w", err)
	}

	record := diskRecord{
		Seq:       event.Seq,
		RunID:     event.RunID,
		Type:      event.Type,
		Timestamp: event.Timestamp,
		Payload:   payload,
	}
	record.CRC32 = checksum(record)

	line, err := json.Marshal(record)
	if err != nil {
		return rt.Event{}, fmt.Errorf("marshal record: %w", err)
	}

	f, err := os.OpenFile(s.runPath(event.RunID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return rt.Event{}, fmt.Errorf("open wal: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return rt.Event{}, fmt.Errorf("append wal: %w", err)
	}

	return event, nil
}

// List returns the valid persisted prefix of the run history.
func (s *Store) List(runID string) ([]rt.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	events, _, err := s.scanLocked(runID)
	return events, err
}

func (s *Store) RunIDs() ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.baseDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read wal dir: %w", err)
	}

	runIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".wal" {
			continue
		}
		runIDs = append(runIDs, name[:len(name)-len(".wal")])
	}
	return runIDs, nil
}

func (s *Store) runPath(runID string) string {
	return filepath.Join(s.baseDir, runID+".wal")
}

func (s *Store) scanLocked(runID string) ([]rt.Event, bool, error) {
	path := s.runPath(runID)
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("open wal for scan: %w", err)
	}
	defer f.Close()

	var (
		events  []rt.Event
		corrupt bool
		scanner = bufio.NewScanner(f)
	)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		record := diskRecord{}
		if err := json.Unmarshal(line, &record); err != nil {
			corrupt = true
			break
		}
		if checksum(record) != record.CRC32 {
			corrupt = true
			break
		}
		payload, err := decodePayload(record.Type, record.Payload)
		if err != nil {
			corrupt = true
			break
		}
		events = append(events, rt.Event{
			Seq:       record.Seq,
			RunID:     record.RunID,
			Type:      record.Type,
			Timestamp: record.Timestamp,
			Payload:   payload,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, false, fmt.Errorf("scan wal: %w", err)
	}
	if corrupt {
		// A broken tail must not erase the valid prefix that came before it.
		return events, true, ErrCorruptTail
	}
	return events, false, nil
}

func checksum(record diskRecord) uint32 {
	copyRecord := record
	copyRecord.CRC32 = 0
	payload, _ := json.Marshal(copyRecord)
	return crc32.ChecksumIEEE(payload)
}

// decodePayload reconstructs typed payloads so replay operates on the same
// event shapes as live execution.
func decodePayload(eventType rt.EventType, raw json.RawMessage) (interface{}, error) {
	switch eventType {
	case rt.EventWorkflowStarted:
		var payload rt.WorkflowStartedPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case rt.EventLeaseGranted:
		var payload rt.LeaseGrantedPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case rt.EventHeartbeatRecorded:
		var payload rt.HeartbeatRecordedPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case rt.EventLeaseExpired:
		var payload rt.LeaseExpiredPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case rt.EventTimerScheduled:
		var payload rt.TimerScheduledPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case rt.EventTimerFired:
		var payload rt.TimerFiredPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case rt.EventNodeReady:
		var payload rt.NodeReadyPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case rt.EventNodeStarted:
		var payload rt.NodeStartedPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case rt.EventNodeCompleted:
		var payload rt.NodeCompletedPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case rt.EventNodeFailed:
		var payload rt.NodeFailedPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case rt.EventWorkflowFailed:
		var payload rt.WorkflowFailedPayload
		if len(raw) == 0 || string(raw) == "null" {
			return payload, nil
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, err
		}
		return payload, nil
	case rt.EventWorkflowCompleted:
		return rt.WorkflowCompletedPayload{}, nil
	default:
		return nil, fmt.Errorf("unknown event type %q", eventType)
	}
}
