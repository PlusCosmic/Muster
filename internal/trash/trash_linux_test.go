package trash

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exercises the freedesktop fallback against a scratch XDG_DATA_HOME so the
// real trash is never touched.
func TestHomeTrashMovesDirectoryAndWritesInfo(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	victim := filepath.Join(root, "victim dir")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(victim, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := homeTrash(victim); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(victim); !os.IsNotExist(err) {
		t.Fatal("victim should be gone")
	}
	if _, err := os.Stat(filepath.Join(root, "Trash", "files", "victim dir", "a.txt")); err != nil {
		t.Fatalf("contents should be in the trash: %v", err)
	}
	info, err := os.ReadFile(filepath.Join(root, "Trash", "info", "victim dir.trashinfo"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(info), "Path="+escapePath(victim)) {
		t.Fatalf("unexpected trashinfo:\n%s", info)
	}

	// A second item with the same name gets a numbered slot.
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := homeTrash(victim); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Trash", "info", "victim dir.2.trashinfo")); err != nil {
		t.Fatalf("second entry should be numbered: %v", err)
	}
}
