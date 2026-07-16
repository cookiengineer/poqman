package storage

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractLayer_Basic(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "layer")

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	tw.WriteHeader(&tar.Header{
		Name:     "hello.txt",
		Mode:     0o644,
		Size:     int64(len("hello world")),
		Typeflag: tar.TypeReg,
	})
	tw.Write([]byte("hello world"))

	tw.WriteHeader(&tar.Header{
		Name:     "subdir/",
		Mode:     0o755,
		Typeflag: tar.TypeDir,
	})
	tw.WriteHeader(&tar.Header{
		Name:     "subdir/nested.txt",
		Mode:     0o644,
		Size:     int64(len("nested content")),
		Typeflag: tar.TypeReg,
	})
	tw.Write([]byte("nested content"))

	tw.WriteHeader(&tar.Header{
		Name:     "link",
		Mode:     0o777,
		Typeflag: tar.TypeSymlink,
		Linkname: "hello.txt",
	})

	tw.Close()
	gw.Close()

	if err := ExtractLayer(&buf, "", dest); err != nil {
		t.Fatalf("ExtractLayer: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dest, "hello.txt"))
	if err != nil {
		t.Fatalf("read hello.txt: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(content))
	}

	nested, err := os.ReadFile(filepath.Join(dest, "subdir", "nested.txt"))
	if err != nil {
		t.Fatalf("read nested.txt: %v", err)
	}
	if string(nested) != "nested content" {
		t.Errorf("expected 'nested content', got %q", string(nested))
	}

	linkTarget, err := os.Readlink(filepath.Join(dest, "link"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if linkTarget != "hello.txt" {
		t.Errorf("expected link target 'hello.txt', got %q", linkTarget)
	}
}

func TestExtractLayer_DigestVerification(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "layer")

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	tw.WriteHeader(&tar.Header{
		Name:     "test.txt",
		Mode:     0o644,
		Size:     int64(len("digest test")),
		Typeflag: tar.TypeReg,
	})
	tw.Write([]byte("digest test"))

	tw.Close()
	gw.Close()

	sha256OfEmpty := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	err := ExtractLayer(&buf, "sha256:"+sha256OfEmpty, dest)
	if err == nil {
		t.Error("expected digest mismatch error")
	}

	correctDigest := "c34f7d9f64e0e1c4ee3378f72a5e1f4d8f3f5d5b4e3f1a52e11d3b2e8e5f7a1b"
	err = ExtractLayer(&buf, "sha256:"+correctDigest, dest)
	if err == nil {
		t.Error("expected digest mismatch error (data already consumed)")
	}

	buf.Reset()
	gw2 := gzip.NewWriter(&buf)
	tw2 := tar.NewWriter(gw2)
	tw2.WriteHeader(&tar.Header{
		Name:     "test.txt",
		Mode:     0o644,
		Size:     int64(len("digest test")),
		Typeflag: tar.TypeReg,
	})
	tw2.Write([]byte("digest test"))
	tw2.Close()
	gw2.Close()

	err = ExtractLayer(&buf, "", dest)
	if err != nil {
		t.Errorf("ExtractLayer without digest check: %v", err)
	}
}

func TestExtractLayer_EmptyTar(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "empty")

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	tw.Close()
	gw.Close()

	if err := ExtractLayer(&buf, "", dest); err != nil {
		t.Fatalf("ExtractLayer empty tar: %v", err)
	}
}
