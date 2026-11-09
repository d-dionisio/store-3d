package quote

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	domain "example.com/loja-3d/backend/internal/domain/quote"
)

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")
var ErrConflict = errors.New("conflict")

type Upload struct {
	ID, Token, ObjectKey, ContentType, Checksum string
	Size                                        int64
	ExpiresAt                                   time.Time
}
type Repository interface {
	Options(context.Context) ([]domain.Options, []domain.Options, error)
	CreateUpload(context.Context, domain.UploadRequest, string, string, string, time.Time) (string, error)
	ConfirmUpload(context.Context, string, string) error
	GetUpload(context.Context, string, string) (Upload, error)
	Create(context.Context, domain.CreateRequest, string) (domain.Created, error)
	Get(context.Context, string, string) (string, time.Time, error)
}
type Storage interface {
	PresignPut(context.Context, string, string, int64, time.Duration) (string, map[string]string, error)
	Verify(context.Context, string, int64, string, string) error
}
type Service struct {
	repo    Repository
	storage Storage
	bucket  string
}

func New(repo Repository, storage Storage, bucket string) *Service {
	return &Service{repo: repo, storage: storage, bucket: bucket}
}
func token() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw := hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(sum[:]), nil
}
func (s *Service) Options(ctx context.Context) ([]domain.Options, []domain.Options, error) {
	return s.repo.Options(ctx)
}
func (s *Service) Initiate(ctx context.Context, req domain.UploadRequest) (Upload, string, map[string]string, error) {
	ext, err := req.Validate()
	if err != nil {
		return Upload{}, "", nil, err
	}
	owner, hash, err := token()
	if err != nil {
		return Upload{}, "", nil, err
	}
	id, err := s.repo.CreateUpload(ctx, req, ext, hash, s.bucket, time.Now().Add(15*time.Minute))
	if err != nil {
		return Upload{}, "", nil, err
	}
	key := "quotes/pending/" + id + "." + ext
	url, headers, err := s.storage.PresignPut(ctx, key, req.Checksum, req.Size, 15*time.Minute)
	return Upload{ID: id, Token: owner, ObjectKey: key, ContentType: req.ContentType, Checksum: req.Checksum, Size: req.Size, ExpiresAt: time.Now().Add(15 * time.Minute)}, url, headers, err
}
func (s *Service) Confirm(ctx context.Context, id, owner string) error {
	upload, err := s.repo.GetUpload(ctx, id, owner)
	if err != nil {
		return err
	}
	if err = s.storage.Verify(ctx, upload.ObjectKey, upload.Size, upload.Checksum, upload.ContentType); err != nil {
		return err
	}
	return s.repo.ConfirmUpload(ctx, id, owner)
}
func (s *Service) Create(ctx context.Context, req domain.CreateRequest) (domain.Created, error) {
	if err := req.Validate(); err != nil {
		return domain.Created{}, err
	}
	access, hash, err := token()
	if err != nil {
		return domain.Created{}, err
	}
	created, err := s.repo.Create(ctx, req, hash)
	if err != nil {
		return domain.Created{}, err
	}
	created.AccessToken = access
	return created, nil
}
func (s *Service) Get(ctx context.Context, reference, access string) (string, time.Time, error) {
	return s.repo.Get(ctx, reference, access)
}
