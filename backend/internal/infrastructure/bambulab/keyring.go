package bambulab

import (
	"fmt"
	"strconv"
	"strings"
)

// LoadKeyring accepts an active key and optional comma-separated historical
// keys in the form "1:base64,2:base64". It never includes key material in errors.
func LoadKeyring(activeEncoded string, activeVersion int16, historical string) (*Cipher, error) {
	keys := map[int16]string{activeVersion: strings.TrimSpace(activeEncoded)}
	for _, item := range strings.Split(historical, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid BAMBU_LAN_DECRYPTION_KEYS entry")
		}
		version, err := strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 16)
		if err != nil || version < 1 {
			return nil, fmt.Errorf("invalid Bambu LAN historical key version")
		}
		if int16(version) == activeVersion {
			continue
		}
		keys[int16(version)] = strings.TrimSpace(parts[1])
	}
	return NewKeyring(activeVersion, keys)
}
