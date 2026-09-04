package trash

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// move prefers `gio trash`, which knows about per-mount trash directories and
// the desktop's own conventions, and falls back to the freedesktop.org home
// trash (`$XDG_DATA_HOME/Trash`) when gio is unavailable.
func move(abs string) error {
	var gioErr error
	if _, err := exec.LookPath("gio"); err == nil {
		out, err := exec.Command("gio", "trash", "--", abs).CombinedOutput()
		if err == nil {
			return nil
		}
		gioErr = fmt.Errorf("gio trash: %s", strings.TrimSpace(string(out)))
	}
	if err := homeTrash(abs); err != nil {
		if gioErr != nil {
			return fmt.Errorf("%w; %v", err, gioErr)
		}
		return err
	}
	return nil
}

func trashRoot() (string, error) {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "Trash"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "Trash"), nil
}

// escapePath percent-encodes a path for a .trashinfo `Path=` line, keeping
// the separators.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

func homeTrash(abs string) error {
	root, err := trashRoot()
	if err != nil {
		return fmt.Errorf("could not locate the trash directory: %w", err)
	}
	files := filepath.Join(root, "files")
	info := filepath.Join(root, "info")
	for _, d := range []string{files, info} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return fmt.Errorf("could not create %s: %w", d, err)
		}
	}

	base := filepath.Base(abs)
	name := base
	var infoPath string
	for n := 1; ; n++ {
		if n > 1 {
			name = fmt.Sprintf("%s.%d", base, n)
		}
		infoPath = filepath.Join(info, name+".trashinfo")
		f, err := os.OpenFile(infoPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return fmt.Errorf("could not write %s: %w", infoPath, err)
		}
		_, werr := fmt.Fprintf(f, "[Trash Info]\nPath=%s\nDeletionDate=%s\n",
			escapePath(abs), time.Now().Format("2006-01-02T15:04:05"))
		f.Close()
		if werr != nil {
			os.Remove(infoPath)
			return fmt.Errorf("could not write %s: %w", infoPath, werr)
		}
		break
	}

	if err := os.Rename(abs, filepath.Join(files, name)); err != nil {
		os.Remove(infoPath)
		return fmt.Errorf("could not move %s to trash (is it on another filesystem?): %w", abs, err)
	}
	return nil
}
