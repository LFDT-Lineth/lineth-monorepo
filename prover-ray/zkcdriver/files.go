package zkcdriver

import (
	"compress/gzip"
	_ "embed"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/consensys/go-corset/pkg/asm"
	"github.com/consensys/go-corset/pkg/binfile"
	"github.com/consensys/go-corset/pkg/corset"
	"github.com/consensys/go-corset/pkg/ir/air"
	"github.com/consensys/go-corset/pkg/ir/mir"
	"github.com/consensys/go-corset/pkg/schema/module"
	"github.com/consensys/go-corset/pkg/trace/lt"
	"github.com/consensys/go-corset/pkg/util/collection/typed"
	"github.com/consensys/go-corset/pkg/util/field"
	"github.com/consensys/go-corset/pkg/util/field/koalabear"
	"github.com/sirupsen/logrus"
)

// UnmarshalZkEVMBin parses and compiles a "zkevm.bin" buffered file into a
// BinaryFile.  This additionally extracts the metadata map from the zkevm.bin
// file.  This contains information which can be used to cross-check the
// zkevm.bin file, such as the git commit of the enclosing repository when it
// was built.
func ReadZkevmBin(r io.Reader) (*binfile.BinaryFile, typed.Map, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, typed.Map{}, fmt.Errorf("io.ReadAll failed to read zkevm.bin: %w", err)
	}
	return UnmarshalZkEVMBin(buf)
}

// UnmarshalZkEVMBin parses and compiles a "zkevm.bin" buffered file into a
// BinaryFile.  This additionally extracts the metadata map from the zkevm.bin
// file.  This contains information which can be used to cross-check the
// zkevm.bin file, such as the git commit of the enclosing repository when it
// was built.
func UnmarshalZkEVMBin(buf []byte) (*binfile.BinaryFile, typed.Map, error) {
	var (
		binf     binfile.BinaryFile
		metadata typed.Map
	)
	//
	gob.Register(binfile.Attribute(&corset.SourceMap{}))
	// Parse zkbinary file
	err := binf.UnmarshalBinary(buf)
	// Sanity check for errors
	if err != nil {
		return nil, metadata, fmt.Errorf("could not parse the read bytes of the 'zkevm.bin' file into a schema: %w", err)
	}
	// Attempt to extract metadata from bin file, and sanity check constraints
	// commit information is available.
	if metadata, err = binf.Header.GetMetaData(); metadata.IsEmpty() {
		return nil, metadata, errors.New("missing metatdata from 'zkevm.bin' file")
	}
	// Done
	return &binf, metadata, err
}

// Compile a "zkevm.bin" BinaryFile into an air.Schema, whilst applying whatever
// optimisations are requested.  This also produces a "limb mapping" which
// determines how to map columns from the trace file into columns in the
// expanded trace.
//
// NOTE: optimisations can impact the size of the generated schema
// and, consequently, the size of the expanded trace.  For example, certain
// optimisations eliminate unnecessary columns creates for multiplicative
// inverses.  However, optimisations do not always improve overall performance,
// as they can increase the complexity of other constraints.  The
// DEFAULT_OPTIMISATION_LEVEL is the recommended level to use in general, whilst
// others are intended for testing purposes (i.e. to try out new optimisations
// to see whether they help or hinder, etc).
func CompileZkevmBin(
	binf *binfile.BinaryFile,
	optConfig *mir.OptimisationConfig,
) (
	*air.Schema[koalabear.Element],
	module.LimbsMap,
) {
	// There are no useful choices for the assembly config. We must always
	// vectorize, and there is only one choice of field (within the prover).
	asmConfig := asm.LoweringConfig{Field: field.KOALABEAR_16, Vectorize: true}
	// Lower to mixed micro schema
	uasmSchema := asm.LowerMixedMacroProgram(asmConfig.Vectorize, binf.Schema)
	// Apply register splitting for field agnosticity
	nasmSchema, mapping := asm.Concretize[koalabear.Element](asmConfig.Field, uasmSchema)
	// Compile
	mirSchema := asm.Compile(nasmSchema)
	// Lower to AIR
	airSchema := mir.LowerToAir(mirSchema, 30, *optConfig)
	// This performs the corset compilation
	return &airSchema, mapping
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

// readTraceFile returns a reader over the trace data.
// If the file ends with .gz, it transparently decompresses it.
// The caller (See ReadLtTraces below) MUST close the returned io.ReadCloser.
func readTraceFile(path string) (io.ReadCloser, error) {

	f, err := os.Open(path)

	if err != nil {
		if os.IsNotExist(err) {
			err = fmt.Errorf("missing trace file, at %v : %w", path, err)
		} else {
			err = fmt.Errorf("unable to open trace file %q: %w", path, err)
		}
		return nil, err
	}

	if !strings.HasSuffix(path, ".gz") {
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

// ReadLtTraces reads a given LT trace file which contains (unexpanded) column
// data, and additionally extracts the metadata map from the zkevm.bin file. The
// metadata contains information which can be used to cross-check the zkevm.bin
// file, such as the git commit of the enclosing repository when it was built.
func ReadLtTraces(f io.ReadCloser) (rawTrace lt.TraceFile, metadata typed.Map, err error) {
	var (
		traceFile lt.TraceFile
		ok        bool
	)
	defer f.Close()
	// Read the trace file, including any metadata embedded within.
	readBytes, err := io.ReadAll(f)
	if err != nil {
		return traceFile, metadata, fmt.Errorf("failed reading the file: %w", err)
	} else if err = traceFile.UnmarshalBinary(readBytes); err != nil {
		return traceFile, metadata, fmt.Errorf("failed parsing the bytes of the raw trace '.lt' file: %w", err)
	}
	// Extract trace file header
	header := traceFile.Header()
	// Attempt to extract metadata from trace file, and sanity check the
	// constraints commit information is present.
	if metadata, err = header.GetMetaData(); metadata.IsEmpty() {
		return traceFile, metadata, errors.New("missing metatdata from '.lt' file")
	} else if metadata, ok = metadata.Map("constraints"); !ok {
		return traceFile, metadata, errors.New("missing constraints metatdata from '.lt' file")
	}
	// Done
	return traceFile, metadata, nil
}
