package bambulab

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRotateCredentialsPostgreSQLTransaction(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil || !strings.HasSuffix(strings.ToLower(cfg.ConnConfig.Database), "_test") || strings.EqualFold(cfg.ConnConfig.Database, "loja_3d") {
		t.Fatal("credential rotation integration test requires a dedicated *_test database")
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var adminID string
	if err = tx.QueryRow(ctx, `SELECT id FROM admin_users ORDER BY created_at LIMIT 1`).Scan(&adminID); err != nil {
		t.Skip("test database has no administrative fixture")
	}
	var printerID string
	if err = tx.QueryRow(ctx, `INSERT INTO printers(code,name,manufacturer,model,build_width_mm,build_depth_mm,build_height_mm)VALUES('BAMBU-ROTATION-TEST','Bambu rotation test','Bambu Lab','A1',256,256,256)RETURNING id`).Scan(&printerID); err != nil {
		t.Fatal(err)
	}
	oldKey := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	newKey := base64.StdEncoding.EncodeToString([]byte("abcdefghijklmnopqrstuvwxyz123456"))
	oldCipher, _ := NewCipher(oldKey, 1)
	ciphertext, _ := oldCipher.Encrypt([]byte("test-code"))
	_, err = tx.Exec(ctx, `INSERT INTO printer_integrations(printer_id,provider,host,port,serial_number,certificate_sha256,credentials_ciphertext,key_version,created_by_admin_user_id,updated_by_admin_user_id)VALUES($1,'bambulab_lan','127.0.0.1',8883,'ROTATION_TEST_SERIAL',repeat('a',64),$2,1,$3,$3)`, printerID, ciphertext, adminID)
	if err != nil {
		t.Fatal(err)
	}
	keyring, err := NewKeyring(2, map[int16]string{1: oldKey, 2: newKey})
	if err != nil {
		t.Fatal(err)
	}
	count, err := RotateCredentials(ctx, tx, keyring)
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	count, err = RotateCredentials(ctx, tx, keyring)
	if err != nil || count != 0 {
		t.Fatalf("idempotent count=%d err=%v", count, err)
	}
	var rotated []byte
	var version int16
	if err = tx.QueryRow(ctx, `SELECT credentials_ciphertext,key_version FROM printer_integrations WHERE printer_id=$1`, printerID).Scan(&rotated, &version); err != nil {
		t.Fatal(err)
	}
	plain, err := keyring.Decrypt(rotated, version)
	if err != nil || string(plain) != "test-code" {
		t.Fatal("rotated credential verification failed")
	}
}
