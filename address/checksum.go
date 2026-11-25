package address

import (
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/sha3"
	"strings"
)

func ToChecksum(addr string) (string, error) {
	// it must have the 0x before to be an valid EVM address
	hasPrefix := strings.HasPrefix(addr, "0x")
	if !hasPrefix {
		return "", fmt.Errorf("input is not an address: %q", addr)
	}

	// It must have 40 characters
	body := addr[2:]
	if len(body) != 40 {
		return "", fmt.Errorf("not an address body: %q", body)
	}

	_, err := hex.DecodeString(body)
	if err != nil {
		return "", fmt.Errorf("input is not a hex value: %w", err)
	}

	lowercasedBody := strings.ToLower(body)

	hash := sha3.NewLegacyKeccak256()
	hash.Write([]byte(lowercasedBody))
	hashHex := hex.EncodeToString(hash.Sum(nil))

	out := []byte(lowercasedBody)
	for i := 0; i < len(out); i++ {
		if out[i] >= 'a' && out[i] <= 'f' && hashHex[i] >= '8' {
			out[i] = out[i] - 'a' + 'A'
		}

	}

	return "0x" + string(out), nil
}

func IsChecksummed(addr string) (bool, error) {
	answer, err := ToChecksum(addr)
	if err != nil {
		return false, fmt.Errorf("Conversion failed: %w", err)
	}

	return answer == addr, nil
}
