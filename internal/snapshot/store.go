package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"time"

	"grael/internal/state"
)

var ErrCorruptSnapshot = errors.New("snapshot: corrupt snapshot")

type Store struct {
	baseDir string
	mu      sync.Mutex
}

// Info is a lightweight inspection view for manual verification and tooling.
type Info struct {
	RunID     string    `json:"run_id"`
	Seq       uint64    `json:"seq"`
	CreatedAt time.Time `json:"created_at"`
	Exists    bool      `json:"exists"`
}

type DiskSnapshot struct {
	RunID     string          `json:"run_id"`
	Seq       uint64          `json:"seq"`
	CreatedAt time.Time       `json:"created_at"`
	State     json.RawMessage `json:"state"`
	CRC32     uint32          `json:"crc32"`
}

func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

// Save persists a point-in-time copy of already-derived state. Snapshots are a
// replay optimization and never replace the WAL as the durable source of truth.
func (s *Store) Save(st *state.ExecutionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	stateBytes, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	snap := DiskSnapshot{
		RunID:     st.RunID,
		Seq:       st.LastSeq,
		CreatedAt: time.Now().UTC(),
		State:     stateBytes,
	}
	snap.CRC32 = checksum(snap)

	payload, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	if err := os.WriteFile(s.path(st.RunID), payload, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}

// Load returns a previously persisted state snapshot when one exists.
func (s *Store) Load(runID string) (*state.ExecutionState, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	content, err := os.ReadFile(s.path(runID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read snapshot: %w", err)
	}

	var snap DiskSnapshot
	if err := json.Unmarshal(content, &snap); err != nil {
		return nil, false, fmt.Errorf("decode snapshot: %w", err)
	}
	if checksum(snap) != snap.CRC32 {
		return nil, false, ErrCorruptSnapshot
	}

	var st state.ExecutionState
	if err := json.Unmarshal(snap.State, &st); err != nil {
		return nil, false, fmt.Errorf("decode snapshot state: %w", err)
	}
	if st.Nodes == nil {
		st.Nodes = map[string]*state.Node{}
	}
	return &st, true, nil
}

// Info reads snapshot metadata without rebuilding the full state payload.
func (s *Store) Info(runID string) (Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	content, err := os.ReadFile(s.path(runID))
	if errors.Is(err, os.ErrNotExist) {
		return Info{RunID: runID, Exists: false}, nil
	}
	if err != nil {
		return Info{}, fmt.Errorf("read snapshot: %w", err)
	}

	var snap DiskSnapshot
	if err := json.Unmarshal(content, &snap); err != nil {
		return Info{}, fmt.Errorf("decode snapshot: %w", err)
	}
	if checksum(snap) != snap.CRC32 {
		return Info{}, ErrCorruptSnapshot
	}

	return Info{
		RunID:     snap.RunID,
		Seq:       snap.Seq,
		CreatedAt: snap.CreatedAt,
		Exists:    true,
	}, nil
}

func checksum(snap DiskSnapshot) uint32 {
	copySnap := snap
	copySnap.CRC32 = 0
	payload, _ := json.Marshal(copySnap)
	return crc32.ChecksumIEEE(payload)
}

func (s *Store) path(runID string) string {
	return filepath.Join(s.baseDir, runID+".snapshot")
}
