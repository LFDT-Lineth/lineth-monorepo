package backend

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validGuestOutput() []byte {
	out := make([]byte, guestOutputSize)
	out[0], out[1] = 0x00, 0x03
	for i := 2; i < guestOutputSize; i++ {
		out[i] = byte(i)
	}
	return out
}

func TestClassifyGuestOutput(t *testing.T) {
	valid := validGuestOutput()

	t.Run("valid output is returned", func(t *testing.T) {
		got, err := classifyGuestOutput(map[string][]byte{guestOutputMemory: valid}, nil)
		require.NoError(t, err)
		assert.Equal(t, valid, got)
	})

	t.Run("guest rejection is distinguished from a VM bug", func(t *testing.T) {
		errs := []error{&vm.Failure{Message: "block 42 invalid: bad state root"}}
		_, err := classifyGuestOutput(nil, errs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "guest rejected the block")
		assert.Contains(t, err.Error(), "bad state root")
	})

	t.Run("internal VM error is reported as such", func(t *testing.T) {
		_, err := classifyGuestOutput(nil, []error{fmt.Errorf("interpreter panic")})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "guest VM execution failed")
	})

	t.Run("missing output means the block was rejected", func(t *testing.T) {
		_, err := classifyGuestOutput(map[string][]byte{"other": {1}}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no guest_output")
	})

	t.Run("wrong length is rejected", func(t *testing.T) {
		_, err := classifyGuestOutput(map[string][]byte{guestOutputMemory: {0x00, 0x03, 0x01}}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "3 bytes, want 34")
	})

	t.Run("wrong schema id is rejected", func(t *testing.T) {
		bad := validGuestOutput()
		bad[0], bad[1] = 0x00, 0x01
		_, err := classifyGuestOutput(map[string][]byte{guestOutputMemory: bad}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "0x0001, want 0x0003")
	})
}
