package admin

import (
	"context"
	"testing"

	domain "example.com/loja-3d/backend/internal/domain/admin"
)

type auditRepository struct {
	Repository
	action, outcome   string
	userID, sessionID *string
}

func (r *auditRepository) Audit(_ context.Context, userID, sessionID *string, action, _, _, outcome, _ string) error {
	r.userID, r.sessionID, r.action, r.outcome = userID, sessionID, action, outcome
	return nil
}

func TestAuditProductionCostSecurityAllowsOnlyBoundedEvents(t *testing.T) {
	repo := &auditRepository{}
	service := New(repo)
	session := domain.Session{ID: "session-id", UserID: "user-id"}
	if err := service.AuditProductionCostSecurity(context.Background(), &session, "production.cost.security.csrf_missing", "denied", "192.0.2.1"); err != nil {
		t.Fatalf("allowed security event rejected: %v", err)
	}
	if repo.action != "production.cost.security.csrf_missing" || repo.outcome != "denied" || repo.userID == nil || *repo.userID != session.UserID || repo.sessionID == nil || *repo.sessionID != session.ID {
		t.Fatalf("unexpected persisted audit context: %+v", repo)
	}
	for _, action := range []string{"production.cost.security." + string(make([]byte, 128)), "production.cost.security.password=" + "secret", "quotes.read"} {
		if err := service.AuditProductionCostSecurity(context.Background(), &session, action, "denied", ""); err == nil {
			t.Errorf("unbounded or unapproved action accepted: %q", action)
		}
	}
	if err := service.AuditProductionCostSecurity(context.Background(), nil, "production.cost.security.session_denied", "denied", ""); err != nil {
		t.Fatalf("anonymous rejection event failed: %v", err)
	}
	if repo.userID != nil || repo.sessionID != nil {
		t.Fatalf("anonymous event unexpectedly has actor context: user=%v session=%v", repo.userID, repo.sessionID)
	}
}
