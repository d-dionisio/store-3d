package bambulab

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// RotateCredentials re-encrypts every non-active credential inside the caller's
// transaction. Re-running it is safe: rows already on the active key are skipped.
func RotateCredentials(ctx context.Context, tx pgx.Tx, keyring *Cipher) (int, error) {
	rows, err := tx.Query(ctx, `SELECT printer_id,credentials_ciphertext,key_version FROM printer_integrations WHERE key_version<>$1 ORDER BY printer_id FOR UPDATE`, keyring.Version())
	if err != nil {
		return 0, err
	}
	type record struct {
		id         string
		ciphertext []byte
		version    int16
	}
	records := []record{}
	for rows.Next() {
		var value record
		if err = rows.Scan(&value.id, &value.ciphertext, &value.version); err != nil {
			rows.Close()
			return 0, err
		}
		records = append(records, value)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, value := range records {
		plain, decryptErr := keyring.Decrypt(value.ciphertext, value.version)
		if decryptErr != nil {
			return 0, fmt.Errorf("credential for printer %s cannot be decrypted: %w", value.id, decryptErr)
		}
		ciphertext, encryptErr := keyring.Encrypt(plain)
		for i := range plain {
			plain[i] = 0
		}
		if encryptErr != nil {
			return 0, encryptErr
		}
		tag, updateErr := tx.Exec(ctx, `UPDATE printer_integrations SET credentials_ciphertext=$1,key_version=$2,version=version+1,updated_at=now() WHERE printer_id=$3 AND key_version=$4`, ciphertext, keyring.Version(), value.id, value.version)
		if updateErr != nil {
			return 0, updateErr
		}
		if tag.RowsAffected() != 1 {
			return 0, fmt.Errorf("credential changed concurrently for printer %s", value.id)
		}
	}
	var invalid int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM printer_integrations WHERE key_version<>$1`, keyring.Version()).Scan(&invalid); err != nil {
		return 0, err
	}
	if invalid != 0 {
		return 0, fmt.Errorf("rotation verification failed")
	}
	return len(records), nil
}
