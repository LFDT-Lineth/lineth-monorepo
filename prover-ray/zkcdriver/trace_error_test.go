package zkcdriver_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/LFDT-Lineth/zkc/pkg/util/collection/bit"
	"github.com/stretchr/testify/require"
)

// TestTryTraceZkcInputs_Error checks that a tracing failure is returned as an
// error rather than panicking, so that a prover can fail the job.
func TestTryTraceZkcInputs_Error(t *testing.T) {
	elf, _ := minimalelf.AllInOneElfProgram()
	_, driver, _, _ := newGuestOutputSystem(t, elf)

	// Without any input memory the guest cannot be booted.
	_, err := driver.TryTraceZkcInputs(&zkcdriver.PreReadInputs{Inputs: map[string]bit.Array{}})
	require.Error(t, err, "tracing without inputs should fail")
	require.NotPanics(t, func() {
		_, _ = driver.TryTraceZkcInputs(&zkcdriver.PreReadInputs{Inputs: map[string]bit.Array{}})
	})
}
