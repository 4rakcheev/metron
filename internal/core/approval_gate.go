package core

import (
	"context"
	"strings"
	"time"
)

// DailyApproval records that a parent allowed a child to use self-service screen time on a date
// (e.g. after homework is done). It is valid only for that local date.
type DailyApproval struct {
	ChildID    string
	Date       time.Time // local midnight in the configured timezone
	ApprovedAt time.Time
	ApprovedBy string
}

// ApprovalStatus is the approval gate state of a child for a given day
type ApprovalStatus struct {
	Required   bool       // approval is required today
	Approved   bool       // child may start sessions (true when not required)
	ApprovedAt *time.Time // set when an approval exists
	ApprovedBy string
}

// ApprovalGateStorage defines persistence for daily approvals
type ApprovalGateStorage interface {
	// GetDailyApproval returns nil, nil when there is no approval for the date
	GetDailyApproval(ctx context.Context, childID string, date time.Time) (*DailyApproval, error)
	SaveDailyApproval(ctx context.Context, approval *DailyApproval) error
	DeleteDailyApproval(ctx context.Context, childID string, date time.Time) error
}

// ApprovalGateService decides whether a child needs a parent's approval to start
// sessions on their own. Only child-initiated sessions are gated: a parent starting
// a session is an approval by itself.
type ApprovalGateService struct {
	requiredDays map[time.Weekday]bool
	storage      ApprovalGateStorage
	timezone     *time.Location
}

// NewApprovalGateService creates the service. requiredDays are lowercase English day names
// (already validated by config).
func NewApprovalGateService(requiredDays []string, storage ApprovalGateStorage, timezone *time.Location) *ApprovalGateService {
	if timezone == nil {
		timezone = time.UTC
	}
	days := make(map[time.Weekday]bool, len(requiredDays))
	for _, name := range requiredDays {
		for wd := time.Sunday; wd <= time.Saturday; wd++ {
			if strings.EqualFold(wd.String(), name) {
				days[wd] = true
			}
		}
	}
	return &ApprovalGateService{requiredDays: days, storage: storage, timezone: timezone}
}

// IsApprovalRequired reports whether approval is needed on the local day of t
func (s *ApprovalGateService) IsApprovalRequired(t time.Time) bool {
	return s.requiredDays[t.In(s.timezone).Weekday()]
}

// localDate returns local midnight for t
func (s *ApprovalGateService) localDate(t time.Time) time.Time {
	l := t.In(s.timezone)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, s.timezone)
}

// GetApprovalStatus returns the gate state of a child at now.
// Storage errors are returned so callers can fail closed.
func (s *ApprovalGateService) GetApprovalStatus(ctx context.Context, childID string, now time.Time) (*ApprovalStatus, error) {
	status := &ApprovalStatus{Required: s.IsApprovalRequired(now)}

	approval, err := s.storage.GetDailyApproval(ctx, childID, s.localDate(now))
	if err != nil {
		return nil, err
	}
	if approval != nil {
		approvedAt := approval.ApprovedAt
		status.ApprovedAt = &approvedAt
		status.ApprovedBy = approval.ApprovedBy
	}
	status.Approved = !status.Required || approval != nil
	return status, nil
}

// Approve allows the child to start sessions for the rest of the local day of now
func (s *ApprovalGateService) Approve(ctx context.Context, childID string, now time.Time, approvedBy string) (*DailyApproval, error) {
	approval := &DailyApproval{
		ChildID:    childID,
		Date:       s.localDate(now),
		ApprovedAt: now,
		ApprovedBy: approvedBy,
	}
	if err := s.storage.SaveDailyApproval(ctx, approval); err != nil {
		return nil, err
	}
	return approval, nil
}

// Revoke removes today's approval. It does not stop sessions that are already running.
func (s *ApprovalGateService) Revoke(ctx context.Context, childID string, now time.Time) error {
	return s.storage.DeleteDailyApproval(ctx, childID, s.localDate(now))
}

// childInitiatedKey marks a request context as coming from the child web app
type childInitiatedKey struct{}

// WithChildInitiated marks ctx as a child-initiated action (subject to the approval gate)
func WithChildInitiated(ctx context.Context) context.Context {
	return context.WithValue(ctx, childInitiatedKey{}, true)
}

// IsChildInitiated reports whether ctx was marked by WithChildInitiated
func IsChildInitiated(ctx context.Context) bool {
	v, _ := ctx.Value(childInitiatedKey{}).(bool)
	return v
}
