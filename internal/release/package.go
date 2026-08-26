package release

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"oral-history-release-desk/internal/casefile"
)

type IntegrityError struct {
	StoredDigest     string `json:"storedDigest"`
	CalculatedDigest string `json:"calculatedDigest"`
}

func (e IntegrityError) Error() string {
	return "发布包摘要不一致，证据内容可能已损坏"
}

type ApprovalResult struct {
	Case       *casefile.Case           `json:"case"`
	Package    *casefile.ReleasePackage `json:"releasePackage"`
	Idempotent bool                     `json:"idempotent"`
}

func (s *Service) FinalApprove(caseID string, meta CommandMeta) (ApprovalResult, error) {
	packageID, err := s.newID("package")
	if err != nil {
		return ApprovalResult{}, err
	}
	result, err := s.execute(caseID, "package.seal", meta, func(c *casefile.Case, now time.Time) error {
		pkg, err := buildPackage(c, packageID, meta.Actor, now)
		if err != nil {
			return err
		}
		if err := c.Seal(pkg, meta.Actor, now); err != nil {
			return err
		}
		valid, _, err := casefile.VerifyPackageDigest(*c.Package)
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("封存时发布包摘要不一致")
		}
		return nil
	})
	if err != nil {
		return ApprovalResult{}, err
	}
	return ApprovalResult{Case: result.Case, Package: result.Case.Package, Idempotent: result.Idempotent}, nil
}

func buildPackage(c *casefile.Case, packageID, approvedBy string, now time.Time) (casefile.ReleasePackage, error) {
	if c.Preview == nil {
		return casefile.ReleasePackage{}, fmt.Errorf("缺少已复核发布预览")
	}
	resultBySegment := make(map[string]string)
	for _, segment := range c.Preview.Segments {
		resultBySegment[segment.SegmentID] = segment.After
	}
	snapshots := make([]casefile.DecisionSnapshot, 0, len(c.Decisions))
	for _, decision := range c.Decisions {
		finding := c.Finding(decision.FindingID)
		if finding == nil || finding.ResolutionStatus == casefile.ResolutionObsolete {
			continue
		}
		snapshots = append(snapshots, casefile.DecisionSnapshot{
			FindingID: decision.FindingID, RuleCode: finding.RuleCode, Action: decision.Action,
			ResultText: resultBySegment[finding.SegmentID], Rationale: decision.Rationale,
		})
	}
	sort.SliceStable(snapshots, func(i, j int) bool {
		if snapshots[i].FindingID != snapshots[j].FindingID {
			return snapshots[i].FindingID < snapshots[j].FindingID
		}
		return snapshots[i].RuleCode < snapshots[j].RuleCode
	})
	pkg := casefile.ReleasePackage{
		ID: packageID, CaseID: c.ID, CaseVersion: c.Version + 1,
		NormalizedText: c.Preview.Text, DecisionSnapshot: snapshots,
		ReviewSnapshot: append([]casefile.ReviewRecord(nil), c.Reviews...),
		VersionSummary: fmt.Sprintf("案件 %s 固定版本 v%d；片段 %d；有效校核项 %d；处置 %d；复核 %d",
			c.ID, c.Version+1, len(c.Segments), activeFindingCount(c), len(snapshots), len(c.Reviews)),
		ApprovedBy: approvedBy, SealedAt: now.UTC(),
	}
	digest, err := casefile.PackageDigest(pkg)
	if err != nil {
		return casefile.ReleasePackage{}, err
	}
	pkg.Digest = digest
	return pkg, nil
}

func activeFindingCount(c *casefile.Case) int {
	count := 0
	for _, finding := range c.Findings {
		if finding.ResolutionStatus != casefile.ResolutionObsolete {
			count++
		}
	}
	return count
}

type VerificationResult struct {
	CaseID           string `json:"caseId"`
	PackageID        string `json:"packageId"`
	StoredDigest     string `json:"storedDigest"`
	CalculatedDigest string `json:"calculatedDigest"`
	Valid            bool   `json:"valid"`
}

func (s *Service) VerifyPackage(caseID string) (VerificationResult, error) {
	c, err := s.store.Get(caseID)
	if err != nil {
		return VerificationResult{}, err
	}
	if c.Package == nil {
		return VerificationResult{}, fmt.Errorf("案件尚未封存发布包")
	}
	valid, calculated, err := casefile.VerifyPackageDigest(*c.Package)
	if err != nil {
		return VerificationResult{}, err
	}
	return VerificationResult{
		CaseID: caseID, PackageID: c.Package.ID, StoredDigest: c.Package.Digest,
		CalculatedDigest: calculated, Valid: valid,
	}, nil
}

type EvidenceCounts struct {
	NormalizedTexts  int `json:"normalizedTexts"`
	Decisions        int `json:"decisions"`
	Reviews          int `json:"reviews"`
	VersionSummaries int `json:"versionSummaries"`
	Approvals        int `json:"approvals"`
	Checksums        int `json:"checksums"`
}

type ApprovalEvidence struct {
	ApprovedBy string `json:"approvedBy"`
	SealedAt   string `json:"sealedAt"`
}

type ChecksumEvidence struct {
	StoredDigest     string `json:"storedDigest"`
	CalculatedDigest string `json:"calculatedDigest"`
	Valid            bool   `json:"valid"`
}

type EvidenceCatalog struct {
	PackageID      string                      `json:"packageId"`
	CaseID         string                      `json:"caseId"`
	CaseVersion    int64                       `json:"caseVersion"`
	Counts         EvidenceCounts              `json:"counts"`
	NormalizedText string                      `json:"normalizedText"`
	Decisions      []casefile.DecisionSnapshot `json:"decisions"`
	Reviews        []casefile.ReviewRecord     `json:"reviews"`
	VersionSummary string                      `json:"versionSummary"`
	Approval       ApprovalEvidence            `json:"approval"`
	Checksum       ChecksumEvidence            `json:"checksum"`
}

type EvidenceFilter struct {
	RuleCode string
	Outcome  casefile.ReviewOutcome
}

func (s *Service) EvidenceCatalog(caseID string, filter EvidenceFilter) (EvidenceCatalog, error) {
	pkg, checksum, err := s.verifiedPackage(caseID)
	if err != nil {
		return EvidenceCatalog{}, err
	}
	ruleCode := strings.TrimSpace(filter.RuleCode)
	if ruleCode != "" && !validFilterToken(ruleCode) {
		return EvidenceCatalog{}, casefile.ValidationError{Field: "ruleCode", Message: "ruleCode 筛选格式无效"}
	}
	if filter.Outcome != "" && filter.Outcome != casefile.ReviewApproved && filter.Outcome != casefile.ReviewReturned {
		return EvidenceCatalog{}, casefile.ValidationError{Field: "outcome", Message: "outcome 必须为 approved 或 returned"}
	}
	decisions, reviews := canonicalEvidenceArrays(pkg)
	filteredDecisions := make([]casefile.DecisionSnapshot, 0, len(decisions))
	for _, decision := range decisions {
		if ruleCode == "" || decision.RuleCode == ruleCode {
			filteredDecisions = append(filteredDecisions, decision)
		}
	}
	filteredReviews := make([]casefile.ReviewRecord, 0, len(reviews))
	for _, review := range reviews {
		if filter.Outcome == "" || review.Outcome == filter.Outcome {
			filteredReviews = append(filteredReviews, review)
		}
	}
	return EvidenceCatalog{
		PackageID: pkg.ID, CaseID: pkg.CaseID, CaseVersion: pkg.CaseVersion,
		Counts:         EvidenceCounts{NormalizedTexts: 1, Decisions: len(filteredDecisions), Reviews: len(filteredReviews), VersionSummaries: 1, Approvals: 1, Checksums: 1},
		NormalizedText: pkg.NormalizedText, Decisions: filteredDecisions, Reviews: filteredReviews,
		VersionSummary: pkg.VersionSummary,
		Approval:       ApprovalEvidence{ApprovedBy: pkg.ApprovedBy, SealedAt: canonicalTime(pkg.SealedAt)},
		Checksum:       checksum,
	}, nil
}

type PackageDownload struct {
	PackageID      string                      `json:"packageId"`
	CaseID         string                      `json:"caseId"`
	CaseVersion    int64                       `json:"caseVersion"`
	SealedAt       string                      `json:"sealedAt"`
	Digest         string                      `json:"digest"`
	NormalizedText string                      `json:"normalizedText"`
	Decisions      []casefile.DecisionSnapshot `json:"decisions"`
	Reviews        []casefile.ReviewRecord     `json:"reviews"`
	VersionSummary string                      `json:"versionSummary"`
	Approval       ApprovalEvidence            `json:"approval"`
	Checksum       ChecksumEvidence            `json:"checksum"`
}

func (s *Service) PackageDownload(caseID string) (string, []byte, error) {
	pkg, checksum, err := s.verifiedPackage(caseID)
	if err != nil {
		return "", nil, err
	}
	decisions, reviews := canonicalEvidenceArrays(pkg)
	payload := PackageDownload{
		PackageID: pkg.ID, CaseID: pkg.CaseID, CaseVersion: pkg.CaseVersion,
		SealedAt: canonicalTime(pkg.SealedAt), Digest: pkg.Digest, NormalizedText: pkg.NormalizedText,
		Decisions: decisions, Reviews: reviews, VersionSummary: pkg.VersionSummary,
		Approval: ApprovalEvidence{ApprovedBy: pkg.ApprovedBy, SealedAt: canonicalTime(pkg.SealedAt)}, Checksum: checksum,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("编码规范化发布包下载: %w", err)
	}
	return fmt.Sprintf("oral-history-release-%s-v%d.json", safeFilenamePart(pkg.CaseID), pkg.CaseVersion), data, nil
}

func (s *Service) verifiedPackage(caseID string) (casefile.ReleasePackage, ChecksumEvidence, error) {
	c, err := s.store.Get(caseID)
	if err != nil {
		return casefile.ReleasePackage{}, ChecksumEvidence{}, err
	}
	if c.Package == nil {
		return casefile.ReleasePackage{}, ChecksumEvidence{}, fmt.Errorf("案件尚未封存发布包")
	}
	valid, calculated, err := casefile.VerifyPackageDigest(*c.Package)
	if err != nil {
		return casefile.ReleasePackage{}, ChecksumEvidence{}, err
	}
	checksum := ChecksumEvidence{StoredDigest: c.Package.Digest, CalculatedDigest: calculated, Valid: valid}
	if !valid {
		return casefile.ReleasePackage{}, checksum, IntegrityError{StoredDigest: c.Package.Digest, CalculatedDigest: calculated}
	}
	return *c.Package, checksum, nil
}

func canonicalEvidenceArrays(pkg casefile.ReleasePackage) ([]casefile.DecisionSnapshot, []casefile.ReviewRecord) {
	decisions := append([]casefile.DecisionSnapshot(nil), pkg.DecisionSnapshot...)
	sort.SliceStable(decisions, func(i, j int) bool {
		if decisions[i].RuleCode != decisions[j].RuleCode {
			return decisions[i].RuleCode < decisions[j].RuleCode
		}
		return decisions[i].FindingID < decisions[j].FindingID
	})
	reviews := append([]casefile.ReviewRecord(nil), pkg.ReviewSnapshot...)
	for i := range reviews {
		reviews[i].AffectedSegmentIDs = append([]string(nil), reviews[i].AffectedSegmentIDs...)
		sort.Strings(reviews[i].AffectedSegmentIDs)
		reviews[i].Items = append([]casefile.ReviewItem(nil), reviews[i].Items...)
		sort.SliceStable(reviews[i].Items, func(a, b int) bool { return reviews[i].Items[a].FindingID < reviews[i].Items[b].FindingID })
	}
	sort.SliceStable(reviews, func(i, j int) bool {
		if !reviews[i].CreatedAt.Equal(reviews[j].CreatedAt) {
			return reviews[i].CreatedAt.Before(reviews[j].CreatedAt)
		}
		return reviews[i].ID < reviews[j].ID
	})
	return decisions, reviews
}

func canonicalTime(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05.000000000Z")
}

func safeFilenamePart(value string) string {
	var result strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			result.WriteRune(char)
		} else {
			result.WriteByte('_')
		}
	}
	if result.Len() == 0 {
		return "case"
	}
	return result.String()
}
