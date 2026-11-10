package payment

import "time"

type CreateRequest struct {
	AttemptID, IdempotencyKey, OrderReference, PayerEmail string
	AmountCents                                           int64
	ExpiresAt                                             time.Time
}
type ProviderPayment struct {
	ID, Status, StatusDetail, Currency, ExternalReference, QRCode, QRCodeBase64, TicketURL string
	AmountCents, RefundedCents                                                             int64
	ExpiresAt                                                                              *time.Time
	ApprovedAt, UpdatedAt                                                                  *time.Time
}
type PIX struct {
	AttemptID, Status, QRCode, QRCodeBase64, TicketURL string `json:",omitempty"`
	AmountCents                                        int64
	ExpiresAt                                          *time.Time
}
type Status struct {
	AttemptID, OrderReference, Method, Status         string
	AmountCents, PaidAmountCents, RefundedAmountCents int64
	ExpiresAt                                         *time.Time
	PaidAt                                            *time.Time
}
type Webhook struct {
	EventID, EventType, ExternalPaymentID, RequestID, Signature string
	Payload                                                     []byte
}
