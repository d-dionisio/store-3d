package quote

import (
	"context"
	"errors"
	domain "example.com/loja-3d/backend/internal/domain/quote"
	"testing"
	"time"
)

type repoStub struct {
	upload                Upload
	confirmErr, createErr error
}

func (r *repoStub) Options(context.Context) ([]domain.Options, []domain.Options, error) {
	return nil, nil, nil
}
func (r *repoStub) CreateUpload(context.Context, domain.UploadRequest, string, string, string, time.Time) (string, error) {
	return "upload-id", nil
}
func (r *repoStub) GetUpload(context.Context, string, string) (Upload, error) {
	return r.upload, r.confirmErr
}
func (r *repoStub) ConfirmUpload(context.Context, string, string) error { return r.confirmErr }
func (r *repoStub) Create(context.Context, domain.CreateRequest, string) (domain.Created, error) {
	return domain.Created{Reference: "ORC-1"}, r.createErr
}
func (r *repoStub) Get(context.Context, string, string) (string, time.Time, error) {
	return "submitted", time.Now(), nil
}

type storageStub struct{ verifyErr error }

func (storageStub) PresignPut(context.Context, string, string, int64, time.Duration) (string, map[string]string, error) {
	return "https://storage.invalid", map[string]string{}, nil
}
func (s storageStub) Verify(context.Context, string, int64, string, string) error { return s.verifyErr }
func TestConfirmDoesNotConfirmWhenStorageFails(t *testing.T) {
	repo := &repoStub{upload: Upload{ObjectKey: "key", Size: 1, Checksum: "sum", ContentType: "model/stl"}}
	service := New(repo, storageStub{verifyErr: errors.New("storage failed")}, "bucket")
	if err := service.Confirm(context.Background(), "id", "token"); err == nil {
		t.Fatal("expected storage failure")
	}
}
func TestCreateRejectsInvalidAndPropagatesRepositoryFailure(t *testing.T) {
	repo := &repoStub{createErr: errors.New("database failed")}
	service := New(repo, storageStub{}, "bucket")
	valid := domain.CreateRequest{Name: "A", Email: "a@example.com", MaterialID: "m", ColorID: "c", Quantity: 1, UploadID: "u", UploadToken: "t"}
	if _, err := service.Create(context.Background(), valid); err == nil {
		t.Fatal("expected database failure")
	}
	valid.Quantity = 0
	if _, err := service.Create(context.Background(), valid); !errors.Is(err, domain.ErrInvalid) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
