package hash

import "testing"

func TestSha256_Hash(t *testing.T) {
	const uuid = "0BE2E2F7-A8A5-41A0-957E-59F92C3D81CB"
	sha := &Sha256{}

	expected := "9eff2aabbfde0c6dcbd8266592b1062023ed05d1a1b384001d8111aa35d484a4"
	actual := sha.Hash(uuid)

	if actual != expected {
		t.Errorf("Sha256.Hash() got: %q, want: %q", actual, expected)
	}
}
