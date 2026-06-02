package ocpp

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

func GenerateUniqueID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback-%d", len(b))
	}
	return hex.EncodeToString(b)
}
