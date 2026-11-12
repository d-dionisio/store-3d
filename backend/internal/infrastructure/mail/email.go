package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

type Message struct{ From, ReplyTo, To, Subject, HTML, Text, IdempotencyKey string }
type EmailSender interface {
	Send(context.Context, Message) (string, error)
}
type PermanentError struct{ Code string }

func (e PermanentError) Error() string { return e.Code }

type ResendSender struct {
	BaseURL, APIKey string
	Client          *http.Client
}

func (s ResendSender) Send(ctx context.Context, m Message) (string, error) {
	if s.APIKey == "" || s.Client == nil {
		return "", errors.New("resend unavailable")
	}
	body, _ := json.Marshal(map[string]any{"from": m.From, "to": []string{m.To}, "subject": m.Subject, "html": m.HTML, "text": m.Text, "reply_to": m.ReplyTo})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(s.BaseURL, "/")+"/emails", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", m.IdempotencyKey)
	resp, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && result.ID != "" {
		return result.ID, nil
	}
	if resp.StatusCode == http.StatusConflict && result.Name == "concurrent_idempotent_requests" {
		return "", errors.New("resend concurrent idempotent request")
	}
	if resp.StatusCode == 400 || resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 409 || resp.StatusCode == 422 {
		return "", PermanentError{Code: fmt.Sprintf("resend_%d", resp.StatusCode)}
	}
	return "", fmt.Errorf("resend temporary status %d", resp.StatusCode)
}

type TemplateData struct{ Link, Reference, Status, Decision string }
type Rendered struct{ Subject, HTML, Text string }

var subjects = map[string]string{"verify_email": "Confirme seu e-mail", "password_reset": "Redefina sua senha", "magic_link": "Seu acesso à Lumé Studio", "password_changed": "Sua senha foi alterada", "mfa_changed": "Segurança da conta atualizada", "security_alert": "Novo evento de segurança", "order_confirmed": "Pedido confirmado", "order_updated": "Atualização do pedido", "quote_proposal": "Sua proposta está disponível", "quote_decision": "Decisão registrada"}

const htmlShell = `<!doctype html><html><body style="margin:0;background:#EAE5DC;color:#292825;font-family:Arial,sans-serif"><table role="presentation" width="100%"><tr><td align="center" style="padding:32px"><table role="presentation" width="600" style="max-width:100%;background:#FAF9F6"><tr><td style="padding:36px"><p style="letter-spacing:.18em">LUMÉ STUDIO</p><h1>{{.Title}}</h1><p>{{.Body}}</p>{{if .Link}}<p><a href="{{.Link}}" style="background:#292825;color:#FAF9F6;padding:14px 20px;text-decoration:none">Continuar com segurança</a></p><p style="word-break:break-all">{{.Link}}</p>{{end}}<hr><small>Se você não solicitou esta mensagem, ignore-a e revise a segurança da conta.</small></td></tr></table></td></tr></table></body></html>`

func Render(kind string, data TemplateData) (Rendered, error) {
	subject, ok := subjects[kind]
	if !ok {
		return Rendered{}, errors.New("unknown template")
	}
	body := map[string]string{"verify_email": "Confirme seu endereço para ativar a conta.", "password_reset": "Use o link temporário para redefinir sua senha.", "magic_link": "Use o link temporário para acessar sua conta.", "password_changed": "A senha da sua conta foi alterada.", "mfa_changed": "A configuração de autenticação em dois fatores foi atualizada.", "security_alert": "Um evento importante de segurança ocorreu em sua conta.", "order_confirmed": "O pagamento foi confirmado e o pedido está registrado.", "order_updated": "O estado do pedido foi atualizado.", "quote_proposal": "Uma proposta comercial está disponível para consulta.", "quote_decision": "Sua decisão sobre a proposta foi registrada."}[kind]
	if data.Reference != "" {
		body += " Referência: " + data.Reference + "."
	}
	if data.Status != "" {
		body += " Estado: " + data.Status + "."
	}
	if data.Decision != "" {
		body += " Decisão: " + data.Decision + "."
	}
	tpl, err := template.New("mail").Parse(htmlShell)
	if err != nil {
		return Rendered{}, err
	}
	var out bytes.Buffer
	if err = tpl.Execute(&out, map[string]string{"Title": subject, "Body": body, "Link": data.Link}); err != nil {
		return Rendered{}, err
	}
	text := subject + "\n\n" + body
	if data.Link != "" {
		text += "\n\n" + data.Link
	}
	return Rendered{subject, out.String(), text}, nil
}
func ValidateAddress(value string) error {
	a, e := mail.ParseAddress(value)
	if e != nil || a.Address == "" {
		return errors.New("invalid email address")
	}
	return nil
}
func LinkExpiry(kind string) time.Duration {
	if kind == "password_reset" {
		return 20 * time.Minute
	}
	if kind == "verify_email" {
		return 30 * time.Minute
	}
	if kind == "magic_link" {
		return 15 * time.Minute
	}
	return 7 * 24 * time.Hour
}
