package column_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {

	store := column.NewStore()

	assert.False(t, store.Exists("a"))
	assert.Panics(t, func() {
		store.IsIgnored("a")
	})

	store.AddToRound(0, "a", 4, column.Committed, true)

	assert.Len(t, store.AllKeys(), 1)
	assert.True(t, store.Exists("a"))
	assert.False(t, store.IsIgnored("a"))

	store.MarkAsIgnored("a")

	assert.True(t, store.IsIgnored("a"))

}
