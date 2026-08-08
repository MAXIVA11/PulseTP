package pulsetp

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := []byte("This is PulseTP")

	ciphertext, err := Encrypt("correct horse", plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if string(ciphertext) == string(plaintext) {
		t.Fatalf("ciphertext matches plaintext, encryption did nothing")
	}

	got, err := Decrypt("correct horse", ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("got %q, want %q", got, plaintext)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	ciphertext, err := Encrypt("correct horse", []byte("secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := Decrypt("wrong horse", ciphertext); err == nil {
		t.Fatalf("expected an error decrypting with the wrong key, got nil")
	}
}

func TestDecryptTamperedDataFails(t *testing.T) {
	ciphertext, err := Encrypt("correct horse", []byte("secret"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	ciphertext[len(ciphertext)-1] ^= 0xFF
	if _, err := Decrypt("correct horse", ciphertext); err == nil {
		t.Fatalf("expected an error decrypting tampered data, got nil")
	}
}

func TestDecryptTooShortFails(t *testing.T) {
	if _, err := Decrypt("key", []byte("short")); err == nil {
		t.Fatalf("expected an error for undersized data, got nil")
	}
}
