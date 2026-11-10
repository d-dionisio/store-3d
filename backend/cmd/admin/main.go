package main

import (
	"context"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"os"
	"strings"
)

func main() {
	email := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	name := strings.TrimSpace(os.Getenv("ADMIN_NAME"))
	password := os.Getenv("ADMIN_PASSWORD")
	if email == "" || name == "" || len(password) < 12 {
		fmt.Fprintln(os.Stderr, "ADMIN_EMAIL, ADMIN_NAME and ADMIN_PASSWORD (minimum 12 characters) are required")
		os.Exit(2)
	}
	config, err := postgres.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid database configuration")
		os.Exit(1)
	}
	pool, err := postgres.NewPool(context.Background(), config)
	if err != nil {
		fmt.Fprintln(os.Stderr, "database unavailable")
		os.Exit(1)
	}
	defer pool.Close()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		os.Exit(1)
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES($1,$2,$3,'admin') ON CONFLICT((lower(email))) DO UPDATE SET full_name=excluded.full_name,password_hash=excluded.password_hash,status='active',updated_at=now()`, email, name, string(hash))
	if err != nil {
		fmt.Fprintln(os.Stderr, "could not create administrator")
		os.Exit(1)
	}
	fmt.Println("administrator created or updated")
}
