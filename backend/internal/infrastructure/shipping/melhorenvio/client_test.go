package melhorenvio

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domain "example.com/loja-3d/backend/internal/domain/shipping"
)

func request() domain.QuoteRequest {
	return domain.QuoteRequest{OriginPostalCode: "01001000", DestinationPostalCode: "30110000", InsuranceValueCents: 10000, Packages: []domain.Package{{Quantity: 1, WeightGrams: 500, LengthMM: 200, WidthMM: 150, HeightMM: 100}}}
}

func TestCalculate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/me/shipment/calculate" || r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("User-Agent") != "Loja 3D (dev@example.com)" {
			t.Fatalf("unexpected request")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":1,"name":"PAC","custom_price":"25.90","custom_delivery_time":5,"company":{"name":"Correios"},"packages":[]}]`))
	}))
	defer srv.Close()
	c, e := New(srv.URL, "secret", "Loja 3D (dev@example.com)", srv.Client())
	if e != nil {
		t.Fatal(e)
	}
	quotes, e := c.Calculate(context.Background(), request())
	if e != nil || len(quotes) != 1 {
		t.Fatalf("quotes=%v err=%v", quotes, e)
	}
	if quotes[0].PriceCents != 2590 || quotes[0].MaxDays != 5 || quotes[0].ServiceCode != "1" {
		t.Fatalf("unexpected quote: %+v", quotes[0])
	}
}

func TestCalculateProviderUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "down", http.StatusServiceUnavailable) }))
	defer srv.Close()
	c, _ := New(srv.URL, "secret", "app (dev@example.com)", srv.Client())
	if _, e := c.Calculate(context.Background(), request()); e == nil {
		t.Fatal("expected error")
	}
}

func TestCalculateTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(50 * time.Millisecond) }))
	defer srv.Close()
	client := srv.Client()
	client.Timeout = 5 * time.Millisecond
	c, _ := New(srv.URL, "secret", "app (dev@example.com)", client)
	if _, e := c.Calculate(context.Background(), request()); e == nil {
		t.Fatal("expected timeout")
	}
}

func TestCalculateSkipsServiceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":1,"name":"PAC","error":"dimensões inválidas","company":{"name":"Correios"}}]`))
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "secret", "app (dev@example.com)", srv.Client())
	quotes, e := c.Calculate(context.Background(), request())
	if e != nil || len(quotes) != 0 {
		t.Fatalf("quotes=%v err=%v", quotes, e)
	}
}
