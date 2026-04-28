package zkcdriver

import (
	"unsafe"

	"github.com/consensys/go-corset/pkg/trace"
	"github.com/consensys/go-corset/pkg/util/field/koalabear"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/linea-monorepo/prover-ray/utils/parallel"
	"github.com/consensys/linea-monorepo/prover-ray/wiop"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Compile-time check: unsafe cast between koalabear.Element and field.Element
// assumes identical layout ([1]uint32).
var _ [1]uint32 = koalabear.Element{}
var _ [1]uint32 = field.Element{}

// ReadExpandedTraces parses the provided trace file, expands it and returns the
// corset object holding the expanded traces.
func AssignFromLtTraces(run *wiop.Runtime, expTraces trace.Trace[koalabear.Element]) {

	// Parallelize across modules
	eg := &errgroup.Group{}
	for modID := range expTraces.Width() {
		eg.Go(func() error {

			trMod := expTraces.Module(modID)

			// Iterate each column in module
			parallel.Execute(int(trMod.Width()), func(start, stop int) {
				for id := start; id < stop; id++ {

					var (
						sys         = run.System
						columnIDMap = sys.Annotations[corsetColumnMapAnnotationKey].(map[string]wiop.ObjectID)
						col         = trMod.Column(uint(id))
						moduleName  = trMod.Name().String()
						name        = qualifiedCorsetName(moduleName, col.Name())
					)

					if _, ok := columnIDMap[name]; !ok {
						logrus.Debugf("zkcdriver: AssignFromLtTraces: skipping unknown column %q", name)
						continue
					}

					var (
						wCol    = sys.LookupColumn(columnIDMap[name])
						padding field.Element
						data    = col.Data()
					)

					// Use unsafe cast to avoid per-element Bytes()/SetBytes()
					// round-trip.
					plain := make([]field.Element, data.Len())
					for i := range plain {
						v := data.Get(uint(i))
						plain[i] = *(*field.Element)(unsafe.Pointer(&v))
					}

					// Configure padding value
					pad := col.Padding()
					padding = *(*field.Element)(unsafe.Pointer(&pad))

					// Done
					run.AssignColumn(
						wCol,
						&wiop.ConcreteVector{Plain: field.VecFromBase(plain), Padding: padding},
					)
				}
			})
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		logrus.Panicf("AssignFromLtTraces failed: %v", err)
	}
}
