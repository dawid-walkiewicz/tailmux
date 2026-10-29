// Package apiauth is the secret that guards tailmux's local HTTP API.
//
// The daemon keeps a random token in a file next to its config, readable
// only by the config's owner (and root/Administrators). Every API request
// must carry it in the X-Tailmux header. That keeps out other users on the
// same machine, and web pages, which can't read the file: not even by
// DNS rebinding, which defeats a header that merely has to be present.
package apiauth

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Header carries the token on every API request.
const Header = "X-Tailmux"

const fileName = "api-token"

// Path is where the token for the config at configPath lives.
func Path(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), fileName)
}

// Load returns the token in path, creating one if there is none. A new
// file is private to the owner of ownerOf (the config file), so the user
// who set tailmux up can still use the apps and the CLI when the daemon
// runs as root.
func Load(path, ownerOf string) (string, error) {
	tok, err := read(path)
	if err == nil {
		return tok, nil
	}
	if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, errUnsafe) {
		return "", err
	}
	if errors.Is(err, errUnsafe) {
		// Others could read it (or it isn't ours to trust): start over.
		if err := os.Remove(path); err != nil {
			return "", err
		}
	}
	return create(path, ownerOf)
}

// Reset replaces the token in path with a new one, private to the
// current user (plus root/Administrators); for installing the service.
func Reset(path string) (string, error) {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return create(path, "")
}

// Read returns the first token that can be read from paths, or "".
func Read(paths ...string) string {
	for _, p := range paths {
		if tok, err := read(p); err == nil {
			return tok
		}
	}
	return ""
}

// Valid reports whether got is the token. An empty token matches nothing.
func Valid(got, token string) bool {
	return token != "" && subtle.ConstantTimeCompare([]byte(got), []byte(token)) == 1
}

var errUnsafe = errors.New("api token file is readable by others")

func read(path string) (string, error) {
	b, err := readPrivate(path)
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		return "", fmt.Errorf("%s: %w", path, errUnsafe)
	}
	return tok, nil
}

func create(path, ownerOf string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	tok := rand.Text()
	if err := writePrivate(path, ownerOf, []byte(tok+"\n")); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return read(path) // another process just made one
		}
		return "", fmt.Errorf("api token: %w", err)
	}
	return tok, nil
}
