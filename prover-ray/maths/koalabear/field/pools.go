package field

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/bufpool"

// Process-wide pools of the prover's large transient vectors, shared by the
// phases of a proof so that a buffer released by one phase serves the next,
// and every proof after the first reuses the previous one's buffers. Slices
// from these pools hold stale values; see [bufpool.Pool].
var (
	BasePool     bufpool.Pool[Element]
	ExtPool      bufpool.Pool[Ext]
	OctupletPool bufpool.Pool[Octuplet]
)
