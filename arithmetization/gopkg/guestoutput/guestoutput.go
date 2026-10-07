// Package guestoutput reads the guest's full output on the host.
//
// The R5 interpreter only traces the first bytes of the guest output (see
// src/main/lib/io/write_output.zkc). Execute runs it in fast mode and, on each
// write_output call, copies the whole output out of guest RAM: this happens in
// the VM host, outside the interpreter program, so it adds nothing to the trace.
package guestoutput

import (
	"errors"
	"fmt"
	"math"

	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

// Execute runs binf on input in fast mode, like BinaryFile.Execute, and also
// returns the guest's full output: the bytes of all write_output calls, in call
// order. They are located by the operands of the RTYPE_WRITE_OUTPUT
// instruction, held by interpreter_b when it calls write_output: the output
// address in v1 and its size in v2 (write_output itself is not given the size).
func Execute(binf *constraints.BinaryFile[koalabear.Element], input map[string][]byte) (outputs map[string][]byte, guestOutput []byte, errs []error) {
	program := binf.ExecutionProgram()
	writeOutput, ok1 := program.HasModule("write_output")
	caller, ok2 := program.HasModule("interpreter_b")
	ram, ok3 := program.HasModule("ram")
	if !ok1 || !ok2 || !ok3 {
		return nil, nil, []error{errors.New("program has no write_output, interpreter_b or ram")}
	}
	v1 := program.Module(caller).HasRegister("v1")
	v2 := program.Module(caller).HasRegister("v2")
	if !v1.HasValue() || !v2.HasValue() {
		return nil, nil, []error{errors.New("interpreter_b has no v1 or v2 register")}
	}

	var captureErr error
	// Break on entry to write_output, with its caller paused just below it on
	// the call stack.
	bci := vm.NewBytecodeInterpreter(program.BreakPoint(writeOutput, vm.ProgramPoint{}))
	bci.BreakPointer(func(uint32) bool {
		cp := bci.CheckPoint()
		calls := cp.CallStack()
		if len(calls) < 2 || calls[len(calls)-2].FunctionId != caller {
			captureErr = errors.New("write_output was not called from interpreter_b")
			return false
		}
		frame := cp.DataStack()[calls[len(calls)-2].FramePointer:]
		offset, size := frame[v1.Unwrap()].Uint64(), frame[v2.Unwrap()].Uint64()
		if size > math.MaxInt || offset+size+3 < offset {
			captureErr = fmt.Errorf("write_output(offset=%d, size=%d) is out of range", offset, size)
			return false
		}
		out := make([]byte, size)
		// Each RAM word holds 4 bytes, little-endian (see read_8); words in no
		// page are zero.
		first, end := offset/4, (offset+size+3)/4
		for _, mem := range cp.Memories() {
			if mem.ModuleId() != ram {
				continue
			}
			for _, page := range mem.Pages() {
				data, start := page.Data(), page.Address()
				for w := max(first, start); w < min(end, start+uint64(len(data))); w++ {
					for b := range uint64(4) {
						if address := 4*w + b; address >= offset && address < offset+size {
							out[address-offset] = byte(data[w-start].Uint64() >> (8 * b))
						}
					}
				}
			}
		}
		guestOutput = append(guestOutput, out...)
		return false
	})
	outputs, _, errs = vm.BootAndExecute(bci, input, math.MaxUint)
	if captureErr != nil {
		errs = append(errs, captureErr)
	}
	return outputs, guestOutput, errs
}
