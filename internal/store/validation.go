package store

import (
	"fmt"
	"strings"

	"oral-history-release-desk/internal/casefile"
)

func validateLedger(state ledger) error {
	if state.SchemaVersion != SchemaVersion {
		return fmt.Errorf("不支持 schemaVersion %d，期望 %d", state.SchemaVersion, SchemaVersion)
	}
	if state.LedgerSequence < 0 {
		return fmt.Errorf("账本序列不能为负数")
	}
	if state.Cases == nil || state.Idempotency == nil {
		return fmt.Errorf("账本映射缺失")
	}
	if int64(len(state.Idempotency)) != state.LedgerSequence {
		return fmt.Errorf("账本序列 %d 与提交记录数 %d 不一致", state.LedgerSequence, len(state.Idempotency))
	}
	ledgerSequences := make(map[int64]bool, len(state.Idempotency))
	for id, c := range state.Cases {
		if c == nil || c.ID != id {
			return fmt.Errorf("案件映射键不一致: %s", id)
		}
		if err := c.ValidateSnapshot(); err != nil {
			return err
		}
		if c.Package != nil {
			valid, calculated, err := casefile.VerifyPackageDigest(*c.Package)
			if err != nil {
				return fmt.Errorf("案件 %s 发布包摘要计算失败: %w", id, err)
			}
			if !valid {
				return fmt.Errorf("案件 %s 发布包摘要不匹配，重算为 %s", id, calculated)
			}
		}
	}
	for compoundKey, record := range state.Idempotency {
		if record.Result == nil || record.CaseID == "" || record.Key == "" || record.Operation == "" {
			return fmt.Errorf("幂等记录 %q 不完整", compoundKey)
		}
		if compoundKey != record.CaseID+"\x00"+record.Key {
			return fmt.Errorf("幂等记录 %q 键不一致", compoundKey)
		}
		if record.LedgerSequence < 1 || record.LedgerSequence > state.LedgerSequence {
			return fmt.Errorf("幂等记录 %q 账本序列越界", compoundKey)
		}
		if ledgerSequences[record.LedgerSequence] {
			return fmt.Errorf("账本序列 %d 重复", record.LedgerSequence)
		}
		ledgerSequences[record.LedgerSequence] = true
		if strings.TrimSpace(record.Result.ID) != record.CaseID {
			return fmt.Errorf("幂等记录 %q 案件快照不一致", compoundKey)
		}
		if err := validateIdempotencyRecord(record); err != nil {
			return fmt.Errorf("幂等记录 %q %w", compoundKey, err)
		}
	}
	for sequence := int64(1); sequence <= state.LedgerSequence; sequence++ {
		if !ledgerSequences[sequence] {
			return fmt.Errorf("账本序列 %d 缺失", sequence)
		}
	}
	return nil
}

// validateIdempotencyRecord 检查单条幂等记录的结果快照是否结构有效，
// 且若包含发布包则摘要仍然一致。它在启动校验与同键重放路径中共同使用，
// 防止历史快照被替换为与时间线不匹配的版本或摘要被破坏后仍被接受。
func validateIdempotencyRecord(record idempotencyRecord) error {
	if err := record.Result.ValidateSnapshot(); err != nil {
		return fmt.Errorf("快照无效: %w", err)
	}
	if record.Result.Package != nil {
		valid, calculated, err := casefile.VerifyPackageDigest(*record.Result.Package)
		if err != nil {
			return fmt.Errorf("发布包摘要计算失败: %w", err)
		}
		if !valid {
			return fmt.Errorf("发布包摘要不匹配，重算为 %s", calculated)
		}
	}
	return nil
}
