package store

import (
	"errors"
	"fmt"
	"time"

	"oral-history-release-desk/internal/casefile"
)

const SchemaVersion = 1

var ErrNotFound = errors.New("案件不存在")

type VersionConflictError struct {
	Expected int64
	Actual   int64
}

func (e VersionConflictError) Error() string {
	return fmt.Sprintf("版本冲突：期望 %d，实际 %d", e.Expected, e.Actual)
}

type IdempotencyConflictError struct {
	Key               string
	OriginalOperation string
	NewOperation      string
}

func (e IdempotencyConflictError) Error() string {
	return fmt.Sprintf("幂等键 %q 已用于 %s，不能用于 %s", e.Key, e.OriginalOperation, e.NewOperation)
}

type idempotencyRecord struct {
	CaseID         string         `json:"caseId"`
	Key            string         `json:"key"`
	Operation      string         `json:"operation"`
	Result         *casefile.Case `json:"result"`
	LedgerSequence int64          `json:"ledgerSequence"`
	CreatedAt      time.Time      `json:"createdAt"`
}

type ledger struct {
	SchemaVersion  int                          `json:"schemaVersion"`
	LedgerSequence int64                        `json:"ledgerSequence"`
	Cases          map[string]*casefile.Case    `json:"cases"`
	Idempotency    map[string]idempotencyRecord `json:"idempotency"`
}

type CaseSummary struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Status    casefile.Status `json:"status"`
	Version   int64           `json:"version"`
	UpdatedAt time.Time       `json:"updatedAt"`
}
