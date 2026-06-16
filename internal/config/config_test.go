package config

import (
	"testing"
)

func TestValidateKokuURL_HTTPS(t *testing.T) {
	cfg := &KokuConfig{APIURL: "https://koku.example.com"}
	if err := validateKokuURL(cfg); err != nil {
		t.Fatalf("expected no error for HTTPS URL, got: %v", err)
	}
}

func TestValidateKokuURL_HTTPRejected(t *testing.T) {
	cfg := &KokuConfig{APIURL: "http://koku.example.com"}
	if err := validateKokuURL(cfg); err == nil {
		t.Fatal("expected error for non-HTTPS URL")
	}
}

func TestValidateKokuURL_HTTPLocalhostAllowed(t *testing.T) {
	for _, url := range []string{"http://localhost:8000", "http://127.0.0.1:8000"} {
		cfg := &KokuConfig{APIURL: url}
		if err := validateKokuURL(cfg); err != nil {
			t.Fatalf("expected localhost to be allowed, got: %v (url=%s)", err, url)
		}
	}
}

func TestValidateKokuURL_InsecureOverride(t *testing.T) {
	cfg := &KokuConfig{APIURL: "http://koku.example.com", AllowInsecure: true}
	if err := validateKokuURL(cfg); err != nil {
		t.Fatalf("expected AllowInsecure to skip validation, got: %v", err)
	}
}

func TestRedactedIdentity(t *testing.T) {
	cfg := &KokuConfig{Identity: "abcdefghijklmnop"}
	r := cfg.RedactedIdentity()
	if r != "abcd...mnop" {
		t.Errorf("expected abcd...mnop, got %s", r)
	}
}

func TestRedactedIdentity_Short(t *testing.T) {
	cfg := &KokuConfig{Identity: "short"}
	r := cfg.RedactedIdentity()
	if r != "***" {
		t.Errorf("expected ***, got %s", r)
	}
}
