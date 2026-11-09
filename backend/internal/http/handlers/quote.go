package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	app "example.com/loja-3d/backend/internal/application/quote"
	domain "example.com/loja-3d/backend/internal/domain/quote"
)

type QuoteHandler struct {
	service *app.Service
	limiter *ipLimiter
}

func NewQuoteHandler(service *app.Service) *QuoteHandler {
	return &QuoteHandler{service: service, limiter: newIPLimiter(20, time.Minute)}
}
func writeQuoteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func quoteError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalid):
		writeQuoteJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "Dados da solicitação são inválidos."}})
	case errors.Is(err, app.ErrForbidden):
		writeQuoteJSON(w, 403, map[string]any{"error": map[string]string{"code": "forbidden", "message": "Upload inválido ou não autorizado."}})
	default:
		writeQuoteJSON(w, 500, map[string]any{"error": map[string]string{"code": "internal_error", "message": "Não foi possível processar a solicitação."}})
	}
}
func (h *QuoteHandler) Options(w http.ResponseWriter, r *http.Request) {
	m, c, err := h.service.Options(r.Context())
	if err != nil {
		quoteError(w, err)
		return
	}
	materials := make([]map[string]string, 0, len(m))
	for _, item := range m {
		materials = append(materials, map[string]string{"id": item.ID, "code": item.Code, "name": item.Name})
	}
	colors := make([]map[string]string, 0, len(c))
	for _, item := range c {
		colors = append(colors, map[string]string{"id": item.ID, "slug": item.Slug, "name": item.Name, "hex_code": item.HexCode})
	}
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]any{"materials": materials, "colors": colors}})
}

type uploadDTO struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Checksum    string `json:"checksum_sha256"`
	Size        int64  `json:"size_bytes"`
}

func (h *QuoteHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.Allow(clientIP(r)) {
		writeQuoteJSON(w, 429, map[string]any{"error": map[string]string{"code": "rate_limited", "message": "Muitas tentativas. Aguarde e tente novamente."}})
		return
	}
	var d uploadDTO
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&d) != nil {
		quoteError(w, domain.ErrInvalid)
		return
	}
	u, url, headers, err := h.service.Initiate(r.Context(), domain.UploadRequest{Filename: d.Filename, ContentType: d.ContentType, Checksum: strings.ToLower(d.Checksum), Size: d.Size})
	if err != nil {
		quoteError(w, err)
		return
	}
	writeQuoteJSON(w, 201, map[string]any{"data": map[string]any{"upload_id": u.ID, "upload_token": u.Token, "upload_url": url, "headers": headers, "expires_at": u.ExpiresAt}})
}

type confirmDTO struct {
	Token string `json:"upload_token"`
}

func (h *QuoteHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.Allow(clientIP(r)) {
		writeQuoteJSON(w, http.StatusTooManyRequests, map[string]any{"error": map[string]string{"code": "rate_limited", "message": "Muitas tentativas."}})
		return
	}
	var d confirmDTO
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&d) != nil {
		quoteError(w, domain.ErrInvalid)
		return
	}
	if err := h.service.Confirm(r.Context(), r.PathValue("id"), d.Token); err != nil {
		quoteError(w, err)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]string{"status": "confirmed", "security_status": "pending"}})
}

type createQuoteDTO struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	MaterialID  string `json:"material_id"`
	ColorID     string `json:"color_id"`
	Quantity    int    `json:"quantity"`
	WidthMM     *int   `json:"width_mm"`
	HeightMM    *int   `json:"height_mm"`
	DepthMM     *int   `json:"depth_mm"`
	Notes       string `json:"notes"`
	UploadID    string `json:"upload_id"`
	UploadToken string `json:"upload_token"`
}

func (h *QuoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.limiter.Allow(clientIP(r)) {
		writeQuoteJSON(w, 429, map[string]any{"error": map[string]string{"code": "rate_limited", "message": "Muitas tentativas."}})
		return
	}
	var d createQuoteDTO
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384)).Decode(&d) != nil {
		quoteError(w, domain.ErrInvalid)
		return
	}
	created, err := h.service.Create(r.Context(), domain.CreateRequest{Name: d.Name, Email: d.Email, Phone: d.Phone, MaterialID: d.MaterialID, ColorID: d.ColorID, Quantity: d.Quantity, WidthMM: d.WidthMM, HeightMM: d.HeightMM, DepthMM: d.DepthMM, Notes: d.Notes, UploadID: d.UploadID, UploadToken: d.UploadToken})
	if err != nil {
		quoteError(w, err)
		return
	}
	writeQuoteJSON(w, 201, map[string]any{"data": map[string]string{"reference": created.Reference, "access_token": created.AccessToken, "status": "submitted"}})
}

func (h *QuoteHandler) Get(w http.ResponseWriter, r *http.Request) {
	access := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if access == "" {
		quoteError(w, app.ErrForbidden)
		return
	}
	status, submitted, err := h.service.Get(r.Context(), r.PathValue("reference"), access)
	if errors.Is(err, app.ErrNotFound) {
		writeQuoteJSON(w, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "not_found", "message": "Solicitação não encontrada."}})
		return
	}
	if err != nil {
		quoteError(w, err)
		return
	}
	writeQuoteJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"reference": r.PathValue("reference"), "status": status, "submitted_at": submitted}})
}

type ipLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string][]time.Time
}

func newIPLimiter(limit int, window time.Duration) *ipLimiter {
	return &ipLimiter{limit: limit, window: window, entries: map[string][]time.Time{}}
}
func (l *ipLimiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cut := now.Add(-l.window)
	kept := l.entries[ip][:0]
	for _, t := range l.entries[ip] {
		if t.After(cut) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= l.limit {
		l.entries[ip] = kept
		return false
	}
	l.entries[ip] = append(kept, now)
	return true
}
func clientIP(r *http.Request) string {
	return r.RemoteAddr
}
