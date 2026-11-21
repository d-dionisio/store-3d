package bambulab

import (
	"context"
	"encoding/base64"
	"os"
	"testing"
	"time"

	testpostgres "example.com/loja-3d/backend/internal/testsupport/postgres"
)

func TestRotateCredentialsPostgreSQLTransaction(t *testing.T) {
	if _, ok := os.LookupEnv(testpostgres.EnvDatabaseURL); !ok {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	schema, err := testpostgres.CreateSchema(ctx, t, "bambulab_rotation")
	if err != nil {
		t.Fatalf("create isolated Bambu Lab rotation schema: %v", err)
	}
	if err = schema.ApplyOfficialMigrations(ctx); err != nil {
		t.Fatalf("apply official migrations to Bambu Lab rotation schema: %v", err)
	}
	pool, err := schema.OpenPool(ctx)
	if err != nil {
		t.Fatalf("open isolated Bambu Lab rotation pool: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var adminID string
	if err = tx.QueryRow(ctx, `INSERT INTO admin_users(email,full_name,password_hash,role) VALUES('bambulab-rotation@example.test','Rotation fixture',repeat('x',60),'production_manager') RETURNING id::text`).Scan(&adminID); err != nil {
		t.Fatalf("insert isolated administrator fixture: %v", err)
	}
	var printerID string
	if err = tx.QueryRow(ctx, `INSERT INTO printers(code,name,manufacturer,model,build_width_mm,build_depth_mm,build_height_mm)VALUES('BAMBU-ROTATION-TEST','Bambu rotation test','Bambu Lab','A1',256,256,256)RETURNING id::text`).Scan(&printerID); err != nil {
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
