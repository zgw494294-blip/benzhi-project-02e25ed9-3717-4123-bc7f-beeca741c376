package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/release"
)

type caseResponse struct {
	Case *casefile.Case `json:"case"`
}

func runSelfCheck(address string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	client := newCheckClient(address)
	var health map[string]any
	if err := client.request(ctx, http.MethodGet, "/api/v1/health", nil, &health); err != nil {
		return err
	}
	caseID := "selfcheck-case"
	current, err := selfCheckCreate(ctx, client, caseID)
	if err != nil {
		return err
	}
	current, err = selfCheckSegment(ctx, client, current)
	if err != nil {
		return err
	}
	current, err = selfCheckMeta(ctx, client, current, "freeze", "freeze")
	if err != nil {
		return err
	}
	current, err = selfCheckMeta(ctx, client, current, "checks", "check")
	if err != nil {
		return err
	}
	if current.Status != casefile.StatusRemediation || len(current.OpenBlocks()) == 0 {
		return fmt.Errorf("校核未产生预期阻断项")
	}
	for _, finding := range current.OpenBlocks() {
		body := release.DecideFindingInput{
			CommandMeta: release.CommandMeta{ExpectedVersion: current.Version, IdempotencyKey: client.key("decision"), Actor: "自检整理员"},
			FindingID:   finding.ID, Action: casefile.ActionRedact, Rationale: "自检确认该敏感内容不进入公开文本",
		}
		var response caseResponse
		if err := client.request(ctx, http.MethodPost, "/api/v1/cases/"+url.PathEscape(caseID)+"/decisions", body, &response); err != nil {
			return err
		}
		current = response.Case
	}
	current, err = selfCheckMeta(ctx, client, current, "preview", "preview")
	if err != nil {
		return err
	}
	if current.Preview == nil || current.Preview.Text == "" {
		return fmt.Errorf("发布预览未生成")
	}
	current, err = selfCheckMeta(ctx, client, current, "review/submit", "submit-review")
	if err != nil {
		return err
	}
	items := make([]casefile.ReviewItem, 0)
	for _, finding := range current.Findings {
		if finding.ResolutionStatus != casefile.ResolutionObsolete && finding.Severity != casefile.SeverityPass {
			items = append(items, casefile.ReviewItem{FindingID: finding.ID, Confirmed: true})
		}
	}
	reviewBody := release.ReviewDecisionInput{
		CommandMeta: release.CommandMeta{ExpectedVersion: current.Version, IdempotencyKey: client.key("review"), Actor: "自检隐私复核员"},
		Outcome:     casefile.ReviewApproved, Items: items,
	}
	var reviewResponse caseResponse
	if err := client.request(ctx, http.MethodPost, "/api/v1/cases/"+url.PathEscape(caseID)+"/review/decision", reviewBody, &reviewResponse); err != nil {
		return err
	}
	current = reviewResponse.Case
	approval := release.CommandMeta{ExpectedVersion: current.Version, IdempotencyKey: client.key("approval"), Actor: "自检开放负责人"}
	var approvalResponse struct {
		Case    *casefile.Case           `json:"case"`
		Package *casefile.ReleasePackage `json:"releasePackage"`
	}
	if err := client.request(ctx, http.MethodPost, "/api/v1/cases/"+url.PathEscape(caseID)+"/approval", approval, &approvalResponse); err != nil {
		return err
	}
	if approvalResponse.Case.Status != casefile.StatusSealed || approvalResponse.Package == nil {
		return fmt.Errorf("最终批准未封存发布包")
	}
	var verification release.VerificationResult
	if err := client.request(ctx, http.MethodGet, "/api/v1/cases/"+url.PathEscape(caseID)+"/package/verify", nil, &verification); err != nil {
		return err
	}
	if !verification.Valid || verification.StoredDigest != verification.CalculatedDigest {
		return fmt.Errorf("发布包摘要校验不一致")
	}
	return nil
}

func selfCheckCreate(ctx context.Context, client *checkClient, caseID string) (*casefile.Case, error) {
	body := release.CreateCaseInput{
		ID: caseID, Title: "自检口述史开放案", InterviewDate: "2024-05-18", IntendedUse: "公开研究与教育展览",
		ConsentScope: []string{"公开研究利用"}, RestrictionTerms: []string{"不得披露受访者姓名"},
		Actor: "自检整理员", IdempotencyKey: client.key("create"),
	}
	var response caseResponse
	err := client.request(ctx, http.MethodPost, "/api/v1/cases", body, &response)
	return response.Case, err
}

func selfCheckSegment(ctx context.Context, client *checkClient, current *casefile.Case) (*casefile.Case, error) {
	body := release.AddSegmentInput{
		CommandMeta: release.CommandMeta{ExpectedVersion: current.Version, IdempotencyKey: client.key("segment"), Actor: "自检整理员"},
		ID:          "segment-1", Sequence: 1, StartMillis: 0, EndMillis: 12500,
		OriginalText: "我叫某某，曾经住在老城北街。", SpeakerLabel: "受访者甲",
		SensitivityTags: []string{"姓名", "住址"}, RiskNote: "公开材料应避免身份回溯",
	}
	var response caseResponse
	err := client.request(ctx, http.MethodPost, "/api/v1/cases/"+url.PathEscape(current.ID)+"/segments", body, &response)
	return response.Case, err
}

func selfCheckMeta(ctx context.Context, client *checkClient, current *casefile.Case, suffix, operation string) (*casefile.Case, error) {
	body := release.CommandMeta{ExpectedVersion: current.Version, IdempotencyKey: client.key(operation), Actor: "自检整理员"}
	var response caseResponse
	err := client.request(ctx, http.MethodPost, "/api/v1/cases/"+url.PathEscape(current.ID)+"/"+suffix, body, &response)
	return response.Case, err
}
