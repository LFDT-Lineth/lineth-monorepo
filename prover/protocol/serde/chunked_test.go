package serde

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPayload is a simple serializable struct for round-trip testing.
// It uses only POD types that the serde package can serialize without
// needing interface registrations.
type testPayload struct {
	A int64
	B [4]uint64
	C float64
}

// TestChunkedRoundTrip_SmallPayload verifies Store→Load round-trip for a
// payload smaller than one chunk.
func TestChunkedRoundTrip_SmallPayload(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "small")

	original := testPayload{A: 42, B: [4]uint64{1, 2, 3, 4}, C: 3.14}

	err := StoreChunked(basePath, original)
	require.NoError(t, err)

	// Manifest and exactly one chunk file should exist
	require.True(t, HasChunkedAsset(basePath))
	assertChunkFilesExist(t, basePath, 1)

	var loaded testPayload
	buf, err := LoadChunkedMmapBacked(basePath, &loaded)
	require.NoError(t, err)
	defer buf.Release()

	assert.Equal(t, original.A, loaded.A)
	assert.Equal(t, original.B, loaded.B)
	assert.Equal(t, original.C, loaded.C)
}

// TestChunkedRoundTrip_MultiChunk verifies Store→Load for data that spans
// multiple chunks.
func TestChunkedRoundTrip_MultiChunk(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "multi")

	// Create a large byte array that will span multiple chunks.
	// Use a struct wrapping a large slice-like array.
	type bigPayload struct {
		Tag  uint64
		Data [512]uint64
	}
	original := bigPayload{Tag: 0xDEADBEEF}
	for i := range original.Data {
		original.Data[i] = uint64(i * 7)
	}

	err := StoreChunked(basePath, original)
	require.NoError(t, err)
	require.True(t, HasChunkedAsset(basePath))

	var loaded bigPayload
	buf, err := LoadChunkedMmapBacked(basePath, &loaded)
	require.NoError(t, err)
	defer buf.Release()

	assert.Equal(t, original.Tag, loaded.Tag)
	assert.Equal(t, original.Data, loaded.Data)
}

// TestChunkedRoundTrip_WithSliceData tests a struct containing a slice, which
// exercises the serde's pointer/slice relocation logic across chunk boundaries.
func TestChunkedRoundTrip_WithSliceData(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "slice")

	type slicePayload struct {
		ID    int32
		Items []uint64
	}

	original := slicePayload{
		ID:    99,
		Items: make([]uint64, 1000),
	}
	for i := range original.Items {
		original.Items[i] = uint64(i * 3)
	}

	err := StoreChunked(basePath, original)
	require.NoError(t, err)

	var loaded slicePayload
	buf, err := LoadChunkedMmapBacked(basePath, &loaded)
	require.NoError(t, err)
	defer buf.Release()

	assert.Equal(t, original.ID, loaded.ID)
	require.Len(t, loaded.Items, len(original.Items))
	for i := range original.Items {
		assert.Equal(t, original.Items[i], loaded.Items[i], "mismatch at index %d", i)
	}
}

// TestChunkedManifest_Corrupt verifies that loading rejects a truncated or
// invalid manifest.
func TestChunkedManifest_Corrupt(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "corrupt")

	// Write a too-small manifest
	err := os.WriteFile(basePath+".chunked", []byte{0x01, 0x02}, 0600)
	require.NoError(t, err)

	var dummy testPayload
	_, err = LoadChunkedMmapBacked(basePath, &dummy)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "manifest")
}

// TestChunkedManifest_BadMagic verifies rejection of a manifest with wrong magic.
func TestChunkedManifest_BadMagic(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "badmagic")

	// Write a manifest with correct size but wrong magic
	buf := make([]byte, 16)
	buf[0] = 0xFF // wrong magic
	err := os.WriteFile(basePath+".chunked", buf, 0600)
	require.NoError(t, err)

	var dummy testPayload
	_, err = LoadChunkedMmapBacked(basePath, &dummy)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "magic")
}

// TestChunkedMissingChunkFile verifies that a missing chunk file produces a
// clear read error.
func TestChunkedMissingChunkFile(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "missing")

	original := testPayload{A: 1}
	err := StoreChunked(basePath, original)
	require.NoError(t, err)

	// Delete the first chunk file
	err = os.Remove(filepath.Join(basePath, "chunk-0000.lz4"))
	require.NoError(t, err)

	var loaded testPayload
	_, err = LoadChunkedMmapBacked(basePath, &loaded)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "chunk 0")
}

// TestHasChunkedAsset_NonExistent verifies HasChunkedAsset returns false when
// no manifest exists.
func TestHasChunkedAsset_NonExistent(t *testing.T) {
	assert.False(t, HasChunkedAsset("/nonexistent/path/asset"))
}

// TestChunkedRoundTrip_Incompressible verifies that random (incompressible)
// data round-trips correctly via the raw-storage fallback path.
func TestChunkedRoundTrip_Incompressible(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "random")

	// Fill a fixed-size array with random bytes (incompressible).
	type randomPayload struct {
		Data [4096]byte
	}
	var original randomPayload
	_, err := rand.Read(original.Data[:])
	require.NoError(t, err)

	err = StoreChunked(basePath, original)
	require.NoError(t, err)

	var loaded randomPayload
	buf, err := LoadChunkedMmapBacked(basePath, &loaded)
	require.NoError(t, err)
	defer buf.Release()

	assert.Equal(t, original.Data, loaded.Data)
}

// assertChunkFilesExist checks that at least minChunks chunk files exist
// inside the chunked asset subdirectory.
func assertChunkFilesExist(t *testing.T, basePath string, minChunks int) {
	t.Helper()
	for i := 0; i < minChunks; i++ {
		p := filepath.Join(basePath, fmt.Sprintf("chunk-%04d.lz4", i))
		_, err := os.Stat(p)
		assert.NoError(t, err, "expected chunk file %s to exist", p)
	}
}

// TestChunkedRoundTrip_ForcedMultiChunk uses a tiny chunk size to force the
// payload into multiple chunks, exercising parallel decompress and reassembly.
func TestChunkedRoundTrip_ForcedMultiChunk(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "forced")

	type largePayload struct {
		Data [2048]uint64 // 16 KB of data
	}
	var original largePayload
	for i := range original.Data {
		original.Data[i] = uint64(i)
	}

	// Use 1 KB chunk size → ~16 chunks for a 16 KB payload
	err := StoreChunkedWithSize(basePath, original, 1024)
	require.NoError(t, err)
	require.True(t, HasChunkedAsset(basePath))

	// Verify multiple chunk files were written
	assertChunkFilesExist(t, basePath, 4)

	var loaded largePayload
	buf, err := LoadChunkedMmapBacked(basePath, &loaded)
	require.NoError(t, err)
	defer buf.Release()

	assert.Equal(t, original.Data, loaded.Data)
}

// TestChunkedRoundTrip_ForcedMultiChunk_WithSlices exercises multi-chunk with
// pointer-bearing types (slices) to verify serde relocation across chunks.
func TestChunkedRoundTrip_ForcedMultiChunk_WithSlices(t *testing.T) {
	dir := t.TempDir()
	basePath := filepath.Join(dir, "slicemulti")

	type payload struct {
		Tag  uint32
		Vals []int64
	}
	original := payload{
		Tag:  0xCAFE,
		Vals: make([]int64, 5000), // ~40 KB backing array
	}
	for i := range original.Vals {
		original.Vals[i] = int64(i * 11)
	}

	err := StoreChunkedWithSize(basePath, original, 4096) // 4 KB chunks
	require.NoError(t, err)

	var loaded payload
	buf, err := LoadChunkedMmapBacked(basePath, &loaded)
	require.NoError(t, err)
	defer buf.Release()

	assert.Equal(t, original.Tag, loaded.Tag)
	require.Len(t, loaded.Vals, len(original.Vals))
	for i := range original.Vals {
		if original.Vals[i] != loaded.Vals[i] {
			t.Fatalf("mismatch at index %d: want %d, got %d", i, original.Vals[i], loaded.Vals[i])
		}
	}
}

// BenchmarkChunkedRoundTrip_1MB benchmarks Store+Load for ~1 MB of data.
func BenchmarkChunkedRoundTrip_1MB(b *testing.B) {
	dir := b.TempDir()
	type payload struct {
		Data [131072]uint64 // 1 MB
	}
	var original payload
	for i := range original.Data {
		original.Data[i] = uint64(i)
	}

	for b.Loop() {
		basePath := filepath.Join(dir, fmt.Sprintf("bench-%d", b.N))
		if err := StoreChunked(basePath, original); err != nil {
			b.Fatal(err)
		}
		var loaded payload
		buf, err := LoadChunkedMmapBacked(basePath, &loaded)
		if err != nil {
			b.Fatal(err)
		}
		buf.Release()
	}
}
