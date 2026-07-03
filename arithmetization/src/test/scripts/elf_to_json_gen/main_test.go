package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestParseInBytesSSZFileIsRawPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.ssz")
	raw := []byte{0xaa, 0xbb, 0xcc}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	input, err := parseInBytes("@" + path)
	if err != nil {
		t.Fatal(err)
	}

	if !input.ssz {
		t.Fatal("expected SSZ input")
	}
	if !bytes.Equal(input.data, raw) {
		t.Fatalf("unexpected SSZ payload: got %x want %x", input.data, raw)
	}
}

func TestSSZInputBlobs(t *testing.T) {
	blobs := sszInputBlobs(0x1000, []byte{0xaa, 0xbb, 0xcc})

	if len(blobs) != 2 {
		t.Fatalf("expected 2 blobs, got %d", len(blobs))
	}
	if blobs[0].offset != 0x1000 {
		t.Fatalf("unexpected prefix offset: got %#x", blobs[0].offset)
	}
	if !bytes.Equal(blobs[0].data, []byte{3, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("unexpected prefix bytes: got %x", blobs[0].data)
	}
	if blobs[1].offset != 0x1008 {
		t.Fatalf("unexpected payload offset: got %#x", blobs[1].offset)
	}
	if !bytes.Equal(blobs[1].data, []byte{0xaa, 0xbb, 0xcc}) {
		t.Fatalf("unexpected payload bytes: got %x", blobs[1].data)
	}
}

func TestSSZInputBlobsEmptyPayload(t *testing.T) {
	blobs := sszInputBlobs(0x1000, nil)

	if len(blobs) != 1 {
		t.Fatalf("expected only the length blob, got %d blobs", len(blobs))
	}
	if !bytes.Equal(blobs[0].data, []byte{0, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("unexpected prefix bytes: got %x", blobs[0].data)
	}
}
