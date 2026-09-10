//go:build !linux

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// AppImages are a Linux format; see appimage_linux.go. These keep main.go
// free of build tags.

func appImagePath() string { return "" }

func neutraliseMountedHelper() {}

func offerAppImageRestart(*application.App, string) {}
