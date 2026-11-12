package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	editorialapp "example.com/loja-3d/backend/internal/application/editorial"
	admindomain "example.com/loja-3d/backend/internal/domain/admin"
	editorial "example.com/loja-3d/backend/internal/domain/editorial"
)

type EditorialHandler struct {
	service *editorialapp.Service
	admin   *adminapp.Service
}

func NewEditorialHandler(service *editorialapp.Service, admin *adminapp.Service) *EditorialHandler {
	return &EditorialHandler{service: service, admin: admin}
}

func (h *EditorialHandler) session(r *http.Request) (admindomain.Session, error) {
	cookie, err := r.Cookie("loja3d_admin_session")
	if err != nil {
		return admindomain.Session{}, adminapp.ErrUnauthorized
	}
	return h.admin.Authenticate(r.Context(), cookie.Value)
}
func editorialError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, adminapp.ErrUnauthorized):
		writeError(w, 401, "unauthorized", "Autenticação administrativa necessária.")
	case errors.Is(err, adminapp.ErrForbidden):
		writeError(w, 403, "forbidden", "Acesso negado.")
	case errors.Is(err, editorial.ErrInvalid):
		writeError(w, 400, "invalid_catalog", "Dados editoriais inválidos.")
	case errors.Is(err, editorial.ErrInactiveProduct):
		writeError(w, 409, "inactive_product", "Um ou mais produtos não estão ativos e comercializáveis.")
	case errors.Is(err, editorial.ErrEmpty):
		writeError(w, 409, "empty_catalog", "Adicione ao menos um produto antes de publicar.")
	case errors.Is(err, editorial.ErrConflict):
		writeError(w, 409, "catalog_conflict", "Slug duplicado ou catálogo alterado por outra sessão.")
	case errors.Is(err, editorial.ErrNotFound):
		writeError(w, 404, "catalog_not_found", "Catálogo não encontrado.")
	default:
		writeError(w, 500, "internal_error", "Falha interna.")
	}
}
func editorialDTO(value editorial.Catalog) map[string]any {
	products := make([]productDTO, 0, len(value.Products))
	for _, product := range value.Products {
		products = append(products, toProductDTO(product))
	}
	return map[string]any{"id": value.ID, "slug": value.Slug, "title": value.Title, "description": value.Description, "eyebrow": value.Eyebrow, "edition_label": value.EditionLabel, "cover_image_id": value.CoverImageID, "cover_image_url": value.CoverImageURL, "status": value.Status, "version": value.Version, "published_at": value.PublishedAt, "created_at": value.CreatedAt, "updated_at": value.UpdatedAt, "product_ids": value.ProductIDs, "products": products}
}
func (h *EditorialHandler) PublicList(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListPublished(r.Context())
	if err != nil {
		editorialError(w, err)
		return
	}
	data := make([]map[string]any, 0, len(items))
	for _, item := range items {
		data = append(data, editorialDTO(item))
	}
	writeJSON(w, 200, map[string]any{"data": data})
}
func (h *EditorialHandler) PublicGet(w http.ResponseWriter, r *http.Request) {
	value, err := h.service.GetPublished(r.Context(), r.PathValue("slug"))
	if err != nil {
		editorialError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": editorialDTO(value)})
}
func (h *EditorialHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r)
	if err != nil {
		editorialError(w, err)
		return
	}
	if session.Role != "admin" {
		editorialError(w, adminapp.ErrForbidden)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page == 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 20
	}
	value, err := h.service.ListAdmin(r.Context(), session, r.URL.Query().Get("search"), r.URL.Query().Get("status"), page, limit)
	h.audit(r, session, "catalogs.list", "", err)
	if err != nil {
		editorialError(w, err)
		return
	}
	items := make([]map[string]any, 0, len(value.Items))
	for _, item := range value.Items {
		items = append(items, editorialDTO(item))
	}
	writeJSON(w, 200, map[string]any{"data": map[string]any{"items": items, "meta": map[string]any{"page": value.Page, "limit": value.Limit, "total": value.Total, "total_pages": int((value.Total + int64(value.Limit) - 1) / int64(value.Limit))}}})
}
func (h *EditorialHandler) AdminGet(w http.ResponseWriter, r *http.Request) {
	session, err := h.session(r)
	if err != nil {
		editorialError(w, err)
		return
	}
	if session.Role != "admin" {
		editorialError(w, adminapp.ErrForbidden)
		return
	}
	value, err := h.service.GetAdmin(r.Context(), session, r.PathValue("id"))
	h.audit(r, session, "catalogs.view", r.PathValue("id"), err)
	if err != nil {
		editorialError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": editorialDTO(value)})
}
func decodeEditorialInput(w http.ResponseWriter, r *http.Request) (editorial.Input, error) {
	var body struct {
		Slug            string  `json:"slug"`
		Title           string  `json:"title"`
		Description     string  `json:"description"`
		Eyebrow         string  `json:"eyebrow"`
		EditionLabel    string  `json:"edition_label"`
		CoverImageID    *string `json:"cover_image_id"`
		ExpectedVersion int     `json:"expected_version"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&body); err != nil {
		return editorial.Input{}, editorial.ErrInvalid
	}
	return editorial.Input{Slug: strings.TrimSpace(body.Slug), Title: strings.TrimSpace(body.Title), Description: strings.TrimSpace(body.Description), Eyebrow: strings.TrimSpace(body.Eyebrow), EditionLabel: strings.TrimSpace(body.EditionLabel), CoverImageID: body.CoverImageID, ExpectedVersion: body.ExpectedVersion}, nil
}
func (h *EditorialHandler) mutation(r *http.Request) (admindomain.Session, error) {
	session, err := h.session(r)
	if err != nil {
		return session, err
	}
	if session.Role != "admin" || !adminapp.ConstantHashEqual(r.Header.Get("X-CSRF-Token"), session.CSRFHash) {
		return session, adminapp.ErrForbidden
	}
	return session, nil
}
func (h *EditorialHandler) Create(w http.ResponseWriter, r *http.Request) {
	session, err := h.mutation(r)
	if err != nil {
		editorialError(w, err)
		return
	}
	input, err := decodeEditorialInput(w, r)
	if err == nil {
		var value editorial.Catalog
		value, err = h.service.Create(r.Context(), session, input)
		h.audit(r, session, "catalogs.create", value.ID, err)
		if err == nil {
			writeJSON(w, 201, map[string]any{"data": editorialDTO(value)})
			return
		}
	}
	editorialError(w, err)
}
func (h *EditorialHandler) Update(w http.ResponseWriter, r *http.Request) {
	session, err := h.mutation(r)
	if err != nil {
		editorialError(w, err)
		return
	}
	input, err := decodeEditorialInput(w, r)
	var value editorial.Catalog
	if err == nil {
		value, err = h.service.Update(r.Context(), session, r.PathValue("id"), input)
	}
	h.audit(r, session, "catalogs.update", r.PathValue("id"), err)
	if err != nil {
		editorialError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": editorialDTO(value)})
}
func (h *EditorialHandler) ReplaceProducts(w http.ResponseWriter, r *http.Request) {
	session, err := h.mutation(r)
	if err != nil {
		editorialError(w, err)
		return
	}
	var body struct {
		ProductIDs      []string `json:"product_ids"`
		ExpectedVersion int      `json:"expected_version"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&body) != nil {
		editorialError(w, editorial.ErrInvalid)
		return
	}
	value, err := h.service.ReplaceProducts(r.Context(), session, r.PathValue("id"), body.ProductIDs, body.ExpectedVersion)
	h.audit(r, session, "catalogs.products.reorder", r.PathValue("id"), err)
	if err != nil {
		editorialError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": editorialDTO(value)})
}

func (h *EditorialHandler) AddProduct(w http.ResponseWriter, r *http.Request) {
	session, err := h.mutation(r)
	if err != nil {
		editorialError(w, err)
		return
	}
	var body struct {
		ProductID       string `json:"product_id"`
		ExpectedVersion int    `json:"expected_version"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
		editorialError(w, editorial.ErrInvalid)
		return
	}
	current, err := h.service.GetAdmin(r.Context(), session, r.PathValue("id"))
	if err == nil {
		current.ProductIDs = append(current.ProductIDs, body.ProductID)
		current, err = h.service.ReplaceProducts(r.Context(), session, current.ID, current.ProductIDs, body.ExpectedVersion)
	}
	h.audit(r, session, "catalogs.products.add", r.PathValue("id"), err)
	if err != nil {
		editorialError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": editorialDTO(current)})
}
func (h *EditorialHandler) RemoveProduct(w http.ResponseWriter, r *http.Request) {
	session, err := h.mutation(r)
	if err != nil {
		editorialError(w, err)
		return
	}
	version, parseErr := strconv.Atoi(r.URL.Query().Get("expected_version"))
	if parseErr != nil {
		editorialError(w, editorial.ErrInvalid)
		return
	}
	current, err := h.service.GetAdmin(r.Context(), session, r.PathValue("id"))
	if err == nil {
		ids := make([]string, 0, len(current.ProductIDs))
		for _, id := range current.ProductIDs {
			if id != r.PathValue("productID") {
				ids = append(ids, id)
			}
		}
		if len(ids) == len(current.ProductIDs) {
			err = editorial.ErrNotFound
		} else {
			current, err = h.service.ReplaceProducts(r.Context(), session, current.ID, ids, version)
		}
	}
	h.audit(r, session, "catalogs.products.remove", r.PathValue("id"), err)
	if err != nil {
		editorialError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": editorialDTO(current)})
}
func (h *EditorialHandler) Publish(w http.ResponseWriter, r *http.Request)   { h.status(w, r, true) }
func (h *EditorialHandler) Unpublish(w http.ResponseWriter, r *http.Request) { h.status(w, r, false) }
func (h *EditorialHandler) status(w http.ResponseWriter, r *http.Request, publish bool) {
	session, err := h.mutation(r)
	if err != nil {
		editorialError(w, err)
		return
	}
	var body struct {
		ExpectedVersion int `json:"expected_version"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
		editorialError(w, editorial.ErrInvalid)
		return
	}
	var value editorial.Catalog
	action := "catalogs.unpublish"
	if publish {
		value, err = h.service.Publish(r.Context(), session, r.PathValue("id"), body.ExpectedVersion)
		action = "catalogs.publish"
	} else {
		value, err = h.service.Unpublish(r.Context(), session, r.PathValue("id"), body.ExpectedVersion)
	}
	h.audit(r, session, action, r.PathValue("id"), err)
	if err != nil {
		editorialError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"data": editorialDTO(value)})
}
func (h *EditorialHandler) audit(r *http.Request, session admindomain.Session, action, id string, err error) {
	result := "success"
	if err != nil {
		result = "failure"
	}
	_ = h.admin.Audit(r.Context(), session, action, "editorial_catalog", id, result, adminIP(r))
}
