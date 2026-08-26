package release

import "oral-history-release-desk/internal/casefile"

type TimelineResult struct {
	CaseID   string                   `json:"caseId"`
	Version  int64                    `json:"version"`
	Timeline []casefile.TimelineEvent `json:"timeline"`
}

func (s *Service) Timeline(caseID string) (TimelineResult, error) {
	s.timelineMu.Lock()
	defer s.timelineMu.Unlock()
	if cached, ok := s.timelineCache[caseID]; ok {
		cached.Timeline = append([]casefile.TimelineEvent(nil), cached.Timeline...)
		return cached, nil
	}
	c, err := s.store.Get(caseID)
	if err != nil {
		return TimelineResult{}, err
	}
	result := TimelineResult{
		CaseID: c.ID, Version: c.Version,
		Timeline: append([]casefile.TimelineEvent(nil), c.Timeline...),
	}
	if c.Status == casefile.StatusReturned {
		s.timelineCache[caseID] = result
	}
	result.Timeline = append([]casefile.TimelineEvent(nil), result.Timeline...)
	return result, nil
}
