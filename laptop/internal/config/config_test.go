package config

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempDir points XDG_CONFIG_HOME at a throwaway dir and restores it after.
func withTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", dir)
	t.Cleanup(func() { os.Setenv("XDG_CONFIG_HOME", old) })
	return dir
}

func TestSaveLoadSecretRoundtrip(t *testing.T) {
	withTempDir(t)
	secret := []byte("sO0ai9K4ES7GtoBe4DeQLeEbD-JuGyeGvgoY2eXcHSM")
	if err := SaveSecret(secret); err != nil {
		t.Fatalf("SaveSecret: %v", err)
	}
	got, err := LoadSecret()
	if err != nil {
		t.Fatalf("LoadSecret: %v", err)
	}
	if string(got) != string(secret) {
		t.Errorf("secret = %q, want %q", got, secret)
	}
}

func TestLoadSecretMissingReturnsNil(t *testing.T) {
	withTempDir(t)
	got, err := LoadSecret()
	if err != nil {
		t.Fatalf("LoadSecret should not error on missing file: %v", err)
	}
	if got != nil {
		t.Errorf("secret = %v, want nil", got)
	}
}

func TestSecretFileMode0600(t *testing.T) {
	withTempDir(t)
	if err := SaveSecret([]byte("secretdata")); err != nil {
		t.Fatalf("SaveSecret: %v", err)
	}
	info, err := os.Stat(SecretPath())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("secret file mode = %o, want 0600", mode)
	}
}

func TestGenerateSecretIs32Bytes(t *testing.T) {
	s1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	if len(s1) != 32 {
		t.Errorf("secret len = %d, want 32", len(s1))
	}
	s2, _ := GenerateSecret()
	if string(s1) == string(s2) {
		t.Error("two generated secrets should differ")
	}
}

func TestDeleteSecret(t *testing.T) {
	withTempDir(t)
	if err := SaveSecret([]byte("secretdata")); err != nil {
		t.Fatalf("SaveSecret: %v", err)
	}
	if err := DeleteSecret(); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}
	if _, err := os.Stat(SecretPath()); !os.IsNotExist(err) {
		t.Errorf("secret file should be gone, err = %v", err)
	}
}

func TestSaveLoadPortRoundtrip(t *testing.T) {
	withTempDir(t)
	if err := SavePort(10369); err != nil {
		t.Fatalf("SavePort: %v", err)
	}
	got, err := LoadPort()
	if err != nil {
		t.Fatalf("LoadPort: %v", err)
	}
	if got != 10369 {
		t.Errorf("port = %d, want 10369", got)
	}
}

func TestLoadPortMissingReturnsZero(t *testing.T) {
	withTempDir(t)
	got, err := LoadPort()
	if err != nil {
		t.Fatalf("LoadPort should not error on missing file: %v", err)
	}
	if got != 0 {
		t.Errorf("port = %d, want 0", got)
	}
}

func TestSavePortRejectsInvalid(t *testing.T) {
	for _, p := range []int{0, -1, 65536, 70000} {
		if err := SavePort(p); err == nil {
			t.Errorf("SavePort(%d) should fail", p)
		}
	}
}

func TestLoadPortRejectsCorrupt(t *testing.T) {
	if err := os.WriteFile(PortFile(), []byte("not-a-number"), 0600); err != nil {
		t.Fatalf("write port: %v", err)
	}
	got, err := LoadPort()
	if err != nil {
		t.Fatalf("LoadPort on corrupt file should return 0,nil: %v", err)
	}
	if got != 0 {
		t.Errorf("port = %d, want 0", got)
	}
}

func TestPortFileMode0600(t *testing.T) {
	withTempDir(t)
	if err := SavePort(10369); err != nil {
		t.Fatalf("SavePort: %v", err)
	}
	info, err := os.Stat(PortFile())
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0600 {
		t.Errorf("port file mode = %o, want 0600", mode)
	}
}

func TestSecretDirUsesXDG(t *testing.T) {
	dir := withTempDir(t)
	got := SecretDir()
	want := filepath.Join(dir, "clipconnect")
	if got != want {
		t.Errorf("SecretDir = %q, want %q", got, want)
	}
}
