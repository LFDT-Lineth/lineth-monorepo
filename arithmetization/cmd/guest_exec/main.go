// Command guest_exec runs a guest on the R5 interpreter in fast mode and prints
// its outputs like `zkc exec --fast`, except that guest_output is the whole guest
// output, read by the host (see gopkg/guestoutput). The interpreter only traces
// its first bytes, printed as guest_output_traced.
//
// Usage: guest_exec <input.json>
package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/LFDT-Lineth/zkc/pkg/util/file"
	zkcutil "github.com/LFDT-Lineth/zkc/pkg/zkc/util"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/guestoutput"
)

func main() {
	if len(os.Args) != 2 {
		exit(1, errors.New("usage: guest_exec <input.json>"))
	}
	_, data, err := file.ReadAndUncompress(os.Args[1])
	if err != nil {
		exit(1, err)
	}
	inputs, err := zkcutil.ParseJsonInputFile(data)
	if err != nil {
		exit(1, err)
	}
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		exit(1, err)
	}
	inputs, _ = vm.FilterInputs(binf.RawProgram(), inputs)

	outputs, guestOutput, errs := guestoutput.Execute(binf, inputs)
	if len(errs) > 0 {
		// Same exit code as `zkc exec`; a guest exit failure reads "EXIT CODE = <code>".
		exit(4, errors.Join(errs...))
	}
	outputs["guest_output_traced"] = outputs["guest_output"]
	outputs["guest_output"] = guestOutput
	for _, name := range slices.Sorted(maps.Keys(outputs)) {
		fmt.Printf("%s = 0x%s\n", name, hex.EncodeToString(outputs[name]))
	}
}

func exit(code int, err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(code)
}
