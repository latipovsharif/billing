package base

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func newKey(t *testing.T) string {
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(k)
}

func TestCipherRoundtrip(t *testing.T) {
	c, err := NewCipher(newKey(t))
	if err != nil {
		t.Fatal(err)
	}
	enc, err := c.Encrypt([]byte("tok_secret"))
	if err != nil {
		t.Fatal(err)
	}
	dec, err := c.Decrypt(enc)
	if err != nil || string(dec) != "tok_secret" {
		t.Fatalf("roundtrip: %q %v", dec, err)
	}
}

func TestCipherWrongKeyFails(t *testing.T) {
	c1, _ := NewCipher(newKey(t))
	c2, _ := NewCipher(newKey(t))
	enc, _ := c1.Encrypt([]byte("x"))
	if _, err := c2.Decrypt(enc); err == nil {
		t.Fatal("expected decrypt failure with wrong key")
	}
}

func TestNewCipherRejectsShortKey(t *testing.T) {
	if _, err := NewCipher("c2hvcnQ="); err == nil { // "short"
		t.Fatal("expected error for non-32-byte key")
	}
}
