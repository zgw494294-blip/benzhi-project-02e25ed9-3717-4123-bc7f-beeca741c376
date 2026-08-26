package casefile

import "fmt"

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// MultiValidationError 用于一次返回批量命令中所有可定位的字段错误。
type MultiValidationError struct {
	Fields map[string]string `json:"fields"`
}

func (e MultiValidationError) Error() string {
	return fmt.Sprintf("批量校验失败，共 %d 项错误", len(e.Fields))
}

type DecisionConflict struct {
	SegmentID       string            `json:"segmentId"`
	FindingIDs      []string          `json:"findingIds"`
	CandidateResult map[string]string `json:"candidateResults"`
}

type DecisionConflictError struct {
	Conflicts []DecisionConflict `json:"conflicts"`
}

func (e DecisionConflictError) Error() string {
	return fmt.Sprintf("同一片段存在 %d 组互不一致的候选发布文本", len(e.Conflicts))
}

type StateError struct {
	Status    Status
	Operation string
}

func (e StateError) Error() string {
	return fmt.Sprintf("状态 %s 不允许执行 %s", e.Status, e.Operation)
}

func fieldError(field, message string) error {
	return ValidationError{Field: field, Message: message}
}
