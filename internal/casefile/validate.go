package casefile

import (
	"fmt"
)

func (c *Case) ValidateSnapshot() error {
	if c.ID == "" || c.Version < 1 {
		return fmt.Errorf("案件标识或版本无效")
	}
	if !validStatus(c.Status) {
		return fmt.Errorf("案件 %s 状态无效: %s", c.ID, c.Status)
	}
	seenSegments := make(map[string]bool)
	seenSequences := make(map[int]bool, len(c.Segments))
	for _, segment := range c.Segments {
		if segment.ID == "" || segment.CaseID != c.ID || segment.Revision < 1 {
			return fmt.Errorf("案件 %s 包含无效片段", c.ID)
		}
		if seenSegments[segment.ID] {
			return fmt.Errorf("案件 %s 包含重复片段 %s", c.ID, segment.ID)
		}
		seenSegments[segment.ID] = true
		if segment.Sequence < 1 || seenSequences[segment.Sequence] {
			return fmt.Errorf("案件 %s 包含无效或重复片段顺序 %d", c.ID, segment.Sequence)
		}
		seenSequences[segment.Sequence] = true
	}
	for i, event := range c.Timeline {
		if event.Sequence != int64(i+1) {
			return fmt.Errorf("案件 %s 时间线序列不连续", c.ID)
		}
		if event.CaseVersion < 1 || event.CaseVersion > c.Version {
			return fmt.Errorf("案件 %s 时间线版本越界", c.ID)
		}
		if event.CaseVersion != int64(i+1) {
			return fmt.Errorf("案件 %s 版本时间线不连续", c.ID)
		}
	}
	if int64(len(c.Timeline)) != c.Version {
		return fmt.Errorf("案件 %s 当前版本与时间线不一致", c.ID)
	}
	if c.Status == StatusSealed && c.Package == nil {
		return fmt.Errorf("已封存案件 %s 缺少发布包", c.ID)
	}
	if c.Status != StatusSealed && c.Package != nil {
		return fmt.Errorf("未封存案件 %s 不应包含发布包", c.ID)
	}
	return nil
}

func validStatus(status Status) bool {
	switch status {
	case StatusDraft, StatusPendingCheck, StatusRemediation, StatusPendingReview,
		StatusReturned, StatusPendingApproval, StatusSealed:
		return true
	default:
		return false
	}
}
