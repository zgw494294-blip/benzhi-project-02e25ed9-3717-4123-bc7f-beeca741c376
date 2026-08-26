package release

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/policy"
	"oral-history-release-desk/internal/store"
)

type Service struct {
	store         *store.FileStore
	policy        *policy.Engine
	now           func() time.Time
	riskMu        sync.Mutex
	riskSummaries map[string]riskSummaryCacheEntry
}

func NewService(repository *store.FileStore, engine *policy.Engine) *Service {
	return &Service{
		store: repository, policy: engine, now: time.Now,
		riskSummaries: make(map[string]riskSummaryCacheEntry),
	}
}

func (s *Service) Get(caseID string) (*casefile.Case, error) {
	return s.store.Get(strings.TrimSpace(caseID))
}

func (s *Service) List() []store.CaseSummary {
	return s.store.List()
}

func (s *Service) newID(prefix string) (string, error) {
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("生成标识: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(random), nil
}

func requiredID(value, prefix string, generate func(string) (string, error)) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" {
		return value, nil
	}
	return generate(prefix)
}

type CommandMeta struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	IdempotencyKey  string `json:"idempotencyKey"`
	Actor           string `json:"actor"`
}

type CommandResult struct {
	Case       *casefile.Case `json:"case"`
	Idempotent bool           `json:"idempotent"`
}

func (s *Service) execute(caseID, operation string, meta CommandMeta, fn func(*casefile.Case, time.Time) error) (CommandResult, error) {
	if meta.ExpectedVersion < 1 {
		return CommandResult{}, fmt.Errorf("expectedVersion 必须大于 0")
	}
	if strings.TrimSpace(meta.IdempotencyKey) == "" {
		return CommandResult{}, fmt.Errorf("idempotencyKey 不能为空")
	}
	if strings.TrimSpace(meta.Actor) == "" {
		return CommandResult{}, fmt.Errorf("actor 不能为空")
	}
	now := s.now().UTC()
	result, replayed, err := s.store.Execute(caseID, meta.ExpectedVersion, operation, meta.IdempotencyKey, now, func(c *casefile.Case) error {
		return fn(c, now)
	})
	return CommandResult{Case: result, Idempotent: replayed}, err
}
