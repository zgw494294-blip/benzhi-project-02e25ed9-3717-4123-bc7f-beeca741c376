package policy

import (
	"strings"

	"oral-history-release-desk/internal/casefile"
)

var sensitiveBlockTags = []string{"身份", "姓名", "identity", "健康", "health", "住址", "location", "第三方", "third_party"}
var sensitiveWarningTags = []string{"政治", "political", "宗教", "religion", "财务", "financial"}

func (e *Engine) evaluateSegment(c *casefile.Case, segment casefile.TranscriptSegment, runID string) []casefile.PolicyFinding {
	result := make([]casefile.PolicyFinding, 0, 5)
	publicUse := containsFold([]string{c.IntendedUse}, "公开", "public", "展览", "网络", "出版")
	publicConsent := containsFold(c.ConsentScope, "公开", "public", "出版", "展览", "研究利用")
	if publicUse && !publicConsent {
		result = append(result, finding(c, segment, runID, "CONSENT_SCOPE_MISMATCH", casefile.SeverityBlock,
			"拟开放用途包含公开利用，但参与者授权范围未明确覆盖公开、出版、展览或研究利用。"))
	}
	identityRestricted := containsFold(c.RestrictionTerms, "姓名", "身份", "匿名", "identity", "name")
	if identityRestricted && containsFold(segment.SensitivityTags, "身份", "姓名", "identity") {
		result = append(result, finding(c, segment, runID, "RESTRICTION_IDENTITY", casefile.SeverityBlock,
			"限制条款要求隐藏身份，而片段标记了姓名或身份信息。"))
	}
	if containsFold(segment.SensitivityTags, sensitiveBlockTags...) {
		result = append(result, finding(c, segment, runID, "SENSITIVE_PERSONAL_DATA", casefile.SeverityBlock,
			"片段含身份、健康、住址或第三方个人信息，公开前必须明确保留、替换或遮蔽。"))
	}
	if containsFold(segment.SensitivityTags, sensitiveWarningTags...) {
		result = append(result, finding(c, segment, runID, "SENSITIVE_CONTEXT_REVIEW", casefile.SeverityWarning,
			"片段含政治、宗教或财务语境，建议复核员结合上下文确认公开风险。"))
	}
	if strings.TrimSpace(segment.RiskNote) != "" {
		result = append(result, finding(c, segment, runID, "CURATOR_RISK_NOTE", casefile.SeverityWarning,
			"整理员已记录风险说明："+strings.TrimSpace(segment.RiskNote)))
	}
	if len(result) == 0 {
		result = append(result, finding(c, segment, runID, "SEGMENT_CLEAR", casefile.SeverityPass,
			"片段未触发授权边界、限制条款或敏感类别规则。"))
	}
	return result
}
