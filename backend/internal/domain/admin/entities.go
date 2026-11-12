package admin

import "time"

type User struct {
	ID, Email, FullName, PasswordHash, Role, Status string
	LockedUntil                                     *time.Time
}
type InternalUser struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	FullName    string     `json:"full_name"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
type Session struct {
	ID, UserID, Email, FullName, Role, CSRFHash string
	ExpiresAt                                   time.Time
}
type Stats struct{ Total, Waiting, Reviewing, Quoted, Approved, Rejected int64 }
type QuoteSummary struct {
	Reference, CustomerName, MaterialName, Status, SecurityStatus string
	Quantity                                                      int
	CreatedAt                                                     time.Time
}
type QuotePage struct {
	Items       []QuoteSummary
	Page, Limit int
	Total       int64
}
type QuoteFile struct {
	ID, Filename, Extension, SecurityStatus string
	SizeBytes                               int64
	CreatedAt                               time.Time
}
type HistoryEvent struct {
	Kind, Status string
	At           time.Time
}
type QuoteDetail struct {
	Reference, CustomerName, CustomerEmail, CustomerPhone, MaterialName, ColorName, Status, Notes string
	Quantity                                                                                      int
	WidthMM, HeightMM, DepthMM                                                                    *int
	CreatedAt                                                                                     time.Time
	Files                                                                                         []QuoteFile
	History                                                                                       []HistoryEvent
	Proposals                                                                                     []Proposal
}
