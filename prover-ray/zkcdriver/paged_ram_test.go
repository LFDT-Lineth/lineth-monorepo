package zkcdriver_test

import (
	"bytes"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
	"github.com/stretchr/testify/require"
)

// pagedRAM reports whether the memory named ram of prog is paged.
func pagedRAM[W vm.Word[W]](t *testing.T, prog vm.Program[W]) bool {
	t.Helper()
	for _, m := range prog.Modules() {
		if mem, ok := m.(*vm.Memory[W]); ok && mem.Name() == "ram" {
			return mem.IsPaged()
		}
	}
	t.Fatal("no memory named ram")
	return false
}

// requirePagedRAM checks that both programs of the driver keep the RAM paged,
// as the arithmetization declares it.
func requirePagedRAM(t *testing.T, driver *zkcdriver.ZkCDriver) {
	t.Helper()
	require.True(t, pagedRAM(t, driver.BinaryFile.ExecutionProgram()), "execution program: ram should be paged")
	require.True(t, pagedRAM(t, driver.BinaryFile.TracingProgram()), "tracing program: ram should be paged")
}

func newSystem() *wiop.System {
	sys := wiop.NewSystemf("paged-ram")
	sys.NewRound()
	return sys
}

// TestNewZkCDriverFromBinaryFile_KeepsRAMPaged checks that a driver built from
// compiled constraints keeps the RAM paged.
func TestNewZkCDriverFromBinaryFile_KeepsRAMPaged(t *testing.T) {
	binf, err := embedded.CompiledBinaryFile()
	require.NoError(t, err)
	driver, err := zkcdriver.NewZkCDriverFromBinaryFile(newSystem(), zkcdriver.Settings{TraceRound: 1, ShardSize: 500}, binf)
	require.NoError(t, err)
	requirePagedRAM(t, driver)
}

// TestNewZkCDriver_EncodedRAMIsNotPaged records a zkc bug: encoding the
// constraints drops the paged flag of memories, so a driver read from bytes
// has a flat RAM. When zkc fixes it, this fails, and dev-partial no longer
// needs [zkcdriver.NewZkCDriverFromBinaryFile].
func TestNewZkCDriver_EncodedRAMIsNotPaged(t *testing.T) {
	binf, err := embedded.CompiledBinaryFile()
	require.NoError(t, err)
	bin, err := binf.MarshalBinary()
	require.NoError(t, err)
	driver := zkcdriver.NewZkCDriver(newSystem(), zkcdriver.Settings{TraceRound: 1, ShardSize: 500}, bytes.NewReader(bin))
	require.False(t, pagedRAM(t, driver.BinaryFile.ExecutionProgram()), "zkc now keeps the paged flag: drop the workaround")
}
