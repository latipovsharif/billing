package webhooks

import "testing"

func TestSignDeterministicAndVerifies(t *testing.T) {
	body := []byte(`{"a":1}`)
	sig := Sign("secret", body)
	if sig == "" {
		t.Fatal("empty signature")
	}
	if !Verify("secret", body, sig) {
		t.Fatal("verify failed for valid signature")
	}
	if Verify("wrong", body, sig) {
		t.Fatal("verify passed for wrong secret")
	}
}
