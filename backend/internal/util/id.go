package util

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// NewBusinessID 生成带前缀的业务主键，如 wo-20260923-1a2b3c4d。
func NewBusinessID(prefix string) string {
	now := time.Now().Format("20060102")
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s-%s-%d", prefix, now, time.Now().UnixNano())
	}
	return fmt.Sprintf("%s-%s-%s", prefix, now, hex.EncodeToString(buf))
}
