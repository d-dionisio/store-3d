package handlers

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	customerapp "example.com/loja-3d/backend/internal/application/customer"
	identityapp "example.com/loja-3d/backend/internal/application/identity"
	qrcode "github.com/skip2/go-qrcode"
)

func loginPayload(result identityapp.LoginResult) map[string]any {
	value := map[string]any{"action": result.Action, "challenge_token": result.ChallengeToken}
	if result.SetupURI != "" {
		value["setup_uri"] = result.SetupURI
		value["setup_secret"] = result.SetupSecret
		if png, e := qrcode.Encode(result.SetupURI, qrcode.Medium, 256); e == nil {
			value["qr_code_data_url"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
		}
	}
	return value
}

func identityError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identityapp.ErrUnauthorized), errors.Is(err, identityapp.ErrExpired):
		writeQuoteJSON(w, 401, map[string]any{"error": map[string]string{"code": "unauthorized", "message": "Credenciais ou desafio inválidos."}})
	case errors.Is(err, identityapp.ErrInvalid):
		writeQuoteJSON(w, 400, map[string]any{"error": map[string]string{"code": "invalid_request", "message": "Dados de autenticação inválidos."}})
	case errors.Is(err, identityapp.ErrConflict):
		writeQuoteJSON(w, 409, map[string]any{"error": map[string]string{"code": "conflict", "message": "Não foi possível concluir a solicitação."}})
	case errors.Is(err, identityapp.ErrRateLimited):
		writeQuoteJSON(w, 429, map[string]any{"error": map[string]string{"code": "rate_limited", "message": "Muitas tentativas. Aguarde."}})
	case errors.Is(err, identityapp.ErrUnavailable):
		writeQuoteJSON(w, 503, map[string]any{"error": map[string]string{"code": "email_unavailable", "message": "Envio transacional indisponível."}})
	default:
		writeQuoteJSON(w, 500, map[string]any{"error": map[string]string{"code": "internal_error", "message": "Falha interna."}})
	}
}
func decodeIdentity(w http.ResponseWriter, r *http.Request, value any) bool {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(value) == nil
}
func (h *AdminHandler) identityLogin(w http.ResponseWriter, r *http.Request) {
	if h.identity == nil {
		identityError(w, identityapp.ErrUnavailable)
		return
	}
	if !h.limiter.Allow(adminIP(r)) {
		identityError(w, identityapp.ErrRateLimited)
		return
	}
	var b struct{ Email, Password string }
	if !decodeIdentity(w, r, &b) {
		identityError(w, identityapp.ErrInvalid)
		return
	}
	result, e := h.identity.StartLogin(r.Context(), "admin", b.Email, b.Password, adminIP(r), r.UserAgent())
	if e != nil {
		identityError(w, e)
		return
	}
	writeQuoteJSON(w, 202, map[string]any{"data": loginPayload(result)})
}
func (h *AdminHandler) CompleteIdentityMFA(w http.ResponseWriter, r *http.Request) {
	if h.identity == nil {
		identityError(w, identityapp.ErrUnavailable)
		return
	}
	if !h.limiter.Allow(adminIP(r)) {
		identityError(w, identityapp.ErrRateLimited)
		return
	}
	var b struct {
		ChallengeToken string `json:"challenge_token"`
		Code           string `json:"code"`
		RecoveryCode   string `json:"recovery_code"`
	}
	if !decodeIdentity(w, r, &b) {
		identityError(w, identityapp.ErrInvalid)
		return
	}
	result, e := h.identity.CompleteMFA(r.Context(), b.ChallengeToken, b.Code, b.RecoveryCode, adminIP(r), r.UserAgent())
	if e != nil {
		identityError(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "loja3d_admin_session", Value: result.SessionToken, Path: "/", HttpOnly: true, Secure: h.secureCookies, SameSite: http.SameSiteStrictMode, MaxAge: int(time.Until(result.Session.ExpiresAt).Seconds())})
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]any{"csrf_token": result.CSRFToken, "recovery_codes": result.RecoveryCodes, "user": map[string]string{"email": result.Session.Email, "name": result.Session.FullName, "role": result.Session.Role}}})
}
func (h *AdminHandler) SecurityStatus(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		adminError(w, e)
		return
	}
	enabled, codes, e := h.identity.Status(r.Context(), "admin", s.UserID)
	if e != nil {
		identityError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]any{"mfa_enabled": enabled, "recovery_codes_remaining": codes, "authentication_level": s.AuthenticationLevel}})
}
func (h *AdminHandler) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	s, e := h.mutationSession(r)
	if e != nil {
		adminError(w, e)
		return
	}
	codes, e := h.identity.RegenerateRecoveryCodes(r.Context(), "admin", s.UserID)
	if e != nil {
		identityError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]any{"recovery_codes": codes}})
}

func (h *CustomerHandler) Register(w http.ResponseWriter, r *http.Request) {
	if h.identity == nil {
		identityError(w, identityapp.ErrUnavailable)
		return
	}
	if !h.limiter.Allow(adminIP(r)) {
		writeQuoteJSON(w, 202, map[string]any{"data": map[string]string{"message": "Se os dados forem válidos, enviaremos instruções."}})
		return
	}
	var b struct {
		FullName        string `json:"full_name"`
		Email, Password string
	}
	if !decodeIdentity(w, r, &b) {
		identityError(w, identityapp.ErrInvalid)
		return
	}
	e := h.identity.Register(r.Context(), b.FullName, b.Email, b.Password, adminIP(r))
	if e != nil && !errors.Is(e, identityapp.ErrUnavailable) && !errors.Is(e, identityapp.ErrConflict) {
		identityError(w, e)
		return
	}
	writeQuoteJSON(w, 202, map[string]any{"data": map[string]string{"message": "Se os dados forem válidos, enviaremos instruções de verificação."}})
}
func (h *CustomerHandler) PasswordLogin(w http.ResponseWriter, r *http.Request) {
	if h.identity == nil {
		identityError(w, identityapp.ErrUnavailable)
		return
	}
	if !h.limiter.Allow(adminIP(r)) {
		identityError(w, identityapp.ErrRateLimited)
		return
	}
	var b struct{ Email, Password string }
	if !decodeIdentity(w, r, &b) {
		identityError(w, identityapp.ErrInvalid)
		return
	}
	result, e := h.identity.StartLogin(r.Context(), "customer", b.Email, b.Password, adminIP(r), r.UserAgent())
	if e != nil {
		identityError(w, e)
		return
	}
	if result.Action == "authenticated" {
		http.SetCookie(w, &http.Cookie{Name: "loja3d_customer_session", Value: result.SessionToken, Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(time.Until(result.Session.ExpiresAt).Seconds())})
		writeQuoteJSON(w, 200, map[string]any{"data": map[string]any{"action": result.Action, "csrf_token": result.CSRFToken}})
		return
	}
	writeQuoteJSON(w, 202, map[string]any{"data": loginPayload(result)})
}
func (h *CustomerHandler) CompleteIdentityMFA(w http.ResponseWriter, r *http.Request) {
	if h.identity == nil {
		identityError(w, identityapp.ErrUnavailable)
		return
	}
	if !h.limiter.Allow(adminIP(r)) {
		identityError(w, identityapp.ErrRateLimited)
		return
	}
	var b struct {
		ChallengeToken string `json:"challenge_token"`
		Code           string `json:"code"`
		RecoveryCode   string `json:"recovery_code"`
	}
	if !decodeIdentity(w, r, &b) {
		identityError(w, identityapp.ErrInvalid)
		return
	}
	result, e := h.identity.CompleteMFA(r.Context(), b.ChallengeToken, b.Code, b.RecoveryCode, adminIP(r), r.UserAgent())
	if e != nil {
		identityError(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "loja3d_customer_session", Value: result.SessionToken, Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode, MaxAge: int(time.Until(result.Session.ExpiresAt).Seconds())})
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]any{"csrf_token": result.CSRFToken, "recovery_codes": result.RecoveryCodes}})
}
func (h *CustomerHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var b struct{ Token string }
	if !decodeIdentity(w, r, &b) {
		identityError(w, identityapp.ErrInvalid)
		return
	}
	if e := h.identity.VerifyEmail(r.Context(), b.Token, adminIP(r)); e != nil {
		identityError(w, e)
		return
	}
	w.WriteHeader(204)
}
func (h *CustomerHandler) ResendVerification(w http.ResponseWriter, r *http.Request) {
	var b struct{ Email string }
	if decodeIdentity(w, r, &b) {
		_ = h.identity.ResendVerification(r.Context(), b.Email, adminIP(r))
	}
	writeQuoteJSON(w, 202, map[string]any{"data": map[string]string{"message": "Se aplicável, enviaremos novas instruções."}})
}
func (h *CustomerHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var b struct{ Email string }
	if decodeIdentity(w, r, &b) {
		_ = h.identity.RequestReset(r.Context(), b.Email, adminIP(r))
	}
	writeQuoteJSON(w, 202, map[string]any{"data": map[string]string{"message": "Se o endereço estiver cadastrado, enviaremos instruções."}})
}
func (h *CustomerHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var b struct{ Token, Password string }
	if !decodeIdentity(w, r, &b) {
		identityError(w, identityapp.ErrInvalid)
		return
	}
	if e := h.identity.ResetPassword(r.Context(), b.Token, b.Password, adminIP(r)); e != nil {
		identityError(w, e)
		return
	}
	w.WriteHeader(204)
}
func (h *CustomerHandler) BeginMFA(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if !customerapp.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, customerapp.ErrForbidden)
		return
	}
	result, e := h.identity.BeginSetup(r.Context(), "customer", s.UserID, s.Email, adminIP(r), r.UserAgent())
	if e != nil {
		identityError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": loginPayload(result)})
}
func (h *CustomerHandler) SecurityStatus(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	enabled, codes, e := h.identity.Status(r.Context(), "customer", s.UserID)
	if e != nil {
		identityError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]any{"mfa_enabled": enabled, "recovery_codes_remaining": codes, "authentication_level": s.AuthenticationLevel}})
}
func (h *CustomerHandler) DisableMFA(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if s.AuthenticationLevel != "mfa" || !customerapp.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, customerapp.ErrForbidden)
		return
	}
	if e = h.identity.Disable(r.Context(), "customer", s.UserID); e != nil {
		identityError(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "loja3d_customer_session", Value: "", Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(204)
}
func (h *CustomerHandler) SetPassword(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if !customerapp.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, customerapp.ErrForbidden)
		return
	}
	var b struct{ Password string }
	if !decodeIdentity(w, r, &b) {
		identityError(w, identityapp.ErrInvalid)
		return
	}
	if e = h.identity.SetPassword(r.Context(), "customer", s.UserID, b.Password); e != nil {
		identityError(w, e)
		return
	}
	w.WriteHeader(204)
}
func (h *CustomerHandler) RegenerateRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if s.AuthenticationLevel != "mfa" || !customerapp.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, customerapp.ErrForbidden)
		return
	}
	codes, e := h.identity.RegenerateRecoveryCodes(r.Context(), "customer", s.UserID)
	if e != nil {
		identityError(w, e)
		return
	}
	writeQuoteJSON(w, 200, map[string]any{"data": map[string]any{"recovery_codes": codes}})
}
func (h *CustomerHandler) RevokeAllSessions(w http.ResponseWriter, r *http.Request) {
	s, e := h.session(r)
	if e != nil {
		customerError(w, e)
		return
	}
	if !customerapp.CSRF(r.Header.Get("X-CSRF-Token"), s.CSRFHash) {
		customerError(w, customerapp.ErrForbidden)
		return
	}
	if e = h.identity.RevokeSessions(r.Context(), "customer", s.UserID); e != nil {
		identityError(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "loja3d_customer_session", Value: "", Path: "/", HttpOnly: true, Secure: h.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.WriteHeader(204)
}
