package casefile

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type SegmentInput struct {
	ID              string
	Sequence        int
	StartMillis     int64
	EndMillis       int64
	OriginalText    string
	SpeakerLabel    string
	SensitivityTags []string
	RiskNote        string
}

func (c *Case) AddSegment(in SegmentInput, actor string, now time.Time) error {
	if c.Status != StatusDraft && c.Status != StatusReturned {
		return StateError{Status: c.Status, Operation: "登记片段"}
	}
	if c.Status == StatusReturned {
		return fieldError("segmentId", "退回后只能修改明确受影响的既有片段")
	}
	segment, err := c.validateSegment(in, "")
	if err != nil {
		return err
	}
	segment.CaseID = c.ID
	segment.Revision = 1
	c.Segments = append(c.Segments, segment)
	c.sortSegments()
	c.Touch(now)
	c.AppendEvent("segment.added", c.Status, c.Status, actor, "登记转录片段 "+segment.ID, now)
	return nil
}

func (c *Case) AddSegments(inputs []SegmentInput, actor string, now time.Time) error {
	if c.Status != StatusDraft {
		return StateError{Status: c.Status, Operation: "批量登记片段"}
	}
	if strings.TrimSpace(actor) == "" {
		return fieldError("actor", "操作者不能为空")
	}
	if len(inputs) == 0 {
		return fieldError("segments", "至少提交一条转录片段")
	}
	errorsByField := make(map[string]string)
	normalized := make([]TranscriptSegment, len(inputs))
	for row, input := range inputs {
		prefix := fmt.Sprintf("segments[%d].", row)
		normalized[row] = normalizeSegment(input)
		segment := normalized[row]
		if segment.ID == "" {
			errorsByField[prefix+"id"] = "片段 ID 不能为空"
		}
		if segment.Sequence < 1 {
			errorsByField[prefix+"sequence"] = "片段顺序必须大于 0"
		}
		if segment.StartMillis < 0 || segment.EndMillis <= segment.StartMillis {
			errorsByField[prefix+"timeRange"] = "结束时间必须大于非负的开始时间"
		}
		if segment.OriginalText == "" {
			errorsByField[prefix+"originalText"] = "转录文本不能为空"
		}
		if segment.SpeakerLabel == "" {
			errorsByField[prefix+"speakerLabel"] = "人物身份不能为空"
		}
		for _, existing := range c.Segments {
			if segment.ID != "" && existing.ID == segment.ID {
				errorsByField[prefix+"id"] = "片段 ID 已存在"
			}
			if segment.Sequence > 0 && existing.Sequence == segment.Sequence {
				errorsByField[prefix+"sequence"] = "片段顺序已被已有片段占用"
			}
			if validRange(segment.StartMillis, segment.EndMillis) && rangesOverlap(segment.StartMillis, segment.EndMillis, existing.StartMillis, existing.EndMillis) {
				errorsByField[prefix+"timeRange"] = "片段时间范围与已有片段重叠"
			}
		}
	}
	for i := 0; i < len(normalized); i++ {
		for j := i + 1; j < len(normalized); j++ {
			left, right := normalized[i], normalized[j]
			if left.ID != "" && left.ID == right.ID {
				errorsByField[fmt.Sprintf("segments[%d].id", i)] = fmt.Sprintf("与第 %d 行片段 ID 重复", j+1)
				errorsByField[fmt.Sprintf("segments[%d].id", j)] = fmt.Sprintf("与第 %d 行片段 ID 重复", i+1)
			}
			if left.Sequence > 0 && left.Sequence == right.Sequence {
				errorsByField[fmt.Sprintf("segments[%d].sequence", i)] = fmt.Sprintf("与第 %d 行片段顺序重复", j+1)
				errorsByField[fmt.Sprintf("segments[%d].sequence", j)] = fmt.Sprintf("与第 %d 行片段顺序重复", i+1)
			}
			if validRange(left.StartMillis, left.EndMillis) && validRange(right.StartMillis, right.EndMillis) && rangesOverlap(left.StartMillis, left.EndMillis, right.StartMillis, right.EndMillis) {
				errorsByField[fmt.Sprintf("segments[%d].timeRange", i)] = fmt.Sprintf("与第 %d 行时间范围重叠", j+1)
				errorsByField[fmt.Sprintf("segments[%d].timeRange", j)] = fmt.Sprintf("与第 %d 行时间范围重叠", i+1)
			}
		}
	}
	if len(errorsByField) > 0 {
		return MultiValidationError{Fields: errorsByField}
	}
	minSequence, maxSequence := normalized[0].Sequence, normalized[0].Sequence
	for i := range normalized {
		normalized[i].CaseID = c.ID
		normalized[i].Revision = 1
		c.Segments = append(c.Segments, normalized[i])
		if normalized[i].Sequence < minSequence {
			minSequence = normalized[i].Sequence
		}
		if normalized[i].Sequence > maxSequence {
			maxSequence = normalized[i].Sequence
		}
	}
	c.sortSegments()
	c.Touch(now)
	c.AppendEvent("segments.batch_added", c.Status, c.Status, actor,
		fmt.Sprintf("批量登记 %d 个转录片段，顺序范围 %d-%d", len(normalized), minSequence, maxSequence), now)
	return nil
}

func (c *Case) ReviseReturnedSegment(in SegmentInput, actor string, now time.Time) error {
	if c.Status != StatusReturned {
		return StateError{Status: c.Status, Operation: "定向整改片段"}
	}
	idx := c.segmentIndex(in.ID)
	if idx < 0 {
		return fieldError("segmentId", "片段不存在")
	}
	if !c.Segments[idx].NeedsRecheck {
		return fieldError("segmentId", "该片段未被复核退回，不能修改")
	}
	segment, err := c.validateSegment(in, in.ID)
	if err != nil {
		return err
	}
	segment.CaseID = c.ID
	segment.Revision = c.Segments[idx].Revision + 1
	segment.NeedsRecheck = true
	c.Segments[idx] = segment
	c.sortSegments()
	for i := range c.Findings {
		if c.Findings[i].SegmentID == segment.ID && c.Findings[i].ResolutionStatus != ResolutionObsolete {
			c.Findings[i].ResolutionStatus = ResolutionObsolete
		}
	}
	c.Touch(now)
	c.AppendEvent("segment.revised", c.Status, c.Status, actor, "定向整改片段 "+segment.ID, now)
	return nil
}

func (c *Case) validateSegment(in SegmentInput, replacingID string) (TranscriptSegment, error) {
	segment := normalizeSegment(in)
	if segment.ID == "" {
		return TranscriptSegment{}, fieldError("id", "片段 ID 不能为空")
	}
	if in.Sequence < 1 {
		return TranscriptSegment{}, fieldError("sequence", "片段顺序必须大于 0")
	}
	if in.StartMillis < 0 || in.EndMillis <= in.StartMillis {
		return TranscriptSegment{}, fieldError("timeRange", "结束时间必须大于非负的开始时间")
	}
	if segment.OriginalText == "" {
		return TranscriptSegment{}, fieldError("originalText", "转录文本不能为空")
	}
	if segment.SpeakerLabel == "" {
		return TranscriptSegment{}, fieldError("speakerLabel", "人物身份不能为空")
	}
	for _, existing := range c.Segments {
		if existing.ID != replacingID && existing.ID == segment.ID {
			return TranscriptSegment{}, fieldError("id", "片段 ID 已存在")
		}
		if existing.ID != replacingID && existing.Sequence == in.Sequence {
			return TranscriptSegment{}, fieldError("sequence", "片段顺序已被占用")
		}
		if existing.ID != replacingID && rangesOverlap(in.StartMillis, in.EndMillis, existing.StartMillis, existing.EndMillis) {
			return TranscriptSegment{}, fieldError("timeRange", "片段时间范围与已有片段重叠")
		}
	}
	return segment, nil
}

func normalizeSegment(in SegmentInput) TranscriptSegment {
	return TranscriptSegment{
		ID: strings.TrimSpace(in.ID), Sequence: in.Sequence, StartMillis: in.StartMillis, EndMillis: in.EndMillis,
		OriginalText: strings.TrimSpace(in.OriginalText), SpeakerLabel: strings.TrimSpace(in.SpeakerLabel),
		SensitivityTags: cleanStrings(in.SensitivityTags), RiskNote: strings.TrimSpace(in.RiskNote),
	}
}

func validRange(start, end int64) bool { return start >= 0 && end > start }

func rangesOverlap(aStart, aEnd, bStart, bEnd int64) bool {
	return aStart < bEnd && bStart < aEnd
}

func (c *Case) sortSegments() {
	sort.SliceStable(c.Segments, func(i, j int) bool {
		return c.Segments[i].Sequence < c.Segments[j].Sequence
	})
}

func (c *Case) segmentIndex(id string) int {
	for i := range c.Segments {
		if c.Segments[i].ID == id {
			return i
		}
	}
	return -1
}

func (c *Case) Segment(id string) *TranscriptSegment {
	idx := c.segmentIndex(id)
	if idx < 0 {
		return nil
	}
	return &c.Segments[idx]
}
