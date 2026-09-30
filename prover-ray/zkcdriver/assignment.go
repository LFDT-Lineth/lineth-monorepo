package zkcdriver

import (
	"unsafe"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/zkc/pkg/ir/air"
	"github.com/LFDT-Lineth/zkc/pkg/trace"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Compile-time check: unsafe cast between koalabear.Element and field.Element
// assumes identical layout ([1]uint32).
var _ [1]uint32 = koalabear.Element{}
var _ [1]uint32 = field.Element{}

// declareDynamicSizes fixes every dynamic module's domain size from the trace
// shard, before any column is assigned.
//
// It mirrors the traversal in [AssignFromTraceShard] exactly — same module
// skipping, same unknown-column skipping, same per-column length — so the sizes
// declared here are the ones round-0 assignment would otherwise have grown the
// modules to. A module whose columns are all unknown to the system contributes
// no size and is left undeclared, which [wiop.Runtime.AdvanceRound] then
// reports rather than silently sizing to zero.
func declareDynamicSizes(
	run *wiop.Runtime,
	shard trace.Shard[koalabear.Element],
	schema air.Schema[koalabear.Element],
) {
	var (
		sys         = run.System
		columnIDMap = sys.Annotations[corsetColumnMapAnnotationKey].(map[string]wiop.ObjectID)
		sizes       = map[*wiop.Module]int{}
	)

	for modID := range shard.Width() {
		trMod := shard.Module(modID)
		if schema.Module(modID).IsStatic() {
			continue
		}
		for id := range int(trMod.Width()) {
			name := qualifiedCorsetName(trMod.Name(), trMod.Descriptor().Columns[id].Name)
			objID, ok := columnIDMap[name]
			if !ok {
				continue
			}
			wCol := sys.LookupColumn(objID)
			if !wCol.Module.IsDynamic() {
				continue
			}
			sizes[wCol.Module] = max(sizes[wCol.Module], int(trMod.Column(uint(id)).Len()))
		}
	}

	for mod, size := range sizes {
		run.DeclareDynamicSize(mod, size)
	}
}

// AssignFromTraceShard expands and assigns the trace to the given runtime.
func AssignFromTraceShard(
	run *wiop.Runtime,
	shard trace.Shard[koalabear.Element],
	schema air.Schema[koalabear.Element],
	sharedRandomness field.Octuplet,
) {

	// Only when the system was compiled to expect a γ. The driver does not choose
	// the compiler options, so it cannot assume the caller asked for shared
	// randomness — an unsharded protocol, or one whose compilation was skipped
	// entirely, declares no γ cell to write to.
	if messagebus.HasSharedRandomness(run.System) {
		// γ lives on round 0 and must be written while the runtime is still there.
		messagebus.AssignSharedRandomnessSeed(run, sharedRandomness)
		// The trace columns live on the coin round rather than round 0, so their
		// assignment can no longer be what teaches each dynamic module its size:
		// AdvanceRound feeds those sizes into Fiat-Shamir on the way out of round
		// 0, before a single column has been assigned. Declare them from the trace
		// first, then step onto the coin round — AssignColumn requires the
		// runtime's current round to match the column's. Advancing is also what
		// absorbs γ into the transcript α and β are drawn from, which is the whole
		// point of putting γ on round 0.
		declareDynamicSizes(run, shard, schema)
		run.AdvanceRound()
	}

	eg := &errgroup.Group{}

	// Parallelize across modules
	for modID := range shard.Width() {
		eg.Go(func() error {

			trMod := shard.Module(modID)
			scMod := schema.Module(modID)

			if scMod.IsStatic() {
				// @alex: the current version of corset flags modules as being
				// static or not static. But it may be the case, that a module
				// has static size, some its column have static content but some
				// do not have static content.
				return nil
			}

			// Iterate each column in module
			parallel.Execute(int(trMod.Width()), func(start, stop int) {
				for id := start; id < stop; id++ {

					var (
						sys         = run.System
						columnIDMap = sys.Annotations[corsetColumnMapAnnotationKey].(map[string]wiop.ObjectID)
						col         = trMod.Column(uint(id))
						moduleName  = trMod.Name()
						name        = qualifiedCorsetName(moduleName, trMod.Descriptor().Columns[id].Name)
					)

					if _, ok := columnIDMap[name]; !ok {
						logrus.Debugf("zkcdriver: AssignFromTrace: skipping unknown column %q", name)
						continue
					}
					wCol := sys.LookupColumn(columnIDMap[name])

					// Use unsafe cast to avoid per-element Bytes()/SetBytes()
					// round-trip.
					plain := make([]field.Element, col.Len())
					for i := range plain {
						v := col.Get(uint(i))
						plain[i] = *(*field.Element)(unsafe.Pointer(&v))
					}

					// Done
					run.AssignColumn(
						wCol,
						&wiop.ConcreteVector{
							Plain: field.VecFromBase(plain),
						},
					)
				}
			})
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		logrus.Panicf("zkcdriver: AssignFromTrace failed: %v", err)
	}
}
