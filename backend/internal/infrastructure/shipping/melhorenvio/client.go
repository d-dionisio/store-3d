package melhorenvio

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	domain "example.com/loja-3d/backend/internal/domain/shipping"
)

type Client struct {
	baseURL, token, userAgent string
	http                      *http.Client
}
type Unavailable struct{}

func (Unavailable) Calculate(context.Context, domain.QuoteRequest) ([]domain.ProviderQuote, error) {
	return nil, errors.New("Melhor Envio is not configured")
}

func New(baseURL, token, userAgent string, client *http.Client) (*Client, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	u, e := url.Parse(baseURL)
	if e != nil || u.Scheme == "" || u.Host == "" || token == "" || strings.TrimSpace(userAgent) == "" || client == nil {
		return nil, errors.New("invalid Melhor Envio configuration")
	}
	return &Client{baseURL: baseURL, token: token, userAgent: userAgent, http: client}, nil
}

type volume struct {
	Width     float64 `json:"width"`
	Height    float64 `json:"height"`
	Length    float64 `json:"length"`
	Weight    float64 `json:"weight"`
	Insurance float64 `json:"insurance,omitempty"`
}
type calculateRequest struct {
	From struct {
		PostalCode string `json:"postal_code"`
	} `json:"from"`
	To struct {
		PostalCode string `json:"postal_code"`
	} `json:"to"`
	Volumes []volume `json:"volumes"`
	Options struct {
		Receipt bool `json:"receipt"`
		OwnHand bool `json:"own_hand"`
	} `json:"options"`
}
type calculateResponse struct {
	ID                  int                    `json:"id"`
	Name                string                 `json:"name"`
	Price               string                 `json:"price"`
	CustomPrice         string                 `json:"custom_price"`
	DeliveryTime        int                    `json:"delivery_time"`
	CustomDeliveryTime  int                    `json:"custom_delivery_time"`
	DeliveryRange       struct{ Min, Max int } `json:"delivery_range"`
	CustomDeliveryRange struct{ Min, Max int } `json:"custom_delivery_range"`
	Company             struct {
		Name string `json:"name"`
	} `json:"company"`
	Packages json.RawMessage `json:"packages"`
	Error    string          `json:"error"`
}

func (c *Client) Calculate(ctx context.Context, input domain.QuoteRequest) ([]domain.ProviderQuote, error) {
	var body calculateRequest
	body.From.PostalCode = input.OriginPostalCode
	body.To.PostalCode = input.DestinationPostalCode
	volumeCount := 0
	for _, p := range input.Packages {
		volumeCount += p.Quantity
	}
	insurance := 0.0
	if volumeCount > 0 {
		insurance = float64(input.InsuranceValueCents) / 100 / float64(volumeCount)
	}
	for _, p := range input.Packages {
		for i := 0; i < p.Quantity; i++ {
			body.Volumes = append(body.Volumes, volume{Width: float64(p.WidthMM) / 10, Height: float64(p.HeightMM) / 10, Length: float64(p.LengthMM) / 10, Weight: float64(p.WeightGrams) / 1000, Insurance: insurance})
		}
	}
	payload, _ := json.Marshal(body)
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v2/me/shipment/calculate", bytes.NewReader(payload))
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	res, e := c.http.Do(req)
	if e != nil {
		return nil, e
	}
	defer res.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if e != nil {
		return nil, e
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("provider status %d", res.StatusCode)
	}
	var decoded []calculateResponse
	if json.Unmarshal(raw, &decoded) != nil {
		return nil, errors.New("invalid provider response")
	}
	out := make([]domain.ProviderQuote, 0, len(decoded))
	for _, q := range decoded {
		if q.Error != "" {
			continue
		}
		price := q.CustomPrice
		if price == "" {
			price = q.Price
		}
		amount, e := money(price)
		if e != nil || q.ID <= 0 || q.Name == "" || q.Company.Name == "" {
			continue
		}
		min, max := q.CustomDeliveryRange.Min, q.CustomDeliveryRange.Max
		if max == 0 {
			min, max = q.DeliveryRange.Min, q.DeliveryRange.Max
		}
		if max == 0 {
			max = q.CustomDeliveryTime
			min = max
		}
		if max == 0 {
			max = q.DeliveryTime
			min = max
		}
		if min < 0 || max < min {
			continue
		}
		var snapshot any
		_ = json.Unmarshal(q.Packages, &snapshot)
		out = append(out, domain.ProviderQuote{ServiceCode: strconv.Itoa(q.ID), CarrierName: q.Company.Name, ServiceName: q.Name, PriceCents: amount, MinDays: min, MaxDays: max, PackageSnapshot: snapshot})
	}
	return out, nil
}

func money(v string) (int64, error) {
	f, e := strconv.ParseFloat(strings.ReplaceAll(v, ",", "."), 64)
	if e != nil || f < 0 {
		return 0, errors.New("invalid money")
	}
	return int64(f*100 + 0.5), nil
}
