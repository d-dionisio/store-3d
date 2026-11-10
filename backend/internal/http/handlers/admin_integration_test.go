package handlers_test

import (
	"bytes"
	"context"
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
	var userID string
	err = pool.QueryRow(context.Background(), `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES('http-admin@example.com','HTTP Admin',$1,'admin') ON CONFLICT((lower(email))) DO UPDATE SET password_hash=excluded.password_hash,status='active' RETURNING id`, string(hash)).Scan(&userID)
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
	login := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session", bytes.NewBufferString(`{"email":"http-admin@example.com","password":"HTTP-Integration-2026!"}`))
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
}
