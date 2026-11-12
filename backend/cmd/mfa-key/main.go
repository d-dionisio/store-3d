package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"example.com/loja-3d/backend/internal/infrastructure/postgres"
	security "example.com/loja-3d/backend/internal/security/auth"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "status" && os.Args[1] != "rotate") {
		log.Fatal("usage: go run ./cmd/mfa-key <status|rotate>")
	}
	config, err := postgres.LoadConfig()
	if err != nil {
		log.Fatal("invalid database configuration")
	}
	pool, err := postgres.NewPool(context.Background(), config)
	if err != nil {
		log.Fatal("database unavailable")
	}
	defer pool.Close()
	if os.Args[1] == "status" {
		rows, err := pool.Query(context.Background(), `SELECT key_version,count(*) FROM auth_mfa_factors GROUP BY key_version ORDER BY key_version`)
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var version, count int
			if rows.Scan(&version, &count) != nil {
				log.Fatal("invalid result")
			}
			fmt.Printf("version=%d factors=%d\n", version, count)
		}
		return
	}
	active64, err := strconv.ParseInt(os.Getenv("AUTH_MFA_KEY_VERSION"), 10, 16)
	if err != nil || active64 < 1 {
		log.Fatal("invalid active key version")
	}
	active := int16(active64)
	keys := map[int16]string{active: os.Getenv("AUTH_MFA_ENCRYPTION_KEY")}
	for _, entry := range strings.Split(os.Getenv("AUTH_MFA_DECRYPTION_KEYS"), ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 {
			continue
		}
		v, e := strconv.ParseInt(parts[0], 10, 16)
		if e == nil && v > 0 {
			keys[int16(v)] = parts[1]
		}
	}
	keyring, err := security.NewKeyring(active, keys)
	if err != nil {
		log.Fatal("invalid MFA keyring")
	}
	tx, err := pool.Begin(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	rows, err := tx.Query(context.Background(), `SELECT id,secret_ciphertext,key_version FROM auth_mfa_factors WHERE key_version<>$1 FOR UPDATE`, active)
	if err != nil {
		log.Fatal(err)
	}
	type factor struct {
		id      string
		value   []byte
		version int16
	}
	var factors []factor
	for rows.Next() {
		var f factor
		if err = rows.Scan(&f.id, &f.value, &f.version); err != nil {
			log.Fatal(err)
		}
		factors = append(factors, f)
	}
	rows.Close()
	for _, f := range factors {
		plain, e := keyring.Decrypt(f.value, f.version)
		if e != nil {
			log.Fatalf("cannot decrypt factor %s with key version %d", f.id, f.version)
		}
		encrypted, e := keyring.Encrypt(plain)
		for i := range plain {
			plain[i] = 0
		}
		if e != nil {
			log.Fatal("reencryption failed")
		}
		if _, e = tx.Exec(context.Background(), `UPDATE auth_mfa_factors SET secret_ciphertext=$2,key_version=$3,updated_at=now() WHERE id=$1 AND key_version=$4`, f.id, encrypted, active, f.version); e != nil {
			log.Fatal(e)
		}
	}
	if err = tx.Commit(context.Background()); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("reencrypted=%d active_version=%d\n", len(factors), active)
}
