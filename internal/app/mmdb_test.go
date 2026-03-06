package app

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"testing"

	"geoloom/internal/config"
)

type mockReadCloser struct{}

func (m *mockReadCloser) Read(p []byte) (int, error) {
	copy(p, []byte("mock-mmdb"))
	return len("mock-mmdb"), io.EOF
}

func (m *mockReadCloser) Close() error {
	return nil
}

func TestPrepareMMDBPathWithExplicitPath(t *testing.T) {
	cfg := config.GeoConfig{MMDBPath: "relative.mmdb"}
	got, err := prepareMMDBPath(context.Background(), cfg, "/tmp/conf/config.yaml")
	if err != nil {
		t.Fatalf("prepareMMDBPath 返回错误: %v", err)
	}
	want := filepath.Clean("/tmp/conf/relative.mmdb")
	if got != want {
		t.Fatalf("路径错误: got=%q want=%q", got, want)
	}
}

func TestPrepareMMDBPathUseExecutableDefaultFile(t *testing.T) {
	oldExecutable := osExecutable
	oldStat := osStat
	t.Cleanup(func() {
		osExecutable = oldExecutable
		osStat = oldStat
	})

	osExecutable = func() (string, error) {
		return filepath.Join("/tmp", "app", "geoloom"), nil
	}
	osStat = func(name string) (fs.FileInfo, error) {
		if name == filepath.Join("/tmp", "app", defaultMMDBFileName) {
			return nil, nil
		}
		return nil, fs.ErrNotExist
	}

	got, err := prepareMMDBPath(context.Background(), config.GeoConfig{}, "")
	if err != nil {
		t.Fatalf("prepareMMDBPath 返回错误: %v", err)
	}
	want := filepath.Join("/tmp", "app", defaultMMDBFileName)
	if got != want {
		t.Fatalf("路径错误: got=%q want=%q", got, want)
	}
}

func TestPrepareMMDBPathDownloadWhenMissing(t *testing.T) {
	oldExecutable := osExecutable
	oldStat := osStat
	oldHTTPDo := httpDo
	oldWriteFile := osWriteFile
	t.Cleanup(func() {
		osExecutable = oldExecutable
		osStat = oldStat
		httpDo = oldHTTPDo
		osWriteFile = oldWriteFile
	})

	osExecutable = func() (string, error) {
		return filepath.Join("/tmp", "app", "geoloom"), nil
	}
	osStat = func(name string) (fs.FileInfo, error) {
		return nil, fs.ErrNotExist
	}

	wrotePath := ""
	osWriteFile = func(name string, data []byte, perm fs.FileMode) error {
		wrotePath = name
		return nil
	}

	httpDo = func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       &mockReadCloser{},
		}, nil
	}

	got, err := prepareMMDBPath(context.Background(), config.GeoConfig{MMDBURL: "https://example.com/mmdb"}, "")
	if err != nil {
		t.Fatalf("prepareMMDBPath 返回错误: %v", err)
	}
	want := filepath.Join("/tmp", "app", defaultMMDBFileName)
	if got != want {
		t.Fatalf("路径错误: got=%q want=%q", got, want)
	}
	if wrotePath != want {
		t.Fatalf("写入路径错误: got=%q want=%q", wrotePath, want)
	}
}

func TestPrepareMMDBPathNoPathNoURL(t *testing.T) {
	oldExecutable := osExecutable
	oldStat := osStat
	t.Cleanup(func() {
		osExecutable = oldExecutable
		osStat = oldStat
	})

	osExecutable = func() (string, error) {
		return filepath.Join("/tmp", "app", "geoloom"), nil
	}
	osStat = func(name string) (fs.FileInfo, error) {
		return nil, fs.ErrNotExist
	}

	got, err := prepareMMDBPath(context.Background(), config.GeoConfig{}, "")
	if err != nil {
		t.Fatalf("prepareMMDBPath 返回错误: %v", err)
	}
	if got != "" {
		t.Fatalf("路径错误: got=%q want=%q", got, "")
	}
}

func TestPrepareMMDBPathExecutableError(t *testing.T) {
	oldExecutable := osExecutable
	t.Cleanup(func() {
		osExecutable = oldExecutable
	})

	osExecutable = func() (string, error) {
		return "", errors.New("boom")
	}

	_, err := prepareMMDBPath(context.Background(), config.GeoConfig{MMDBURL: "https://example.com/mmdb"}, "")
	if err == nil {
		t.Fatal("预期返回错误，但得到 nil")
	}
}
