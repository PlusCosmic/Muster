package main

// Self-update inside an AppImage.
//
// Wails' updater replaces os.Executable() and re-executes it as the swap
// helper. Inside an AppImage both are wrong: the executable is a file in a
// read-only FUSE mount that vanishes when the app exits, and what needs
// replacing is the AppImage file itself ($APPIMAGE, set by its runtime).
// The updater has no hook for either, so this file wraps its helper protocol
// around it. The updater still checks, downloads and verifies; then:
//
//   - stageBesideAppImage copies the verified artifact next to the AppImage.
//     The helper swaps with a rename, which fails across filesystems, and
//     the updater stages under os.TempDir, which usually is one.
//   - launchAppImageHelper runs the AppImage file as the helper with the
//     updater's environment protocol: target $APPIMAGE, new = the staged
//     copy. Its runtime mounts it independently of the exiting parent.
//     application.New in that process calls updater.HandleHelperMode, which
//     waits for the parent to exit, swaps the file, restores the executable
//     bit and relaunches it.
//
// The env variable names are the updater's (pkg/updater/helper.go), where
// they are unexported.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	helperModeEnv   = "WAILS_UPDATER_HELPER"
	helperTargetEnv = "WAILS_UPDATER_HELPER_TARGET"
	helperNewEnv    = "WAILS_UPDATER_HELPER_NEW"
	helperPIDEnv    = "WAILS_UPDATER_HELPER_PID"
	helperLogEnv    = "WAILS_UPDATER_HELPER_LOG"
)

// appImagePath is the AppImage this process runs from, or "" when it is not
// one. The type-2 runtime exports the absolute path as APPIMAGE.
func appImagePath() string {
	p := os.Getenv("APPIMAGE")
	if p == "" || !filepath.IsAbs(p) {
		return ""
	}
	if fi, err := os.Stat(p); err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	return p
}

// neutraliseMountedHelper exits a helper the updater itself spawned from
// inside a mount: its target is the read-only mounted binary, so its swap
// can only fail and its restore could only relaunch a path that is about to
// disappear. Only launchAppImageHelper's helper, whose target is the AppImage
// file, is allowed to proceed. Called before application.New, which runs the
// helper.
func neutraliseMountedHelper() {
	if os.Getenv(helperModeEnv) != "1" {
		return
	}
	appImage := os.Getenv("APPIMAGE")
	if appImage == "" {
		return
	}
	if target := os.Getenv(helperTargetEnv); target != appImage {
		os.Exit(0)
	}
}

// stageBesideAppImage copies the verified artifact into a fresh directory
// next to the AppImage and returns the copy's path. The directory is named
// so the updater's helper removes it after the swap (it cleans any
// `wails-update-*` parent of the new file).
func stageBesideAppImage(staged, appImage string) (string, error) {
	dir, err := os.MkdirTemp(filepath.Dir(appImage), "wails-update-")
	if err != nil {
		return "", fmt.Errorf("the AppImage's directory is not writable: %w", err)
	}
	dst := filepath.Join(dir, filepath.Base(appImage))
	if err := copyFile(staged, dst, 0o755); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dst, nil
}

func copyFile(src, dst string, mode os.FileMode) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// launchAppImageHelper starts the AppImage as a detached swap helper for the
// current process. The caller quits afterwards; the helper waits for that.
func launchAppImageHelper(appImage, staged string) error {
	if !strings.HasPrefix(filepath.Base(filepath.Dir(staged)), "wails-update-") {
		return errors.New("staged update is not beside the AppImage")
	}
	logPath := filepath.Join(os.TempDir(), fmt.Sprintf("muster-update-%d.log", os.Getpid()))
	cmd := exec.Command(appImage)
	cmd.Env = append(os.Environ(),
		helperModeEnv+"=1",
		helperTargetEnv+"="+appImage,
		helperNewEnv+"="+staged,
		helperPIDEnv+"="+fmt.Sprint(os.Getpid()),
		helperLogEnv+"="+logPath,
	)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	// Its own session, so it outlives this process and its process group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}
