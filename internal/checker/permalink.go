package checker

import (
	"crypto/sha256"
	"fmt"
	"time"
)

const base62chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// NewID generates a short URL-safe base62 ID from name+type+current time.
func NewID(name, qtype string) string {
	raw := fmt.Sprintf("%s|%s|%d", name, qtype, time.Now().UnixNano())
	sum := sha256.Sum256([]byte(raw))

	// encode first 8 bytes as base62
	var n uint64
	for i := 0; i < 8; i++ {
		n = (n << 8) | uint64(sum[i])
	}

	result := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		result[i] = base62chars[n%62]
		n /= 62
	}
	return string(result)
}
