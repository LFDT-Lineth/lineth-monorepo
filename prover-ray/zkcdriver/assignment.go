package zkcdriver

import (
	"unsafe"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils/parallel"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/zkc/pkg/ir/air"
	"github.com/LFDT-Lineth/zkc/pkg/schema/register"
	"github.com/LFDT-Lineth/zkc/pkg/trace"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Compile-time check: unsafe cast between koalabear.Element and field.Element
// assumes identical layout ([1]uint32).
var _ [1]uint32 = koalabear.Element{}
var _ [1]uint32 = field.Element{}

// ReadExpandedTraces parses the provided trace file, expands it and returns the
// corset object holding the expanded traces.
func AssignFromTrace(
	run *wiop.Runtime,
	traces trace.Trace[koalabear.Element],
	schema air.Schema[koalabear.Element],
	sharedRandomness field.Octuplet,
) {

	// Only when the system was compiled to expect a γ. The driver does not choose
	// the compiler options, so it cannot assume the caller asked for shared
	// randomness — an unsharded protocol, or one whose compilation was skipped
	// entirely, declares no γ cell to write to.
	if messagebus.HasSharedRandomness(run.System) {
		messagebus.AssignSharedRandomnessSeed(run, sharedRandomness)
	}

	// Parallelize across modules
	eg := &errgroup.Group{}
	for modID := range traces.Width() {
		eg.Go(func() error {

			trMod := traces.Module(modID)
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
						columnName  = scMod.Register(register.NewId(uint(id))).Name()
						name        = qualifiedCorsetName(moduleName, columnName)
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
					var padding field.Element
					if scMod.AllowPadding() && len(plain) != 0 {
						// Expanded ZkC traces include a leading padding row. WIOP may
						// prepend more rows when rounding the module domain, and those
						// rows must repeat each column's valid padding value. In
						// particular, computed inverse columns do not generally pad
						// with zero.
						padding = plain[0]
					}

					// Done
					run.AssignColumn(
						wCol,
						&wiop.ConcreteVector{
							Plain:   field.VecFromBase(plain),
							Padding: padding,
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
