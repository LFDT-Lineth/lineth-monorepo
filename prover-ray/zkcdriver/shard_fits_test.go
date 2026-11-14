package zkcdriver_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
)

// TestCheckShardFits traces the minimal guest into one shard and checks it
// against column limits below and above its largest module.
func TestCheckShardFits(t *testing.T) {
	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatalf("compiling R5 arithmetization: %v", err)
	}
	bin, err := binf.MarshalBinary()
	if err != nil {
		t.Fatalf("encoding R5 arithmetization: %v", err)
	}
	sys := wiop.NewSystemf("check-shard-fits")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{TraceRound: 1}, bytes.NewReader(bin))

	elf, _ := minimalelf.AllInOneElfProgram()
	inputs, err := predecoding.PrepareInputs(elf, nil)
	if err != nil {
		t.Fatalf("preparing inputs: %v", err)
	}
	lazyTrace, err := driver.TryTraceZkcInputs(&zkcdriver.PreReadInputs{Inputs: inputs})
	if err != nil {
		t.Fatalf("tracing: %v", err)
	}
	maybeShard, errs := lazyTrace.Get(0)
	if len(errs) > 0 {
		t.Fatalf("tracing shard 0: %v", errors.Join(errs...))
	}
	shard := maybeShard.Unwrap()

	// The largest module of the shard.
	var largest struct {
		name string
		rows uint
	}
	for k := range shard.Width() {
		if m := shard.Module(k); m.Height() > largest.rows {
			largest.name, largest.rows = m.Name(), m.Height()
		}
	}

	t.Run("fits", func(t *testing.T) {
		for _, maxRows := range []uint{largest.rows, wiop.ColumnSizeMaxSupported} {
			if err := driver.CheckShardFits(shard, maxRows); err != nil {
				t.Fatalf("the shard should fit in %d rows: %v", maxRows, err)
			}
		}
	})

	t.Run("over the limit", func(t *testing.T) {
		maxRows := largest.rows - 1
		err := driver.CheckShardFits(shard, maxRows)
		want := fmt.Sprintf("module %s has %d rows, more than the %d a column can hold", largest.name, largest.rows, maxRows)
		if err == nil || !bytes.Contains([]byte(err.Error()), []byte(want)) {
			t.Fatalf("got error %v, want one that contains %q", err, want)
		}
	})

}
