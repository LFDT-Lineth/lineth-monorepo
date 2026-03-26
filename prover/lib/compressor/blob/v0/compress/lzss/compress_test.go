package lzss

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/icza/bitio"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func testCompressionRoundTrip(t *testing.T, d []byte) {
	compressor, err := NewCompressor(getDictionary(), BestCompression)
	require.NoError(t, err)

	c, err := compressor.Compress(d)
	require.NoError(t, err)

	dBack, err := Decompress(c, getDictionary())
	require.NoError(t, err)

	if !bytes.Equal(d, dBack) {
		t.Fatal("round trip failed")
	}
}

func Test8Zeros(t *testing.T) {
	testCompressionRoundTrip(t, []byte{0, 0, 0, 0, 0, 0, 0, 0})
}

func Test300Zeros(t *testing.T) { // probably won't happen in our calldata
	testCompressionRoundTrip(t, make([]byte, 300))
}

func TestNoCompression(t *testing.T) {
	testCompressionRoundTrip(t, []byte{'h', 'i'})
}

func TestNoCompressionAttempt(t *testing.T) {

	d := []byte{253, 254, 255}

	compressor, err := NewCompressor(getDictionary(), NoCompression)
	require.NoError(t, err)

	c, err := compressor.Compress(d)
	require.NoError(t, err)

	dBack, err := Decompress(c, getDictionary())
	require.NoError(t, err)

	if !bytes.Equal(d, dBack) {
		t.Fatal("round trip failed")
	}
}

func Test9E(t *testing.T) {
	testCompressionRoundTrip(t, []byte{1, 1, 1, 1, 2, 1, 1, 1, 1})
}

func Test8ZerosAfterNonzero(t *testing.T) { // probably won't happen in our calldata
	testCompressionRoundTrip(t, append([]byte{1}, make([]byte, 8)...))
}

// Fuzz test the compression / decompression
func FuzzCompress(f *testing.F) {

	f.Fuzz(func(t *testing.T, input, dict []byte, cMode uint8) {
		if len(input) > MaxInputSize {
			t.Skip("input too large")
		}
		if len(dict) > MaxDictSize {
			t.Skip("dict too large")
		}
		var level Level
		if cMode&2 == 2 {
			level = 2
		} else if cMode&4 == 4 {
			level = 4
		} else if cMode&8 == 8 {
			level = 8
		} else {
			level = BestCompression
		}

		checkDecompressResult := func(compressedBytes []byte) {
			decompressedBytes, err := Decompress(compressedBytes, dict)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(input, decompressedBytes) {
				t.Log("compression level:", level)
				t.Log("original bytes:", hex.EncodeToString(input))
				t.Log("decompressed bytes:", hex.EncodeToString(decompressedBytes))
				t.Log("dict", hex.EncodeToString(dict))
				t.Fatal("decompressed bytes are not equal to original bytes")
			}
		}

		// test compress (i.e write all the bytes)
		compressor, err := NewCompressor(dict, level)
		if err != nil {
			t.Fatal(err)
		}
		compressedBytes, err := compressor.Compress(input)
		if err != nil {
			t.Fatal(err)
		}

		checkDecompressResult(compressedBytes)

		// test write byte by byte
		compressor, err = NewCompressor(dict, level)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range input {
			if _, err := compressor.Write([]byte{b}); err != nil {
				t.Fatal(err)
			}
		}
		checkDecompressResult(compressor.Bytes())

		// test write byte by byte with revert
		compressor, err = NewCompressor(dict, level)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range input {
			if _, err := compressor.Write([]byte{b}); err != nil {
				t.Fatal(err)
			}
			if err := compressor.Revert(); err != nil {
				t.Fatal(err)
			}
		}

		// test write byte by byte with revert and write again
		compressor, err = NewCompressor(dict, level)
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range input {
			if _, err := compressor.Write([]byte{b}); err != nil {
				t.Fatal(err)
			}
			if err := compressor.Revert(); err != nil {
				t.Fatal(err)
			}
			if _, err := compressor.Write([]byte{b}); err != nil {
				t.Fatal(err)
			}
		}
		checkDecompressResult(compressor.Bytes())

		// Write after Reset should be the same as Write after NewCompressor
		compressor.Reset()

		if _, err := compressor.Write(input); err != nil {
			t.Fatal(err)
		}
		checkDecompressResult(compressor.Bytes())

		if len(input) > 1 {
			compressor.Reset()

			// write all but the last byte
			if _, err := compressor.Write(input[:len(input)-1]); err != nil {
				t.Fatal(err)
			}
			// write the last byte
			if _, err := compressor.Write([]byte{input[len(input)-1]}); err != nil {
				t.Fatal(err)
			}
			checkDecompressResult(compressor.Bytes())

			compressor.Reset()
			// write the first byte
			if _, err := compressor.Write([]byte{input[0]}); err != nil {
				t.Fatal(err)
			}
			// write the rest
			if _, err := compressor.Write(input[1:]); err != nil {
				t.Fatal(err)
			}
			checkDecompressResult(compressor.Bytes())
		}

	})
}

func FuzzCompressedSize(f *testing.F) {

	f.Fuzz(func(t *testing.T, input, dict []byte, cMode uint8) {
		const maxInputSize = 1 << 18 // 256KB
		if len(input) > maxInputSize {
			t.Skip("input too large")
		}
		if len(dict) > MaxDictSize {
			t.Skip("dict too large")
		}
		var level Level
		if cMode&2 == 2 {
			level = 2
		} else if cMode&4 == 4 {
			level = 4
		} else if cMode&8 == 8 {
			level = 8
		} else {
			level = BestCompression
		}

		compressor, err := NewCompressor(dict, level)
		if err != nil {
			t.Fatal(err)
		}

		compressed, err := compressor.Compress(input)
		if err != nil {
			t.Fatal(err)
		}

		n, err := compressor.CompressedSize256k(input)
		if err != nil {
			t.Fatal(err)
		}

		if n != len(compressed) {
			t.Fatal("CompressedSize256k returned wrong size")
		}

	})

}

func Test300ZerosAfterNonzero(t *testing.T) { // probably won't happen in our calldata
	testCompressionRoundTrip(t, append([]byte{'h', 'i'}, make([]byte, 300)...))
}

func TestConstantedNonzero(t *testing.T) {
	testCompressionRoundTrip(t, []byte{'h', 'i', 'h', 'i', 'h', 'i'})
}

func TestAverageBatch(t *testing.T) {
	assert := require.New(t)

	// read "average_block.hex" file
	d, err := os.ReadFile("./testdata/average_block.hex")
	assert.NoError(err)

	// convert to bytes
	data, err := hex.DecodeString(string(d))
	assert.NoError(err)

	dict := getDictionary()
	compressor, err := NewCompressor(dict, BestCompression)
	assert.NoError(err)

	lzssRes, err := compresslzss_v1(compressor, data)
	assert.NoError(err)

	fmt.Println("lzss compression ratio:", lzssRes.ratio)

	lzssDecompressed, err := decompresslzss_v1(lzssRes.compressed, dict)
	assert.NoError(err)
	assert.True(bytes.Equal(data, lzssDecompressed))

}

func BenchmarkAverageBatch(b *testing.B) {
	// read the file
	d, err := os.ReadFile("./testdata/average_block.hex")
	if err != nil {
		b.Fatal(err)
	}

	// convert to bytes
	data, err := hex.DecodeString(string(d))
	if err != nil {
		b.Fatal(err)
	}

	dict := getDictionary()

	compressor, err := NewCompressor(dict, BestCompression)
	if err != nil {
		b.Fatal(err)
	}

	// benchmark lzss
	b.Run("lzss", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := compressor.Compress(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

type compressResult struct {
	compressed []byte
	inputSize  int
	outputSize int
	ratio      float64
}

func decompresslzss_v1(data, dict []byte) ([]byte, error) {
	return Decompress(data, dict)
}

func compresslzss_v1(compressor *Compressor, data []byte) (compressResult, error) {
	c, err := compressor.Compress(data)
	if err != nil {
		return compressResult{}, err
	}
	return compressResult{
		compressed: c,
		inputSize:  len(data),
		outputSize: len(c),
		ratio:      float64(len(data)) / float64(len(c)),
	}, nil
}

func getDictionary() []byte {
	d, err := os.ReadFile("./testdata/dict_naive")
	if err != nil {
		panic(err)
	}
	return d
}

func TestRevert(t *testing.T) {
	assert := require.New(t)

	// read the file
	d, err := os.ReadFile("./testdata/average_block.hex")
	assert.NoError(err)

	// convert to bytes
	data, err := hex.DecodeString(string(d))
	assert.NoError(err)

	dict := getDictionary()
	compressor, err := NewCompressor(dict, BestCompression)
	assert.NoError(err)

	const (
		inChunkSize = 1000
		outMaxSize  = 5000
	)

	for i0 := 0; i0 < len(data); {

		i := i0
		for ; i < len(data) && compressor.Len() < outMaxSize; i += inChunkSize {
			_, err = compressor.Write(data[i:min(i+inChunkSize, len(data))])
			assert.NoError(err)
			if uncompressedSize := i + inChunkSize - i0 + 3; compressor.Len() >= outMaxSize &&
				uncompressedSize <= outMaxSize &&
				compressor.Len() > uncompressedSize {
				assert.True(compressor.ConsiderBypassing())
			}
		}

		if compressor.Len() > outMaxSize {
			assert.NoError(compressor.Revert())
			i -= inChunkSize
		}

		c := compressor.Bytes()
		dBack, err := Decompress(c, dict)
		assert.NoError(err)
		assert.Equal(data[i0:min(i, len(data))], dBack, i0)

		compressor.Reset()
		i0 = i
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestInvalidBackref(t *testing.T) {
	data := make([]byte, 100)
	for i := 0; i < 32; i++ {
		data[i] = 1
	}
	for i := 32; i < 64; i++ {
		data[i] = 2
	}
	for i := 64; i < 100; i++ {
		data[i] = 3
	}
	assert := require.New(t)

	compressor, err := NewCompressor([]byte{}, BestCompression)
	assert.NoError(err)

	c, err := compressor.Compress(data)
	assert.NoError(err)

	_, err = Decompress(c, []byte{})
	assert.NoError(err)

	// c[:HeaderSize] is the header
	// c[HeaderSize] is the first byte of the compressed data --> should be a 1
	assert.Equal(byte(1), c[HeaderSize])

	// then we should have a backref to the first 1
	assert.Equal(byte(SymbolShort), c[HeaderSize+1])

	shortBackref, _, _ := InitBackRefTypes(0, BestCompression)

	sbr := backref{bType: shortBackref}

	err = sbr.readFrom(bitio.NewReader(bytes.NewReader(c[HeaderSize+1:])))
	assert.NoError(err)

	sbr.address = 255 // should be invalid
	var buf bytes.Buffer
	sbr.writeTo(bitio.NewWriter(&buf), 0)

	copy(c[HeaderSize+1:], buf.Bytes())

	_, err = Decompress(c, []byte{})
	assert.Error(err)
}

func TestCraftExpandingInput(t *testing.T) {
	assert := require.New(t)
	dict := getDictionary()

	// craft an input we know will expand
	d := craftExpandingInput(dict, 100000)
	compressor, err := NewCompressor(dict, BestCompression)
	assert.NoError(err)
	c, err := compressor.Compress(d)
	lenC := len(c)
	assert.NoError(err)
	assert.Greater(10*len(c)/len(d), 12) // 1.2⁻¹ : a very disappointing compression ratio

	// ensure that bypassing works.
	compressor.Reset()
	_, err = compressor.Write(d)
	assert.NoError(err)
	assert.True(compressor.ConsiderBypassing(), "should consider bypassing")
	assert.Less(compressor.Len(), lenC, "should have switched to NoCompression")
}

func craftExpandingInput(dict []byte, size int) []byte {
	_, _, dRefType := InitBackRefTypes(len(dict), BestCompression)
	nbBytesExpandingBlock := dRefType.nbBytesBackRef

	// the following two methods convert between a byte slice and a number; just for convenient use as map keys and counters
	bytesToNum := func(b []byte) uint64 {
		var res uint64
		for i := range b {
			res += uint64(b[i]) << uint64(i*8)
		}
		return res
	}

	fillNum := func(dst []byte, n uint64) {
		for i := range dst {
			dst[i] = byte(n)
			n >>= 8
		}
	}

	covered := make(map[uint64]struct{}) // combinations present in the dictionary, to avoid
	for i := range dict {
		if dict[i] == 255 {
			covered[bytesToNum(dict[i+1:i+nbBytesExpandingBlock])] = struct{}{}
		}
	}
	isCovered := func(n uint64) bool {
		_, ok := covered[n]
		return ok
	}

	res := make([]byte, size)
	var blockCtr uint64
	for i := 0; i < len(res); i += nbBytesExpandingBlock {
		for isCovered(blockCtr) {
			blockCtr++
			if blockCtr == 0 {
				panic("overflow")
			}
		}
		res[i] = 255
		fillNum(res[i+1:i+nbBytesExpandingBlock], blockCtr)
		blockCtr++
		if blockCtr == 0 {
			panic("overflow")
		}
	}
	return res
}

func TestRevertAfterBypass(t *testing.T) {
	const (
		block1Size = 500
		block2Size = 1000
	)

	d, err := os.ReadFile("./testdata/average_block.hex")
	assert.NoError(t, err)

	dict := getDictionary()
	compressor, err := NewCompressor(dict, BestCompression)
	assert.NoError(t, err)

	_, err = compressor.Write(d[:block1Size])
	assert.NoError(t, err)

	block2 := craftExpandingInput(dict, block2Size)
	_, err = compressor.Write(block2)
	assert.NoError(t, err)

	assert.True(t, compressor.ConsiderBypassing())

	assert.NoError(t, compressor.Revert())

	c := compressor.Bytes()
	dBack, err := Decompress(c, dict)
	assert.NoError(t, err)
	assert.Equal(t, d[:block1Size], dBack)
	assert.Less(t, len(c), block1Size, "first block should be compressed")
}

func BenchmarkCompressNomial100kB(b *testing.B) {
	// read the file
	d, err := os.ReadFile("./testdata/average_block.hex")
	if err != nil {
		b.Fatal(err)
	}

	// convert to bytes
	data, err := hex.DecodeString(string(d))
	if err != nil {
		b.Fatal(err)
	}
	if len(data) > (100 * 1024) {
		data = data[:100*1024]
	}

	dict := getDictionary()
	compressor, err := NewCompressor(dict, BestCompression)
	if err != nil {
		b.Fatal(err)
	}

	// benchmark lzss
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compressor.Compress(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompressConstanted100kB(b *testing.B) {
	// 100kB of zeroes.
	data := make([]byte, 100*1024)

	dict := getDictionary()
	compressor, err := NewCompressor(dict, BestCompression)
	if err != nil {
		b.Fatal(err)
	}

	// benchmark lzss
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compressor.Compress(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompressedSize(b *testing.B) {
	// read the file
	d, err := os.ReadFile("./testdata/average_block.hex")
	if err != nil {
		b.Fatal(err)
	}

	// convert to bytes
	data, err := hex.DecodeString(string(d))
	if err != nil {
		b.Fatal(err)
	}
	data = data[1<<17 : (1<<17)+1<<16]

	dict := getDictionary()

	compressor, err := NewCompressor(dict, BestCompression)
	if err != nil {
		b.Fatal(err)
	}

	// benchmark lzss
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := compressor.CompressedSize256k(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}
