package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"oral-history-release-desk/internal/casefile"
)

type FileStore struct {
	mu       sync.RWMutex
	dir      string
	path     string
	state    ledger
	writeSem chan struct{}
}

func Open(dir string) (*FileStore, error) {
	if dir == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("创建数据目录: %w", err)
	}
	s := &FileStore{
		dir:      dir,
		path:     filepath.Join(dir, "ledger.json"),
		writeSem: make(chan struct{}, 1),
		state: ledger{
			SchemaVersion: SchemaVersion,
			Cases:         make(map[string]*casefile.Case),
			Idempotency:   make(map[string]idempotencyRecord),
		},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *FileStore) load() error {
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("打开账本: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 64<<20))
	decoder.DisallowUnknownFields()
	var loaded ledger
	if err := decoder.Decode(&loaded); err != nil {
		return fmt.Errorf("账本截断或损坏: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("账本包含额外数据")
	}
	if err := validateLedger(loaded); err != nil {
		return fmt.Errorf("账本校验失败: %w", err)
	}
	s.state = loaded
	return nil
}

func (s *FileStore) Get(caseID string) (*casefile.Case, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.state.Cases[caseID]
	if !ok {
		return nil, ErrNotFound
	}
	return c.Clone(), nil
}

func (s *FileStore) List() []CaseSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]CaseSummary, 0, len(s.state.Cases))
	for _, c := range s.state.Cases {
		result = append(result, CaseSummary{ID: c.ID, Title: c.Title, Status: c.Status, Version: c.Version, UpdatedAt: c.UpdatedAt})
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].UpdatedAt.Equal(result[j].UpdatedAt) {
			return result[i].ID < result[j].ID
		}
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

func (s *FileStore) Close() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	close(s.writeSem)
	return nil
}

func (s *FileStore) LedgerSequence() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.LedgerSequence
}
