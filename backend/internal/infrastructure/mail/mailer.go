package mail

import (
	"context"
	"errors"
	"log/slog"
)

type Development struct{}

func (Development) SendMagicLink(_ context.Context, _, _ string) error {
	slog.Warn("development mail adapter: magic link was not delivered; token intentionally omitted")
	return nil
}
func (Development) SendDecision(_ context.Context, _, _, _ string) error {
	slog.Warn("development mail adapter: decision notification was not delivered; personal data omitted")
	return nil
}
func (Development) SendVerification(context.Context, string, string) error {
	return errors.New("development mail adapter does not deliver verification tokens")
}
func (Development) SendPasswordReset(context.Context, string, string) error {
	return errors.New("development mail adapter does not deliver password reset tokens")
}
func MagicLinkTemplate(link string) string {
	return "Acesse sua conta Loja 3D pelo link temporário: " + link
}
func ProposalTemplate(reference, link string) string {
	return "Nova proposta para " + reference + ". Consulte com segurança: " + link
}
func DecisionTemplate(reference, decision string) string {
	return "A decisão " + decision + " foi registrada para " + reference + "."
}

type Unavailable struct{}

func (Unavailable) SendMagicLink(context.Context, string, string) error {
	return errors.New("email provider unavailable")
}
func (Unavailable) SendDecision(context.Context, string, string, string) error {
	return errors.New("email provider unavailable")
}
func (Unavailable) SendVerification(context.Context, string, string) error {
	return errors.New("email provider unavailable")
}
func (Unavailable) SendPasswordReset(context.Context, string, string) error {
	return errors.New("email provider unavailable")
}
