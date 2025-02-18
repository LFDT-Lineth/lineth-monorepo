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
	"github.com/consensys/go-corset/pkg/schema"
	"github.com/consensys/go-corset/pkg/trace"
	"github.com/consensys/go-corset/pkg/trace/lt"
	"github.com/sirupsen/logrus"
)

const TraceOverflowExitCode = 77

// Embed the whole constraint system at compile time, so no
// more need to keep it in sync
//
//go:embed zkevm.bin
var zkevmStr string

// ReadZkEvmBin parses and compiles a "zkevm.bin" into an air.Schema. f is closed
// at the end of the function call.
func ReadZkevmBin() (*air.Schema, error) {
	var (
		binf binfile.BinaryFile
		buf  []byte = []byte(zkevmStr)
	)
	// TODO: why is only this one needed??
	gob.Register(binfile.Attribute(&corset.SourceMap{}))
	// Parse zkbinary file
	err := binf.UnmarshalBinary(buf)
	// Sanity check for errors
	if err != nil {
		return nil, fmt.Errorf("could not parse the read bytes of the 'zkevm.bin' file into an hir.Schema: %w", err)
	}
	// Extract schema
	hirSchema := &binf.Schema
	// This performs the corset compilation
	return hirSchema.LowerToMir().LowerToAir(), nil
}

func ReadLtTraces(f io.ReadCloser, sch *air.Schema) (trace.Trace, error) {

	defer f.Close()

	readBytes, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed reading the file: %w", err)
	}

	rawTraces, err := lt.FromBytes(readBytes)
	if err != nil {
		return nil, fmt.Errorf("failed parsing the bytes of the raw trace '.lt' file: %w", err)
	}

	expTraces, errs := schema.NewTraceBuilder(sch).Build(rawTraces)
	if len(errs) > 0 {
		logrus.Warnf("corset expansion gave the following errors: %v", errors.Join(errs...).Error())
	}

	return expTraces, nil
}
