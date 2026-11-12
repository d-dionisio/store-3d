package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"example.com/loja-3d/backend/internal/infrastructure/mail"
	"example.com/loja-3d/backend/internal/infrastructure/postgres"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "status" && os.Args[1] != "rotate") {
		log.Fatal("usage: go run ./cmd/email-key <status|rotate>")
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
	ctx := context.Background()
	if os.Args[1] == "status" {
		rows, queryErr := pool.Query(ctx, `SELECT key_version,status,count(*) FROM email_outbox GROUP BY key_version,status ORDER BY key_version,status`)
		if queryErr != nil {
			log.Fatal(queryErr)
		}
		defer rows.Close()
		for rows.Next() {
			var version, count int
			var status string
			if rows.Scan(&version, &status, &count) != nil {
				log.Fatal("invalid result")
			}
			fmt.Printf("version=%d status=%s messages=%d\n", version, status, count)
		}
		return
	}
	active64, err := strconv.ParseInt(os.Getenv("EMAIL_OUTBOX_KEY_VERSION"), 10, 16)
	if err != nil || active64 < 1 {
		log.Fatal("invalid active key version")
	}
	active := int16(active64)
	keys := map[int16]string{active: os.Getenv("EMAIL_OUTBOX_ENCRYPTION_KEY")}
	for _, entry := range strings.Split(os.Getenv("EMAIL_OUTBOX_DECRYPTION_KEYS"), ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 {
			continue
		}
		version, parseErr := strconv.ParseInt(parts[0], 10, 16)
		if parseErr == nil && version > 0 {
			keys[int16(version)] = parts[1]
		}
	}
	keyring, err := mail.NewOutboxKeyring(active, keys)
	if err != nil {
		log.Fatal("invalid email outbox keyring")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT id,payload_ciphertext,key_version FROM email_outbox WHERE key_version<>$1 FOR UPDATE`, active)
	if err != nil {
		log.Fatal(err)
	}
	type message struct {
		id         string
		ciphertext []byte
		version    int16
	}
	var messages []message
	for rows.Next() {
		var item message
		if err = rows.Scan(&item.id, &item.ciphertext, &item.version); err != nil {
			log.Fatal(err)
		}
		messages = append(messages, item)
	}
	rows.Close()
	for _, item := range messages {
		plain, decryptErr := keyring.Decrypt(item.ciphertext, item.version)
		if decryptErr != nil {
			log.Fatalf("cannot decrypt message %s with key version %d", item.id, item.version)
		}
		encrypted, encryptErr := keyring.Encrypt(plain)
		for i := range plain {
			plain[i] = 0
		}
		if encryptErr != nil {
			log.Fatal("reencryption failed")
		}
		if _, err = tx.Exec(ctx, `UPDATE email_outbox SET payload_ciphertext=$2,key_version=$3 WHERE id=$1 AND key_version=$4`, item.id, encrypted, active, item.version); err != nil {
			log.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("reencrypted=%d active_version=%d\n", len(messages), active)
}
