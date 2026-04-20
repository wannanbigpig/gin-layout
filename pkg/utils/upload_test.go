package utils

import (
	"bytes"
	"errors"
	"io"
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
