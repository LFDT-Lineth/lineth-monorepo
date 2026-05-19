package zkcdriver

import (
	"compress/gzip"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/consensys/go-corset/pkg/util/collection/typed"
	zkc_util "github.com/consensys/go-corset/pkg/zkc/util"
	"github.com/sirupsen/logrus"
)

// ReadConstraintsFile reads in a binary representation of the constraints to be
// used for creating proofs. This additionally extracts the metadata map from
// the binary file.  This contains information which can be used to cross-check
// the constraints file (e.g. the git commit of the enclosing repository when it
// was built).
func ReadConstraintsFile(r io.Reader) (*BinaryFile, typed.Map, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, typed.Map{}, fmt.Errorf("io.ReadAll failed to read zkevm.bin: %w", err)
	}
	return UnmarshalConstraintsFile(buf)
}

// UnmarshalConstraintsFile parses a binary constraints file (e.g. blake.bin)
// into a BinaryFile instance.  This additionally extracts the metadata map from
// the binary file. This contains information which can be used to cross-check
// the constraints file (e.g. the git commit of the enclosing repository when it
// was built).
func UnmarshalConstraintsFile(buf []byte) (*BinaryFile, typed.Map, error) {
	var (
		binf     BinaryFile
		metadata typed.Map
	)
	// Parse zkbinary file
	err := binf.UnmarshalBinary(buf)
	// Sanity check for errors
	if err != nil {
		return nil, metadata, fmt.Errorf("failed parsing binary constraints file: %w", err)
	}
	// extract file header (which contains versioning info + metadata)
	header := binf.Header()
	// Check metadata in binary constraints file is valid
	if metadata, err = header.GetMetaData(); err != nil {
		return nil, metadata, errors.New("corrupt metatdata in binary constraints file")
	}
	// Done
	return &binf, metadata, err
}

type readCloserChain struct {
	io.Reader
	closers []io.Closer
}

func (r *readCloserChain) Close() error {
	var firstErr error
	for _, c := range r.closers {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// ReadMaybeCompressedFile returns a reader over the zkc input data. If the file
// ends with .gz, it transparently decompresses it. The caller (See ReadLtTraces
// below) MUST close the returned io.ReadCloser.
func ReadMaybeCompressedFile(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)

	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("missing zkc inputs file, at %v : %w", path, err)
		} else {
			err = fmt.Errorf("unable to open zkc inputs file %q: %w", path, err)
		}
		return nil, err
	} else if !strings.HasSuffix(path, ".gz") {
		return f, nil
	}

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip.Reader for %q: %w", path, err)
	}

	// Wrap so closing the reader also closes the underlying file
	rc := &readCloserChain{
		Reader: gzr,
		closers: []io.Closer{
			gzr,
			f,
		},
	}

	logrus.Infof("Streaming decompression of traceFile:%q", path)
	return rc, nil
}

// ReadZkcInputFile reads a given JSON inputs file which contains byte strings
// (e.g. in hex) for each input memory declared in the ZkC program.
func ReadZkcInputFile(f io.ReadCloser) (inputs map[string][]byte, err error) {
	defer f.Close()
	// Read the trace file, including any metadata embedded within.
	readBytes, err := io.ReadAll(f)
	if err != nil {
		return inputs, fmt.Errorf("failed reading the file: %w", err)
	} else if inputs, err = zkc_util.ParseJsonInputFile(readBytes); err != nil {
		// wrap parser error with something more useful.
		return inputs, fmt.Errorf("failed parsing zkc input file: %w", err)
	}
	//
	return inputs, nil
}
