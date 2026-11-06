package field

import "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/bufpool"

// Process-wide pools of the prover's large transient vectors, shared by the
// phases of a proof so that a buffer released by one phase serves the next,
// and every proof after the first reuses the previous one's buffers. Slices
// from these pools hold stale values; see [bufpool.Pool].
//
// Pooled memory stays resident between proofs: the garbage collector cannot
// reclaim a released buffer while its pool holds it. On a full proof this
// saves about 9 GB of allocation per proof (and about 6% of warm proving
// time) at the price of a peak RSS higher by about 3.5-3.8 GiB. [TrimPools],
// which every proof runs, only drops the sizes no recent proof asked for; a
// long-lived caller that sits idle between proofs (e.g. a service between
// jobs) should call [DrainPools] to give that memory back.
var (
	BasePool     bufpool.Pool[Element]
	ExtPool      bufpool.Pool[Ext]
	OctupletPool bufpool.Pool[Octuplet]
)

// TrimPools trims every pool (see [bufpool.Pool.Trim]): the buffers of the
// sizes no request asked for during the last few calls are dropped.
func TrimPools() {
	BasePool.Trim()
	ExtPool.Trim()
	OctupletPool.Trim()
}

// DrainPools drops every released buffer of every pool (see
// [bufpool.Pool.Drain]), so the garbage collector can reclaim them; buffers
// in use are not touched. The next proof allocates its buffers afresh, as the
// first proof of a process does.
//
// It is meant for long-lived callers between proofs, such as a service
// waiting for its next job. To also hand the memory back to the operating
// system promptly, follow it with [runtime/debug.FreeOSMemory].
func DrainPools() {
	BasePool.Drain()
	ExtPool.Drain()
	OctupletPool.Drain()
}
