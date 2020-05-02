package auth

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"strings"
)

// GenerateChallenge creates a cryptographically strong random
// 128-bit challenge to authenticate the connection in CRAM mode.
func GenerateChallenge() []byte {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		panic("The universe just collapsed")
	}
	return b
}

// GenerateResponse takes a challenge, a password, and a hash type
// (as a string) and generates a response of the given type.
func GenerateResponse(hashStr string, challenge []byte, password string) (string, error) {
	h, err := hashNew(hashStr)
	if err != nil {
		return "", err
	}
	mac := hmac.New(h, []byte(password))
	mac.Write(challenge)
	return hex.EncodeToString(mac.Sum(nil)), nil
}

// ChallengeToString returns a hex-encoded string representing
// the given challenge bytes.
func ChallengeToString(challenge []byte) string {
	return hex.EncodeToString(challenge)
}

// DecodeChallenge decodes the hex-string challenge into bytes.
func DecodeChallenge(challenge string) ([]byte, error) {
	return hex.DecodeString(challenge)
}

// ValidateResponse returns true if and only if `response` is
// a successful reply to the given challenge and password.
func ValidateResponse(hashStr string, challenge []byte, response string, password string) bool {
	h, err := hashNew(hashStr)
	if err != nil {
		return false
	}
	mac := hmac.New(h, []byte(password))
	mac.Write(challenge)
	expectedMAC := mac.Sum(nil)
	responseMAC, err := hex.DecodeString(response)
	if err != nil {
		return false
	}
	return hmac.Equal(responseMAC, expectedMAC)
}

// hashNew returns a `New` function for the hash type named
// by `hashStr`.
func hashNew(hashStr string) (func() hash.Hash, error) {
	switch strings.ToLower(hashStr) {
	case "md5":
		return md5.New, nil
	case "sha1":
		return sha1.New, nil
	case "sha256":
		return sha256.New, nil
	default:
		return nil, fmt.Errorf("Unknown hash type %q", hashStr)
	}
}
