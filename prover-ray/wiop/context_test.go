package wiop_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewRootFramef verifies that a root frame is created with the correct label
// and has no parent.
func TestNewRootFramef(t *testing.T) {
	cases := []struct {
		desc     string
		msg      string
		args     []any
		wantPath string
	}{
		{"plain label", "root", nil, "root"},
		{"formatted label", "proto-%d", []any{42}, "proto-42"},
		{"empty label", "", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			f := wiop.NewRootFramef(tc.msg, tc.args...)
			require.NotNil(t, f)
			assert.Equal(t, tc.wantPath, f.Path())
			assert.Nil(t, f.Parent)
			assert.Zero(t, f.PC)
		})
	}
}

// TestContextFrame_Childf verifies that child frames form the correct ancestry.
func TestContextFrame_Childf(t *testing.T) {
	root := wiop.NewRootFramef("root")
	child := root.Childf("child")
	grandchild := child.Childf("gc-%s", "x")

	assert.Equal(t, "root/child", child.Path())
	assert.Equal(t, "root/child/gc-x", grandchild.Path())
	assert.Equal(t, root, child.Parent)
	assert.Equal(t, child, grandchild.Parent)
	assert.NotZero(t, child.PC, "Childf should capture caller PC")
}

// TestContextFrame_Childf_NilPanic verifies that Childf panics on a nil receiver.
func TestContextFrame_Childf_NilPanic(t *testing.T) {
	var f *wiop.ContextFrame
	assert.Panics(t, func() { f.Childf("x") })
}

// TestContextFrame_Path covers nil, root, and multi-level paths.
func TestContextFrame_Path(t *testing.T) {
	cases := []struct {
		desc     string
		build    func() *wiop.ContextFrame
		wantPath string
	}{
		{"nil", func() *wiop.ContextFrame { return nil }, ""},
		{"root only", func() *wiop.ContextFrame { return wiop.NewRootFramef("r") }, "r"},
		{"depth 3", func() *wiop.ContextFrame {
			r := wiop.NewRootFramef("a")
			b := r.Childf("b")
			return b.Childf("c")
		}, "a/b/c"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.wantPath, tc.build().Path())
		})
	}
}

// TestContextFrame_String verifies that String() matches Path().
func TestContextFrame_String(t *testing.T) {
	f := wiop.NewRootFramef("root").Childf("child")
	assert.Equal(t, f.Path(), f.String())
}

// TestContextFrame_CallerInfo verifies the caller info behaviour for root and
// child frames.
func TestContextFrame_CallerInfo(t *testing.T) {
	root := wiop.NewRootFramef("r")
	assert.Empty(t, root.CallerInfo(), "root frames have zero PC → empty CallerInfo")

	child := root.Childf("c")
	info := child.CallerInfo()
	require.NotEmpty(t, info, "child frame should have a caller info")
	assert.Contains(t, info, ":", "CallerInfo should be file:line")
}
