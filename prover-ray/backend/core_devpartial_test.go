package backend

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/devmode"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimal_elf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// devmodeImage is the devmode proof image dev-partial must return for the guest
// output hash: the hash bytes and a zero program VK as the public inputs.
func devmodeImage(t *testing.T, hash []byte) []byte {
	t.Helper()
	sys, err := devmode.NewSystem()
	require.NoError(t, err)
	pub := make(wiop.PublicInput, devmode.NumPublicInputs)
	for i := range pub {
		var e field.Element
		if i < len(hash) {
			e.SetUint64(uint64(hash[i]))
		}
		pub[i] = field.ElemFromBase(e)
	}
	proof, pub, err := devmode.Prove(sys, pub)
	require.NoError(t, err)
	img, err := devmode.Encode(sys, proof, pub, devmode.GuestBase)
	require.NoError(t, err)
	return img
}

// devPartialJob is an l2-execution job of guest id.
func devPartialJob(id [32]byte) Job {
	return Job{ID: "dev-partial", Type: ProofTypeL2Execution, GuestProgramID: id}
}

// TestProveDevPartial runs dev-partial on guests that write a given output: it checks
// the trace, decodes the public inputs from the output and returns a devmode
// proof of the output hash.
func TestProveDevPartial(t *testing.T) {
	hash, framed := referenceFrame(t)
	badHash := slices.Clone(hash)
	badHash[0] ^= 0xff
	// A well-hashed frame whose public inputs are shorter than the fixed head.
	shortFrame := framed[:guestOutputSchemaIDSize+10]

	dir := t.TempDir()
	valid := writeGuest(t, dir, minimal_elf.OutputAndLogELF(hash, framed))
	wrongHash := writeGuest(t, dir, minimal_elf.OutputAndLogELF(badHash, framed))
	shortInputs := writeGuest(t, dir, minimal_elf.OutputAndLogELF(keccak256(shortFrame), shortFrame))

	c, err := New(Config{Mode: ProverModeDevPartial, GuestsDir: dir})
	require.NoError(t, err)
	prove := func(id [32]byte) Result {
		return c.Prove(context.Background(), devPartialJob(id))
	}

	t.Run("valid output", func(t *testing.T) {
		result := prove(valid)
		require.Equal(t, ResultStatusOK, result.Status, "prove err: %v", result.Err)
		assert.Equal(t, referencePublicInputs(t), result.PublicInputs, "the public inputs should be decoded from the guest output")
		assert.Equal(t, slices.Concat(hash, framed), result.GuestOutput, "the guest output should be returned for the native runner check")
		assert.Equal(t, devmodeImage(t, hash), result.ProofBytes, "the proof should be a devmode proof of the output hash")
	})

	t.Run("hash does not match the frame", func(t *testing.T) {
		result := prove(wrongHash)
		assert.Equal(t, ResultStatusFailed, result.Status)
		assert.ErrorContains(t, result.Err, "hash does not match")
	})

	t.Run("public inputs do not decode", func(t *testing.T) {
		result := prove(shortInputs)
		assert.Equal(t, ResultStatusFailed, result.Status)
		assert.ErrorContains(t, result.Err, "fixed head")
	})
}

func TestNew_DevPartialNeedsGuestsDir(t *testing.T) {
	_, err := New(Config{Mode: ProverModeDevPartial})
	assert.ErrorContains(t, err, "guests folder must be set")
}

// TestDevmodePublicInputs checks that the traced guest output hash must equal
// the executed one.
func TestDevmodePublicInputs(t *testing.T) {
	c, err := New(Config{Mode: ProverModeDevPartial, GuestsDir: t.TempDir()})
	require.NoError(t, err)
	pub := make(wiop.PublicInput, len(c.sys.PublicInputs))
	for i := range pub {
		pub[i] = field.ElemFromBase(field.Element{})
	}

	got, err := c.devmodePublicInputs(pub, make([]byte, guestOutputHashSize))
	require.NoError(t, err)
	assert.Len(t, got, devmode.NumPublicInputs)

	hash := make([]byte, guestOutputHashSize)
	hash[3] = 1
	_, err = c.devmodePublicInputs(pub, hash)
	assert.ErrorContains(t, err, "traced guest output byte 3")
}

// TestNewDevPartial_EveryQueryIsChecked checks that every query left in the
// check-only system is of a type whose Check runs on the trace. A query whose
// Check does nothing, such as a MessageBus, would silently not be checked.
func TestNewDevPartial_EveryQueryIsChecked(t *testing.T) {
	for _, shardSize := range []uint64{0, devPartialShardSize} {
		c, err := New(Config{Mode: ProverModeDevPartial, GuestsDir: t.TempDir(), ShardSize: shardSize})
		require.NoError(t, err)
		queries := c.sys.UnreducedQueries()
		require.NotEmpty(t, queries)
		for _, q := range queries {
			switch q.(type) {
			// Before adding a type, check that its Check runs on the trace.
			case *wiop.Vanishing, *wiop.TableRelationQuery:
			default:
				t.Errorf("shard size %d: %T at %s is not a query type dev-partial checks", shardSize, q, q.Context().Path())
			}
		}
	}
}

// devPartialShardSize splits the trace of the OutputAndLogELF guest into 2 shards.
const devPartialShardSize = 500

// TestProveDevPartial_Sharded checks that dev-partial checks a sharded trace shard by
// shard, merges the shards with the mock merger and returns what it returns
// without sharding.
func TestProveDevPartial_Sharded(t *testing.T) {
	hash, framed := referenceFrame(t)
	dir := t.TempDir()
	id := writeGuest(t, dir, minimal_elf.OutputAndLogELF(hash, framed))
	job := devPartialJob(id)

	unsharded, err := New(Config{Mode: ProverModeDevPartial, GuestsDir: dir})
	require.NoError(t, err)
	want := unsharded.Prove(context.Background(), job)
	require.Equal(t, ResultStatusOK, want.Status, "prove err: %v", want.Err)

	c, err := New(Config{Mode: ProverModeDevPartial, GuestsDir: dir, ShardSize: devPartialShardSize})
	require.NoError(t, err)
	got := c.Prove(context.Background(), job)
	require.Equal(t, ResultStatusOK, got.Status, "prove err: %v", got.Err)
	assert.Equal(t, want, got, "sharding should not change the result")
}

// TestProveDevPartial_Sharded_UnbalancedBus checks that with sharding, where no
// shard checks its own message bus, dev-partial still rejects a trace that
// unbalances the bus: the mock merger checks it across the shards.
func TestProveDevPartial_Sharded_UnbalancedBus(t *testing.T) {
	hash, framed := referenceFrame(t)
	dir := t.TempDir()
	id := writeGuest(t, dir, minimal_elf.OutputAndLogELF(hash, framed))
	c, err := New(Config{Mode: ProverModeDevPartial, GuestsDir: dir, ShardSize: devPartialShardSize})
	require.NoError(t, err)
	interpreter := interpreterColumns(t, c)
	tamper := &tamperAction{f: func(rt *wiop.Runtime) {
		setInterpreterRows(t, rt, interpreter, func(rows []field.Element, row int) { rows[row] = rows[row-1] })
	}}
	c.sys.Rounds[1].RegisterAction(tamper)

	result := c.Prove(context.Background(), devPartialJob(id))
	assert.Equal(t, ResultStatusFailed, result.Status)
	assert.ErrorContains(t, result.Err, "does not balance across the shards")
}

// interpreterColumns returns the trace columns of the interpreter module.
func interpreterColumns(t *testing.T, c *Core) []*wiop.Column {
	t.Helper()
	var interpreter []*wiop.Column
	for _, col := range c.sys.Rounds[1].Columns {
		if strings.HasPrefix(col.Context.Path(), wiopSystemName+"/module-interpreter/column-") {
			interpreter = append(interpreter, col)
		}
	}
	require.NotEmpty(t, interpreter, "the interpreter module should have trace columns")
	return interpreter
}

// setInterpreterRows overrides the interpreter columns with edit applied to
// their rows, at a row that differs from the one before.
func setInterpreterRows(t *testing.T, rt *wiop.Runtime, interpreter []*wiop.Column, edit func(rows []field.Element, row int)) {
	t.Helper()
	row := -1
	for _, col := range interpreter {
		rows := rt.GetColumnAssignment(col).Plain.AsBase()
		for k := 1; k < len(rows) && row < 0; k++ {
			if rows[k] != rows[k-1] {
				row = k
			}
		}
	}
	require.Positive(t, row, "two interpreter rows should differ")
	for _, col := range interpreter {
		v := rt.GetColumnAssignment(col)
		rows := slices.Clone(v.Plain.AsBase())
		edit(rows, row)
		rt.OverrideColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(rows), Padding: v.Padding})
	}
}

// tamperAction runs f, if set, on the trace round after the trace is assigned.
type tamperAction struct{ f func(*wiop.Runtime) }

func (a *tamperAction) Run(rt *wiop.Runtime) {
	if a.f != nil {
		a.f(rt)
	}
}

// TestCheckAssignment checks that dev-partial rejects a trace that breaks a
// constraint, and one that keeps every constraint but unbalances the message
// bus, which only the verifier actions check.
func TestCheckAssignment(t *testing.T) {
	hash, framed := referenceFrame(t)
	dir := t.TempDir()
	id := writeGuest(t, dir, minimal_elf.OutputAndLogELF(hash, framed))
	c, err := New(Config{Mode: ProverModeDevPartial, GuestsDir: dir})
	require.NoError(t, err)
	tamper := &tamperAction{}
	c.sys.Rounds[1].RegisterAction(tamper)

	g, err := c.loadGuest(id)
	require.NoError(t, err)
	inputs, err := c.encodeGuestInputs(Job{}, g)
	require.NoError(t, err)
	lazyTrace, err := c.driver.TryTraceZkcInputs(&zkcdriver.PreReadInputs{Inputs: inputs})
	require.NoError(t, err)
	shard, err := traceShard(lazyTrace, 0)
	require.NoError(t, err)
	assign := func(rt *wiop.Runtime) { c.driver.AssignTraceShard(rt, shard, field.Octuplet{}) }

	interpreter := interpreterColumns(t, c)
	set := func(rt *wiop.Runtime, edit func(rows []field.Element, row int)) {
		setInterpreterRows(t, rt, interpreter, edit)
	}

	t.Run("honest trace", func(t *testing.T) {
		tamper.f = nil
		_, _, err := c.checkAssignment(assign)
		require.NoError(t, err)
	})

	t.Run("broken constraint", func(t *testing.T) {
		tamper.f = func(rt *wiop.Runtime) {
			set(rt, func(rows []field.Element, row int) {
				one := field.One()
				rows[row].Add(&rows[row], &one)
			})
		}
		_, _, err := c.checkAssignment(assign)
		assert.ErrorContains(t, err, "unreduced query check failed")
	})

	t.Run("unbalanced message bus", func(t *testing.T) {
		// A copied row keeps every row constraint but sends its messages twice.
		tamper.f = func(rt *wiop.Runtime) {
			set(rt, func(rows []field.Element, row int) { rows[row] = rows[row-1] })
		}
		_, _, err := c.checkAssignment(assign)
		assert.ErrorContains(t, err, "product is")
	})
}
