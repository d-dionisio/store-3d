package production

import (
	"context"
	"errors"
	"testing"

	admindomain "example.com/loja-3d/backend/internal/domain/admin"
)

func TestProductionAuthorizationBoundaries(t *testing.T) {
	service := New(nil)
	operator := admindomain.Session{UserID: "operator", Role: "production_operator"}
	viewer := admindomain.Session{UserID: "viewer", Role: "quote_viewer"}

	if _, err := service.QueueProductionJob(context.Background(), operator, "job", "queue", 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("operator queued job: %v", err)
	}
	if _, err := service.CreatePrinter(context.Background(), operator, PrinterInput{}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("operator created printer: %v", err)
	}
	if _, err := service.CompleteProductionJob(context.Background(), operator, "job", 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("operator completed job: %v", err)
	}
	if _, err := service.ListProductionQueue(context.Background(), viewer, "", 1, 20); !errors.Is(err, ErrForbidden) {
		t.Fatalf("quote viewer accessed production: %v", err)
	}
}

func TestProductionPaginationValidation(t *testing.T) {
	service := New(nil)
	manager := admindomain.Session{UserID: "manager", Role: "production_manager"}
	if _, err := service.ListProductionQueue(context.Background(), manager, "", 0, 20); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid page accepted: %v", err)
	}
}
