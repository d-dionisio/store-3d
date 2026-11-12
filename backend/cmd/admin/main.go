package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	adminapp "example.com/loja-3d/backend/internal/application/admin"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "bootstrap" {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/admin bootstrap")
		os.Exit(2)
	}
	if os.Getenv("ADMIN_BOOTSTRAP_ENABLED") != "true" {
		fail("bootstrap disabled; set ADMIN_BOOTSTRAP_ENABLED=true only for the initial authorized operation")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fail("bootstrap requires an interactive terminal")
	}
	reader := bufio.NewReader(os.Stdin)
	email := prompt(reader, "E-mail: ")
	name := prompt(reader, "Nome completo: ")
	password := secret("Senha (mínimo 12 caracteres): ")
	confirmation := secret("Confirme a senha: ")
	if !strings.Contains(email, "@") || name == "" || len(password) < 12 || len(password) > 128 || password != confirmation {
		fmt.Fprintln(os.Stderr, "dados inválidos ou senhas diferentes")
		os.Exit(2)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	password, confirmation = "", ""
	if err != nil {
		fail("could not hash password")
	}
	config, err := postgres.LoadConfig()
	if err != nil {
		fail("invalid database configuration")
	}
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, config)
	if err != nil {
		fail("database unavailable")
	}
	defer pool.Close()
	user, err := postgres.NewAdminRepository(pool).BootstrapAdmin(ctx, strings.ToLower(strings.TrimSpace(email)), name, string(hash))
	if errors.Is(err, adminapp.ErrForbidden) {
		fail("bootstrap permanently unavailable: an internal user already exists")
	}
	if err != nil {
		fail("could not create initial administrator")
	}
	fmt.Printf("initial administrator created: %s (%s)\n", user.Email, user.ID)
}

func prompt(reader *bufio.Reader, label string) string {
	fmt.Fprint(os.Stderr, label)
	value, _ := reader.ReadString('\n')
	return strings.TrimSpace(value)
}
func secret(label string) string {
	fmt.Fprint(os.Stderr, label)
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fail("could not read password")
	}
	return string(value)
}
func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
