package main

import (
	"bytes"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
)

func TestResolveConfigPathWithFlagValue(t *testing.T) {
	got := resolveConfigPath("custom/config.yaml")
	if got != "custom/config.yaml" {
		t.Fatalf("配置路径错误: got=%q want=%q", got, "custom/config.yaml")
	}
}

func TestResolveConfigPathUseExecutableDirConfig(t *testing.T) {
	oldExecutablePath := executablePath
	oldFileStat := fileStat
	t.Cleanup(func() {
		executablePath = oldExecutablePath
		fileStat = oldFileStat
	})

	executablePath = func() (string, error) {
		return filepath.Join("/tmp", "app", "geoloom"), nil
	}
	fileStat = func(name string) (fs.FileInfo, error) {
		if name == filepath.Join("/tmp", "app", "config.yaml") {
			return nil, nil
		}
		return nil, fs.ErrNotExist
	}

	got := resolveConfigPath("")
	want := filepath.Join("/tmp", "app", "config.yaml")
	if got != want {
		t.Fatalf("配置路径错误: got=%q want=%q", got, want)
	}
}

func TestResolveConfigPathFallbackToConfigs(t *testing.T) {
	oldExecutablePath := executablePath
	oldFileStat := fileStat
	t.Cleanup(func() {
		executablePath = oldExecutablePath
		fileStat = oldFileStat
	})

	executablePath = func() (string, error) {
		return "", errors.New("boom")
	}
	fileStat = func(name string) (fs.FileInfo, error) {
		return nil, fs.ErrNotExist
	}

	got := resolveConfigPath("")
	want := filepath.Join("configs", "config.yaml")
	if got != want {
		t.Fatalf("配置路径错误: got=%q want=%q", got, want)
	}
}

func TestPrintVersion(t *testing.T) {
	oldVersion := Version
	oldCommit := Commit
	oldBuildTime := BuildTime
	t.Cleanup(func() {
		Version = oldVersion
		Commit = oldCommit
		BuildTime = oldBuildTime
	})

	Version = "1.2.3"
	Commit = "abc123"
	BuildTime = "2026-03-05T16:00:00Z"

	buf := bytes.NewBuffer(nil)
	printVersion(buf)

	got := buf.String()
	want := "GeoLoom version=1.2.3 commit=abc123 build_time=2026-03-05T16:00:00Z\n"
	if got != want {
		t.Fatalf("版本输出错误: got=%q want=%q", got, want)
	}
}
