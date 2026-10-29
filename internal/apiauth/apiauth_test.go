//go:build unix

package apiauth

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.json")
	os.WriteFile(cfg, []byte("{}"), 0o600)
	path := Path(cfg)

	tok, err := Load(path, cfg)
	if err != nil || len(tok) < 20 {
		t.Fatalf("new token %q %v", tok, err)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v, want 0600", fi.Mode().Perm())
	}
	if again, err := Load(path, cfg); err != nil || again != tok {
		t.Errorf("second load %q %v, want the same token", again, err)
	}
	if got := Read(filepath.Join(dir, "nope"), path); got != tok {
		t.Errorf("Read %q, want %q", got, tok)
	}

	t.Run("readable by others: replaced", func(t *testing.T) {
		os.Chmod(path, 0o644)
		if Read(path) != "" {
			t.Error("read a token others can read")
		}
		fresh, err := Load(path, cfg)
		if err != nil || fresh == tok {
			t.Fatalf("got %q %v, want a new token", fresh, err)
		}
		tok = fresh
	})

	t.Run("symlink refused", func(t *testing.T) {
		secret := filepath.Join(dir, "secret")
		os.WriteFile(secret, []byte("not a token\n"), 0o600)
		link := filepath.Join(dir, "link")
		os.Symlink(secret, link)
		if Read(link) != "" {
			t.Error("followed a symlink")
		}
		if _, err := Load(link, cfg); err != nil {
			t.Fatal(err)
		}
		if b, _ := os.ReadFile(secret); string(b) != "not a token\n" {
			t.Error("wrote through the symlink")
		}
	})

	t.Run("reset", func(t *testing.T) {
		fresh, err := Reset(path)
		if err != nil || fresh == tok || Read(path) != fresh {
			t.Fatalf("reset: %q %v", fresh, err)
		}
	})
}

func TestValid(t *testing.T) {
	for _, c := range []struct {
		got, tok string
		want     bool
	}{
		{"abc", "abc", true},
		{"abd", "abc", false},
		{"1", "abc", false},
		{"", "", false},
		{"x", "", false},
	} {
		if Valid(c.got, c.tok) != c.want {
			t.Errorf("Valid(%q, %q) != %v", c.got, c.tok, c.want)
		}
	}
}
