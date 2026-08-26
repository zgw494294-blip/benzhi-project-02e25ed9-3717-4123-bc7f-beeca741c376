package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"oral-history-release-desk/internal/casefile"
)

func (s *FileStore) Create(c *casefile.Case, operation, idempotencyKey string, now time.Time) (*casefile.Case, bool, error) {
	if c == nil {
		return nil, false, fmt.Errorf("案件不能为空")
	}
	return s.commit(c.ID, 0, operation, idempotencyKey, now, func(candidate *ledger) (*casefile.Case, error) {
		if _, exists := candidate.Cases[c.ID]; exists {
			return nil, fmt.Errorf("案件 ID 已存在")
		}
		created := c.Clone()
		candidate.Cases[c.ID] = created
		return created, nil
	})
}

func (s *FileStore) Execute(caseID string, expectedVersion int64, operation, idempotencyKey string, now time.Time, mutate func(*casefile.Case) error) (*casefile.Case, bool, error) {
	if mutate == nil {
		return nil, false, fmt.Errorf("变更函数不能为空")
	}
	return s.commit(caseID, expectedVersion, operation, idempotencyKey, now, func(candidate *ledger) (*casefile.Case, error) {
		current, ok := candidate.Cases[caseID]
		if !ok {
			return nil, ErrNotFound
		}
		if current.Version != expectedVersion {
			return nil, VersionConflictError{Expected: expectedVersion, Actual: current.Version}
		}
		working := current.Clone()
		originalEvents := len(working.Timeline)
		if err := mutate(working); err != nil {
			return nil, err
		}
		working.Version = current.Version + 1
		working.UpdatedAt = now.UTC()
		for i := originalEvents; i < len(working.Timeline); i++ {
			working.Timeline[i].CaseVersion = working.Version
		}
		if err := working.ValidateSnapshot(); err != nil {
			return nil, fmt.Errorf("变更后案件无效: %w", err)
		}
		candidate.Cases[caseID] = working
		return working, nil
	})
}

type ledgerMutation func(*ledger) (*casefile.Case, error)

func (s *FileStore) commit(caseID string, expectedVersion int64, operation, key string, now time.Time, mutate ledgerMutation) (*casefile.Case, bool, error) {
	if strings.TrimSpace(caseID) == "" || strings.TrimSpace(operation) == "" {
		return nil, false, fmt.Errorf("案件 ID 和操作名不能为空")
	}
	if strings.TrimSpace(key) == "" {
		return nil, false, fmt.Errorf("幂等键不能为空")
	}
	s.writeSem <- struct{}{}
	defer func() { <-s.writeSem }()
	s.mu.Lock()
	defer s.mu.Unlock()
	compoundKey := caseID + "\x00" + key
	if record, ok := s.state.Idempotency[compoundKey]; ok {
		if record.Operation != operation {
			return nil, false, IdempotencyConflictError{Key: key, OriginalOperation: record.Operation, NewOperation: operation}
		}
		return record.Result.Clone(), true, nil
	}
	candidate := cloneLedger(s.state)
	result, err := mutate(&candidate)
	if err != nil {
		return nil, false, err
	}
	candidate.LedgerSequence++
	candidate.Idempotency[compoundKey] = idempotencyRecord{
		CaseID: caseID, Key: key, Operation: operation, Result: result.Clone(),
		LedgerSequence: candidate.LedgerSequence, CreatedAt: now.UTC(),
	}
	if err := validateLedger(candidate); err != nil {
		return nil, false, fmt.Errorf("候选账本无效: %w", err)
	}
	if err := s.writeAtomic(candidate); err != nil {
		return nil, false, err
	}
	s.state = candidate
	return result.Clone(), false, nil
}

func cloneLedger(source ledger) ledger {
	result := ledger{
		SchemaVersion: source.SchemaVersion, LedgerSequence: source.LedgerSequence,
		Cases:       make(map[string]*casefile.Case, len(source.Cases)),
		Idempotency: make(map[string]idempotencyRecord, len(source.Idempotency)),
	}
	for id, c := range source.Cases {
		result.Cases[id] = c.Clone()
	}
	for key, record := range source.Idempotency {
		record.Result = record.Result.Clone()
		result.Idempotency[key] = record
	}
	return result
}

func (s *FileStore) writeAtomic(candidate ledger) error {
	temp, err := os.CreateTemp(s.dir, ".ledger-*.tmp")
	if err != nil {
		return fmt.Errorf("创建候选快照: %w", err)
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0o640); err != nil {
		_ = temp.Close()
		return fmt.Errorf("设置候选快照权限: %w", err)
	}
	encoder := json.NewEncoder(temp)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(candidate); err != nil {
		_ = temp.Close()
		return fmt.Errorf("编码候选快照: %w", err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("同步候选快照: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("关闭候选快照: %w", err)
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		return fmt.Errorf("原子替换账本: %w", err)
	}
	removeTemp = false
	directory, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return fmt.Errorf("打开账本目录: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("同步账本目录: %w", err)
	}
	return nil
}
