package auth

import (
	"bufio"
	"context"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type PasswordChecker interface {
	Compromised(context.Context, string) (bool, error)
}
type NoopPasswordChecker struct{}

func (NoopPasswordChecker) Compromised(context.Context, string) (bool, error) { return false, nil }

type KAnonymityChecker struct {
	BaseURL    string
	Client     *http.Client
	FailClosed bool
}

func (c KAnonymityChecker) Compromised(ctx context.Context, password string) (bool, error) {
	sum := sha1.Sum([]byte(password))
	encoded := strings.ToUpper(hex.EncodeToString(sum[:]))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+"/"+encoded[:5], nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Add-Padding", "true")
	response, err := c.Client.Do(req)
	if err != nil {
		if c.FailClosed {
			return false, err
		}
		return false, nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if c.FailClosed {
			return false, errors.New("breach provider unavailable")
		}
		return false, nil
	}
	reader := bufio.NewReader(io.LimitReader(response.Body, 2<<20))
	for {
		line, readErr := reader.ReadString('\n')
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) == 2 && len(parts[0]) == 35 && subtle.ConstantTimeCompare([]byte(strings.ToUpper(parts[0])), []byte(encoded[5:])) == 1 {
			count, _ := strconv.Atoi(parts[1])
			return count > 0, nil
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return false, readErr
		}
	}
	return false, nil
}
