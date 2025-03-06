package arithmetization

import (
	_ "embed"
	"encoding/gob"
	"errors"
	"fmt"
	"io"

	"github.com/consensys/go-corset/pkg/air"
	"github.com/consensys/go-corset/pkg/binfile"
	"github.com/consensys/go-corset/pkg/corset"
	"github.com/consensys/go-corset/pkg/mir"
	"github.com/consensys/go-corset/pkg/trace"
	"github.com/consensys/go-corset/pkg/trace/lt"
	"github.com/consensys/go-corset/pkg/util/collection/typed"
)

const TraceOverflowExitCode = 77

// Embed the whole constraint system at compile time, so no
// more need to keep it in sync
//
//go:embed zkevm.bin
var zkevmStr string

// ReadZkevmBin parses and compiles a "zkevm.bin" file into an air.Schema,
// whilst applying whatever optimisations are requested. Optimisations can
// impact the size of the generated schema and, consequently, the size of the
// expanded trace.  For example, certain optimisations eliminate unnecessary
// columns creates for multiplicative inverses.  However, optimisations do not
// always improve overall performance, as they can increase the complexity of
// other constraints.  The DEFAULT_OPTIMISATION_LEVEL is the recommended level
// to use in general, whilst others are intended for testing purposes (i.e. to
// try out new optimisations to see whether they help or hinder, etc).
//
// This additionally extracts the metadata map from the zkevm.bin file.  This
// contains information which can be used to cross-check the zkevm.bin file,
// such as the git commit of the enclosing repository when it was built.
func ReadZkevmBin(optConfig *mir.OptimisationConfig) (schema *air.Schema, metadata typed.Map, err error) {
	var (
		binf binfile.BinaryFile
		buf  []byte = []byte(zkevmStr)
	)
	// TODO: why is only this one needed??
	gob.Register(binfile.Attribute(&corset.SourceMap{}))
	// Parse zkbinary file
	err = binf.UnmarshalBinary(buf)
	// Sanity check for errors
	if err != nil {
		return nil, metadata, fmt.Errorf("could not parse the read bytes of the 'zkevm.bin' file into an hir.Schema: %w", err)
	}
	// Extract schema
	hirSchema := &binf.Schema
	// Attempt to extract metadata from bin file, and sanity check constraints
	// commit information is available.
	if metadata, err = binf.Header.GetMetaData(); metadata.IsEmpty() {
		return nil, metadata, errors.New("missing metatdata from 'zkevm.bin' file")
	}
	// This performs the corset compilation
	return hirSchema.LowerToMir().LowerToAir(*optConfig), metadata, err
}

// ReadLtTraces reads a given LT trace file which contains (unexpanded) column
// data, and additionally extracts the metadata map from the zkevm.bin file. The
// metadata contains information which can be used to cross-check the zkevm.bin
// file, such as the git commit of the enclosing repository when it was built.
func ReadLtTraces(f io.ReadCloser, sch *air.Schema) (rawColumns []trace.RawColumn, metadata typed.Map, err error) {
	var (
		traceFile lt.TraceFile
		ok        bool
	)
	defer f.Close()
	// Read the trace file, including any metadata embedded within.
	readBytes, err := io.ReadAll(f)
	if err != nil {
		return nil, metadata, fmt.Errorf("failed reading the file: %w", err)
	} else if err = traceFile.UnmarshalBinary(readBytes); err != nil {
		return nil, metadata, fmt.Errorf("failed parsing the bytes of the raw trace '.lt' file: %w", err)
	}
	// Attempt to extract metadata from trace file, and sanity check the
	// constraints commit information is present.
	if metadata, err = traceFile.Header.GetMetaData(); metadata.IsEmpty() {
		return nil, metadata, errors.New("missing metatdata from '.lt' file")
	} else if metadata, ok = metadata.Map("constraints"); !ok {
		return nil, metadata, errors.New("missing constraints metatdata from '.lt' file")
	}
	// Done
	return traceFile.Columns, metadata, nil
}
