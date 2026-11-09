package quote

import (
	"errors"
	"net/mail"
	"path/filepath"
	"strings"
)

const MaxFileSize int64 = 200 * 1024 * 1024

var ErrInvalid = errors.New("invalid quote data")

type Options struct{ ID, Code, Slug, Name, HexCode string }
type UploadRequest struct {
	Filename, ContentType, Checksum string
	Size                            int64
}
type CreateRequest struct {
	Name, Email, Phone, MaterialID, ColorID, Notes string
	Quantity                                       int
	WidthMM, HeightMM, DepthMM                     *int
	UploadID, UploadToken                          string
}
type Created struct{ Reference, AccessToken string }

func (r UploadRequest) Validate() (string, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(r.Filename), "."))
	validType := (ext == "stl" && r.ContentType == "model/stl") || (ext == "3mf" && r.ContentType == "application/vnd.ms-package.3dmanufacturing-3dmodel+xml")
	if !validType || r.Size <= 0 || r.Size > MaxFileSize || len(r.Checksum) != 64 {
		return "", ErrInvalid
	}
	for _, c := range r.Checksum {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return "", ErrInvalid
		}
	}
	return ext, nil
}
func (r CreateRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" || len(r.Name) > 150 || r.Quantity < 1 || r.Quantity > 1000 || r.MaterialID == "" || r.ColorID == "" || r.UploadID == "" || r.UploadToken == "" {
		return ErrInvalid
	}
	if _, err := mail.ParseAddress(r.Email); err != nil {
		return ErrInvalid
	}
	if len(r.Notes) > 5000 || len(r.Phone) > 30 {
		return ErrInvalid
	}
	for _, v := range []*int{r.WidthMM, r.HeightMM, r.DepthMM} {
		if v != nil && (*v < 1 || *v > 10000) {
			return ErrInvalid
		}
	}
	return nil
}
