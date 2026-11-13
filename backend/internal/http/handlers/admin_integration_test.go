package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	adminapp "example.com/loja-3d/backend/internal/application/admin"
	apihttp "example.com/loja-3d/backend/internal/http"
	"example.com/loja-3d/backend/internal/http/handlers"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

type readyStub struct{}

func (readyStub) Check(context.Context) error { return nil }
func TestAdminHTTPAuthorizationIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	hash, _ := bcrypt.GenerateFromPassword([]byte("HTTP-Integration-2026!"), bcrypt.MinCost)
	emailAdmin := "http-admin-" + time.Now().Format("150405000000000") + "@example.com"
	var userID string
	err = pool.QueryRow(context.Background(), `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,'HTTP Admin',$2,'admin') RETURNING id`, emailAdmin, string(hash)).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), `DELETE FROM admin_users WHERE id=$1`, userID)
	admin := handlers.NewAdminHandler(adminapp.New(postgres.NewAdminRepository(pool)), false)
	router := apihttp.NewHandlerWithAdmin("http://localhost:3000", readyStub{}, nil, nil, admin)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status=%d", response.Code)
	}
	invalidLogin := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session", bytes.NewBufferString(`{"email":"`+emailAdmin+`","password":"wrong-password"}`))
	invalidLoginResponse := httptest.NewRecorder()
	router.ServeHTTP(invalidLoginResponse, invalidLogin)
	if invalidLoginResponse.Code != http.StatusUnauthorized {
		t.Fatalf("invalid login status=%d", invalidLoginResponse.Code)
	}
	login := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session", bytes.NewBufferString(`{"email":"`+emailAdmin+`","password":"HTTP-Integration-2026!"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	router.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginResponse.Code, loginResponse.Body.String())
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatal("secure session cookie missing")
	}
	authorized := httptest.NewRequest(http.MethodGet, "/api/v1/admin/dashboard", nil)
	authorized.AddCookie(cookies[0])
	authorizedResponse := httptest.NewRecorder()
	router.ServeHTTP(authorizedResponse, authorized)
	if authorizedResponse.Code != http.StatusOK {
		t.Fatalf("authorized status=%d body=%s", authorizedResponse.Code, authorizedResponse.Body.String())
	}
	var loginBody struct {
		Data struct {
			CSRFToken string `json:"csrf_token"`
		} `json:"data"`
	}
	if err = json.Unmarshal(loginResponse.Body.Bytes(), &loginBody); err != nil || loginBody.Data.CSRFToken == "" {
		t.Fatalf("missing csrf: %v", err)
	}
	email := "http-operator-" + time.Now().Format("150405000000000") + "@example.com"
	body := []byte(`{"email":"` + email + `","full_name":"HTTP Operator","password":"HTTP-Operator-2026!","role":"quote_viewer"}`)
	withoutCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(body))
	withoutCSRF.AddCookie(cookies[0])
	withoutCSRFResponse := httptest.NewRecorder()
	router.ServeHTTP(withoutCSRFResponse, withoutCSRF)
	if withoutCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("missing csrf status=%d", withoutCSRFResponse.Code)
	}
	create := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(body))
	create.AddCookie(cookies[0])
	create.Header.Set("X-CSRF-Token", loginBody.Data.CSRFToken)
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, create)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create user status=%d body=%s", createResponse.Code, createResponse.Body.String())
	}
	defer pool.Exec(context.Background(), `DELETE FROM admin_users WHERE lower(email)=lower($1)`, email)
}
