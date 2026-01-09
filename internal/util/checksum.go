package util

import (
	"crypto/sha256"
	"encoding/base64"
)

func CheckSumOf(m map[string]string) string {
	h := sha256.New()
	for k, v := range m {
		h.Write([]byte(k))
		h.Write([]byte(v))
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
