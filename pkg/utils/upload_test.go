package utils

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

func TestIsAllowedImageHandlesShortNonImageFile(t *testing.T) {
	file := bytes.NewReader([]byte("x"))

	ext, allowed, err := IsAllowedImage(file)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if allowed {
		t.Fatalf("expected non-image short file to be rejected")
	}
	if ext != "" {
		t.Fatalf("expected empty extension, got %q", ext)
	}
}

func TestAppendHeaderSampleRespectsLimit(t *testing.T) {
	header := make([]byte, 0, fileHeaderSampleSize)
	chunk := bytes.Repeat([]byte("a"), fileHeaderSampleSize+10)

	header = appendHeaderSample(header, chunk)
	if len(header) != fileHeaderSampleSize {
		t.Fatalf("expected header size %d, got %d", fileHeaderSampleSize, len(header))
	}

	header = appendHeaderSample(header, []byte("b"))
	if len(header) != fileHeaderSampleSize {
		t.Fatalf("expected header size to stay %d, got %d", fileHeaderSampleSize, len(header))
	}
}

func TestShouldStopUploadRead(t *testing.T) {
	done, err := shouldStopUploadRead(nil)
	if done || err != nil {
		t.Fatalf("expected continue reading, got done=%v err=%v", done, err)
	}

	done, err = shouldStopUploadRead(io.EOF)
	if !done || err != nil {
		t.Fatalf("expected eof to finish cleanly, got done=%v err=%v", done, err)
	}

	wantErr := errors.New("read failed")
	done, err = shouldStopUploadRead(wantErr)
	if !done || !errors.Is(err, wantErr) {
		t.Fatalf("expected done with original err, got done=%v err=%v", done, err)
	}
}

func TestSaveUploadedImageWithUUIDAllowsWebP(t *testing.T) {
	header := newUtilsMultipartFileHeader(t, "avatar.gif", testWebPContent())

	info, err := SaveUploadedImageWithUUID(header, t.TempDir())
	if err != nil {
		t.Fatalf("expected webp image to be saved, got %v", err)
	}
	if info.Ext != ".webp" {
		t.Fatalf("expected detected extension .webp, got %q", info.Ext)
	}
	if info.MimeType != "image/webp" {
		t.Fatalf("expected image/webp mime type, got %q", info.MimeType)
	}
	if !strings.HasSuffix(info.Name, ".webp") {
		t.Fatalf("expected saved filename to use .webp, got %q", info.Name)
	}
}

func TestSaveUploadedImageWithUUIDKeepsGIFCompatibility(t *testing.T) {
	header := newUtilsMultipartFileHeader(t, "avatar.gif", []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00"))

	info, err := SaveUploadedImageWithUUID(header, t.TempDir())
	if err != nil {
		t.Fatalf("expected gif image to be saved, got %v", err)
	}
	if info.Ext != ".gif" || info.MimeType != "image/gif" {
		t.Fatalf("expected detected gif fields, got ext=%q mime=%q", info.Ext, info.MimeType)
	}
}

func newUtilsMultipartFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create multipart file failed: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write multipart file failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, "/", body)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(int64(len(content) + 1024)); err != nil {
		t.Fatalf("parse multipart form failed: %v", err)
	}
	return req.MultipartForm.File["file"][0]
}

func testWebPContent() []byte {
	return []byte{'R', 'I', 'F', 'F', 0x10, 0x00, 0x00, 0x00, 'W', 'E', 'B', 'P', 'V', 'P', '8', ' ', 0x00, 0x00, 0x00, 0x00}
}
