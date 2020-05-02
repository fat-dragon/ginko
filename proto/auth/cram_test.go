package auth

import (
	"encoding/hex"
	"testing"
)

func TestChallengeToString(t *testing.T) {
	challenge := []byte{0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe, 0xf0, 0x0d}
	const expected = "deadbeefcafef00d"
	val := ChallengeToString(challenge)
	if val != expected {
		t.Errorf("ChallengeToString failed. Expected %q, got %q", expected, val)
	}
}

func TestValidateChallenge(t *testing.T) {
	challenge := "45e1f73c9b7a888d0650cc0c74b56dee"
	response := "5b6733465fdc87cb60b175d677ace798"
	password := "NOT_MY_REAL_PASSWORD"
	c, err := hex.DecodeString(challenge)
	if err != nil {
		t.Fatalf("decoding challenge %q failed: %v", challenge, err)
	}
	if !ValidateResponse("MD5", c, response, password) {
		t.Errorf("response validation failed!")
	}
}
