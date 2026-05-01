package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
)

// GenerateID creates a unique agent ID
func GenerateID(hostname string) string {
	// Simple implementation - in production use proper UUID
	return hostname + "-" + randomString(8)
}

// GenerateHMAC creates an HMAC-SHA256 signature
func GenerateHMAC(data string, secret string) string {
	if secret == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// GenerateMessageHMAC creates HMAC for a message object
func GenerateMessageHMAC(msg interface{}, secret string) (string, error) {
	if secret == "" {
		return "", nil
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}

	return GenerateHMAC(string(data), secret), nil
}

// randomString generates a random alphanumeric string
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}
