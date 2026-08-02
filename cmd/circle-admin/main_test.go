package main

import "testing"

func TestResolvePasswordHashUsesEnvironmentValue(t *testing.T) {
	value, err := resolvePasswordHash("hash-from-env")
	if err != nil {
		t.Fatalf("resolvePasswordHash returned error: %v", err)
	}
	if value != "hash-from-env" {
		t.Fatalf("password hash = %q, want hash-from-env", value)
	}
}

func TestResolvePasswordHashRejectsMissingEnvironmentValue(t *testing.T) {
	_, err := resolvePasswordHash("")
	if err == nil {
		t.Fatal("expected missing password hash env to fail")
	}
}
