package serialization

import (
	"fmt"
	"math/big"
	"reflect"
	"sync"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/collection"
	"github.com/sirupsen/logrus"
)

var (
	DEBUG = false

	// For DEBUG mode uncomment this line
	//TypeTracker = make(map[string]bool)
)

type RawPrecomputed struct {
	//_         struct{}             `cbor:",toarray" serde:"omit"`
	ColID     ifaces.ColID         `cbor:"k"`
	ColAssign ifaces.ColAssignment `cbor:"v"`
}

type PackedPublicInput struct {
	Name string          `cbor:"n"`
	Acc  ifaces.Accessor `cbor:"a"`
}

type PackedExtradata struct {
	Key   string `cbor:"k"`
	Value any    `cbor:"v"`
}

type PackedQuery struct {
	BackReference                   BackReference `cbor:"b"`
	ConcreteType                    int           `cbor:"t"`
	Round                           int           `cbor:"r"`
	IsIgnored                       bool          `cbor:"i"`
	IsSkippedFromProverTranscript   bool          `cbor:"s"`
	IsSkippedFromVerifierTranscript bool          `cbor:"v"`
}

type PackedRawData struct {
	//_             struct{}           `cbor:",toarray" serde:"omit"`
	WasPointer    bool               `cbor:"p"`
	ConcreteType  int                `cbor:"t"`
	ConcreteValue PackedStructObject `cbor:"v"`
}

type packedIOPMetadata struct {
	dummyCompiled          bool
	withStorePointerChecks bool
	coinsLen               int
	selfRecursionCount     int
	precomputedLen         int
	publicinputLen         int
	extradataLen           int
	packedFSSetup          *big.Int
}

var (
	typeofRawPrecomputedData = reflect.TypeOf(RawPrecomputed{})
	typeofRawPublicInput     = reflect.TypeOf(PackedPublicInput{})
	typeofRawExtraData       = reflect.TypeOf(PackedExtradata{})
	typeofPcsCtxs            = reflect.TypeOf((*vortex.Ctx)(nil))
)

// PackedCompiledIOP represents the serialized form of CompiledIOP.
type PackedCompiledIOP struct {
	DummyCompiled          bool `cbor:"a"`
	WithStorePointerChecks bool `cbor:"b"`

	CoinsLen              int `cbor:"c"`
	QueryParamsOuterLen   int `cbor:"d"`
	QueryNoParamsOuterLen int `cbor:"e"`
	SelfRecursionCount    int `cbor:"f"`

	FiatShamirSetup *big.Int `cbor:"g"`

	Columns BackReference   `cbor:"h"`
	Coins   []BackReference `cbor:"i"`

	QueriesParams   []PackedQuery `cbor:"j"`
	QueriesNoParams []PackedQuery `cbor:"k"`

	SubProvers         [][]PackedRawData `cbor:"l"`
	SubVerifiers       [][]PackedRawData `cbor:"m"`
	FSHooksPreSampling [][]PackedRawData `cbor:"n"`

	Precomputed  []PackedStructObject `cbor:"o"`
	PublicInputs []PackedStructObject `cbor:"p"`
	ExtraData    []PackedStructObject `cbor:"q"`

	PcsCtxs any `cbor:"r"`
}

// -------------------- Packing --------------------

func newPackedCompiledIOP(packedMetadata packedIOPMetadata) *PackedCompiledIOP {

	return &PackedCompiledIOP{
		SelfRecursionCount:     packedMetadata.selfRecursionCount,
		DummyCompiled:          packedMetadata.dummyCompiled,
		WithStorePointerChecks: packedMetadata.withStorePointerChecks,
		FiatShamirSetup:        packedMetadata.packedFSSetup,
		Coins:                  make([]BackReference, packedMetadata.coinsLen),
		Precomputed:            make([]PackedStructObject, packedMetadata.precomputedLen),
		PublicInputs:           make([]PackedStructObject, packedMetadata.publicinputLen),
		ExtraData:              make([]PackedStructObject, packedMetadata.extradataLen),
	}
}

func (ser *Serializer) PackCompiledIOPFast(comp *wizard.CompiledIOP) (BackReference, *serdeError) {
	if comp == nil {
		return 0, nil
	}
	if idx, ok := ser.compiledIOPsFast[comp]; ok {
		return BackReference(idx), nil
	}
	refIdx := len(ser.PackedObject.CompiledIOPFast)
	ser.compiledIOPsFast[comp] = refIdx
	ser.PackedObject.CompiledIOPFast = append(ser.PackedObject.CompiledIOPFast, PackedCompiledIOP{})

	var (
		coinNames    = comp.Coins.AllKeys()
		qIDsParams   = comp.QueriesParams.AllKeys()
		qIDsNoParams = comp.QueriesNoParams.AllKeys()
		precomputed  = comp.Precomputed.ListAllKeys()
	)

	metadata := packedIOPMetadata{
		coinsLen:               len(coinNames),
		selfRecursionCount:     comp.SelfRecursionCount,
		precomputedLen:         len(precomputed),
		publicinputLen:         len(comp.PublicInputs),
		extradataLen:           len(comp.ExtraData),
		dummyCompiled:          comp.DummyCompiled,
		withStorePointerChecks: comp.WithStorePointerChecks,
		packedFSSetup:          comp.FiatShamirSetup.BigInt(fieldToSmallBigInt(comp.FiatShamirSetup)),
	}

	if DEBUG {
		logCompiledIOPMetadata(comp, "packing-acutal-compiled-IOP")
		logrus.Printf("Packed metadata: %+v", metadata)
	}

	packedComp := newPackedCompiledIOP(metadata)

	// Precomputed
	{
		for idx, colID := range precomputed {
			pre := RawPrecomputed{ColID: colID, ColAssign: comp.Precomputed.MustGet(colID)}
			obj, err := ser.PackStructObject(reflect.ValueOf(pre))
			if err != nil {
				return 0, err.wrapPath("(compiled-IOP-precomputed)")
			}
			packedComp.Precomputed[idx] = obj
		}
	}

	// PcsCtxs
	{
		if comp.PcsCtxs != nil {
			pcsAny, err := ser.PackValue(reflect.ValueOf(comp.PcsCtxs))
			if err != nil {
				return 0, err.wrapPath("(compiled-IOP-pcs-ctx)")
			}
			packedComp.PcsCtxs = pcsAny
		}
	}

	// Public inputs
	{
		for i, pi := range comp.PublicInputs {
			rawPI := PackedPublicInput{Name: pi.Name, Acc: pi.Acc}
			obj, err := ser.PackStructObject(reflect.ValueOf(rawPI))
			if err != nil {
				return 0, err.wrapPath("(compiled-IOP-public-inputs)")
			}
			packedComp.PublicInputs[i] = obj
		}
	}

	// Extra data (map order not fixed; preserved as-is)
	{
		i := 0
		for k, v := range comp.ExtraData {
			rawED := PackedExtradata{Key: k, Value: v}
			obj, err := ser.PackStructObject(reflect.ValueOf(rawED))
			if err != nil {
				return 0, err.wrapPath("(compiled-IOP-extra-data)")
			}
			packedComp.ExtraData[i] = obj
			i++
		}
	}

	// Column stores
	{
		backRefCol, err := ser.PackStore(comp.Columns)
		if err != nil {
			return 0, err.wrapPath("(compiled-IOP-columns-store)")
		}
		packedComp.Columns = backRefCol
	}

	// Coins
	{

		for i, name := range coinNames {
			c := comp.Coins.Data(name)
			br, err := ser.PackCoin(c)
			if err != nil {
				return 0, err.wrapPath("(compiled-IOP-coins)")
			}
			packedComp.Coins[i] = br
		}

		// IMPORTANT: Record the original per-IOP outer lengths so unpacker can restore exact shape.
		packedComp.CoinsLen = len(comp.Coins.ByRounds.Inner)
	}

	// Queries
	{
		if list, e := ser.packAllQueries(&comp.QueriesParams, qIDsParams, "query-params"); e != nil {
			return 0, e
		} else {
			packedComp.QueriesParams = list
		}
		if list, e := ser.packAllQueries(&comp.QueriesNoParams, qIDsNoParams, "query-no-params"); e != nil {
			return 0, e
		} else {
			packedComp.QueriesNoParams = list
		}

		// IMPORTANT: Record the original per-IOP outer lengths so unpacker can restore exact shape.
		packedComp.QueryParamsOuterLen = len(comp.QueriesParams.ByRounds.Inner)
		packedComp.QueryNoParamsOuterLen = len(comp.QueriesNoParams.ByRounds.Inner)
	}

	// Prover actions
	{
		proverActions := comp.SubProvers.Inner
		packedComp.SubProvers = make([][]PackedRawData, len(proverActions))
		for round, actPerRound := range proverActions {
			packedComp.SubProvers[round] = make([]PackedRawData, len(actPerRound))
			for i, act := range actPerRound {
				obj, serr, skipped := ser.packProverAction(act, "(compiled-IOP-subprovers[%d])", i)
				if serr != nil {
					return 0, serr
				}
				if skipped {
					continue
				}
				packedComp.SubProvers[round][i] = obj
			}
		}
	}

	// FS hooks pre-sampling
	{
		fsHookActions := comp.FiatShamirHooksPreSampling.Inner
		packedComp.FSHooksPreSampling = make([][]PackedRawData, len(fsHookActions))
		for round, actPerRound := range fsHookActions {
			packedComp.FSHooksPreSampling[round] = make([]PackedRawData, len(actPerRound))
			for i, act := range actPerRound {
				obj, serr, skipped := ser.packVerifierAction(act, "(compiled-IOP-fs-hooks-pre-sampling[%d])", i)
				if serr != nil {
					return 0, serr
				}
				if skipped {
					continue
				}
				packedComp.FSHooksPreSampling[round][i] = obj
			}
		}
	}

	// Verifier actions
	{
		verifierActions := comp.SubVerifiers.Inner
		packedComp.SubVerifiers = make([][]PackedRawData, len(verifierActions))
		for round, actPerRound := range verifierActions {
			packedComp.SubVerifiers[round] = make([]PackedRawData, len(actPerRound))
			for i, act := range actPerRound {
				obj, serr, skipped := ser.packVerifierAction(act, "(compiled-IOP-subverifiers[%d])", i)
				if serr != nil {
					return 0, serr
				}
				if skipped {
					continue
				}
				packedComp.SubVerifiers[round][i] = obj
			}
		}
	}

	ser.PackedObject.CompiledIOPFast[refIdx] = *packedComp
	return BackReference(refIdx), nil
}

// -------------------- unpacking --------------------

func newEmptyCompiledIOP(packedCompIOP PackedCompiledIOP) *wizard.CompiledIOP {
	deComp := &wizard.CompiledIOP{
		DummyCompiled:              packedCompIOP.DummyCompiled,
		SelfRecursionCount:         packedCompIOP.SelfRecursionCount,
		WithStorePointerChecks:     packedCompIOP.WithStorePointerChecks,
		Columns:                    column.NewStore(),
		Coins:                      wizard.NewRegister[coin.Name, coin.Info](),
		QueriesParams:              wizard.NewRegister[ifaces.QueryID, ifaces.Query](),
		QueriesNoParams:            wizard.NewRegister[ifaces.QueryID, ifaces.Query](),
		SubProvers:                 collection.VecVec[wizard.ProverAction]{},
		SubVerifiers:               collection.VecVec[wizard.VerifierAction]{},
		FiatShamirHooksPreSampling: collection.VecVec[wizard.VerifierAction]{},
		Precomputed:                collection.NewMapping[ifaces.ColID, ifaces.ColAssignment](),
		PublicInputs:               make([]wizard.PublicInput, len(packedCompIOP.PublicInputs), cap(packedCompIOP.PublicInputs)),
		ExtraData:                  make(map[string]any, len(packedCompIOP.ExtraData)),
		PcsCtxs:                    nil,
	}

	// Preallocate outer slices according to packed metadata
	if packedCompIOP.CoinsLen > 0 {
		deComp.Coins.ByRounds.Inner = make([][]coin.Name, packedCompIOP.CoinsLen)
	}

	if packedCompIOP.QueryParamsOuterLen > 0 {
		deComp.QueriesParams.ByRounds.Inner = make([][]ifaces.QueryID, packedCompIOP.QueryParamsOuterLen)
	}
	if packedCompIOP.QueryNoParamsOuterLen > 0 {
		deComp.QueriesNoParams.ByRounds.Inner = make([][]ifaces.QueryID, packedCompIOP.QueryNoParamsOuterLen)
	}

	return deComp
}

func (de *Deserializer) UnpackCompiledIOPFast(v BackReference) (reflect.Value, *serdeError) {
	if v < 0 || int(v) >= len(de.PackedObject.CompiledIOPFast) {
		return reflect.Value{}, newSerdeErrorf("invalid compiled-IOP backreference: %v", v)
	}
	if de.compiledIOPsFast[v] != nil {
		return reflect.ValueOf(de.compiledIOPsFast[v]), nil
	}
	packedCompIOP := de.PackedObject.CompiledIOPFast[v]

	// Reserve the cache and outer shapes up-front
	deComp := newEmptyCompiledIOP(packedCompIOP)
	de.compiledIOPsFast[v] = deComp

	var wg sync.WaitGroup
	errCh := make(chan *serdeError, 1)

	// Deserialize Columns sequentially so that other components can reference it concurrently
	{
		storeVal, err := de.UnpackStore(packedCompIOP.Columns)
		if err != nil {
			return reflect.Value{}, err.wrapPath("(deser. compiled-IOP-columns)")
		}
		deComp.Columns = storeVal.Interface().(*column.Store)
	}

	runParallel := func(fn func() *serdeError) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if se := fn(); se != nil {
				select {
				case errCh <- se:
				default:
				}
			}
		}()
	}

	// FiatShamirSetup
	runParallel(func() *serdeError {
		f, err := unmarshalBigInt(de, *packedCompIOP.FiatShamirSetup, TypeOfBigInt)
		if err != nil {
			return err.wrapPath("(deser compiled-IOP-fiatshamirsetup)")
		}
		deComp.FiatShamirSetup.SetBigInt(f.Interface().(*big.Int))
		return nil
	})

	// Precomputed
	runParallel(func() *serdeError {
		for _, rp := range packedCompIOP.Precomputed {
			res, err := de.UnpackStructObject(rp, typeofRawPrecomputedData)
			if err != nil {
				return newSerdeErrorf("could not unpack struct object for raw pre-computed data: %w", err)
			}
			pre, ok := res.Interface().(RawPrecomputed)
			if !ok {
				return newSerdeErrorf("could not cast to RawPrecomputed")
			}
			deComp.Precomputed.InsertNew(pre.ColID, pre.ColAssign)
		}
		return nil
	})

	// PcsCtxs
	runParallel(func() *serdeError {
		if packedCompIOP.PcsCtxs != nil {
			pcsVal, err := de.UnpackValue(packedCompIOP.PcsCtxs, typeofPcsCtxs)
			if err != nil {
				return err.wrapPath("(deser compiled-IOP-pcsctxs)")
			}
			deComp.PcsCtxs = pcsVal.Interface()
		} else {
			deComp.PcsCtxs = nil
		}
		return nil
	})

	// PublicInputs
	runParallel(func() *serdeError {
		for i, rpi := range packedCompIOP.PublicInputs {
			res, err := de.UnpackStructObject(rpi, typeofRawPublicInput)
			if err != nil {
				return newSerdeErrorf("could not unpack struct object for raw public input: %w", err)
			}
			pi, ok := res.Interface().(PackedPublicInput)
			if !ok {
				return newSerdeErrorf("could not cast to raw public input")
			}
			deComp.PublicInputs[i] = wizard.PublicInput{Name: pi.Name, Acc: pi.Acc}
		}
		return nil
	})

	// ExtraData
	runParallel(func() *serdeError {
		for _, red := range packedCompIOP.ExtraData {
			res, err := de.UnpackStructObject(red, typeofRawExtraData)
			if err != nil {
				return newSerdeErrorf("could not unpack struct object for raw extra data: %w", err)
			}
			ed, ok := res.Interface().(PackedExtradata)
			if !ok {
				return newSerdeErrorf("could not cast to raw extra data")
			}
			deComp.ExtraData[ed.Key] = ed.Value
		}
		return nil
	})

	// Coins
	runParallel(func() *serdeError {
		for _, br := range packedCompIOP.Coins {
			val, err := de.UnpackCoin(br)
			if err != nil {
				return err.wrapPath("(deser compiled-IOP-coins)")
			}
			info, ok := val.Interface().(coin.Info)
			if !ok {
				return newSerdeErrorf("illegal cast to coin.Info")
			}
			deComp.Coins.AddToRound(info.Round, info.Name, info)
		}
		return nil
	})

	// Queries params
	runParallel(func() *serdeError {
		if se := de.unpackAllQueries(&deComp.QueriesParams, packedCompIOP.QueriesParams, "params"); se != nil {
			return se
		}
		return nil
	})

	// queries no params
	runParallel(func() *serdeError {
		if se := de.unpackAllQueries(&deComp.QueriesNoParams, packedCompIOP.QueriesNoParams, "no-params"); se != nil {
			return se
		}
		return nil
	})

	// Actions & Hooks
	runParallel(func() *serdeError {
		deComp.SubProvers.Inner = make([][]wizard.ProverAction, len(packedCompIOP.SubProvers))
		if se := de.unpackAllProverActions(deComp.SubProvers.Inner, packedCompIOP.SubProvers); se != nil {
			return se
		}

		return nil
	})

	runParallel(func() *serdeError {
		deComp.FiatShamirHooksPreSampling.Inner = make([][]wizard.VerifierAction, len(packedCompIOP.FSHooksPreSampling))
		if se := de.unpackAllFSHooksPreSampling(deComp.FiatShamirHooksPreSampling.Inner, packedCompIOP.FSHooksPreSampling); se != nil {
			return se
		}

		return nil
	})

	runParallel(func() *serdeError {
		deComp.SubVerifiers.Inner = make([][]wizard.VerifierAction, len(packedCompIOP.SubVerifiers))
		if se := de.unpackAllVerifierActions(deComp.SubVerifiers.Inner, packedCompIOP.SubVerifiers); se != nil {
			return se
		}
		return nil
	})

	wg.Wait()

	select {
	case se := <-errCh:
		return reflect.Value{}, se
	default:
	}

	if DEBUG {
		logCompiledIOPMetadata(deComp, "unpacking-deserialized-comp-iop")
	}

	return reflect.ValueOf(de.compiledIOPsFast[v]), nil
}

// -------------------- Helper functions --------------------

func (ser *Serializer) packTypeIndex(concreteTypeStr string) int {
	if idx, ok := ser.typeMap[concreteTypeStr]; ok {
		return idx
	}
	ser.PackedObject.Types = append(ser.PackedObject.Types, concreteTypeStr)
	idx := len(ser.PackedObject.Types) - 1
	ser.typeMap[concreteTypeStr] = idx
	return idx
}

func checkRegisteredOrWarn(concreteTypeStr string, t reflect.Type) (*serdeError, bool) {
	if _, err := findRegisteredImplementation(concreteTypeStr); err != nil {

		if DEBUG {

			// UNCOMMENT this line for DEBUG mode
			// if !TypeTracker[concreteTypeStr] {
			// 	logrus.Warnf("attempted to serialize unregistered type repr=%q type=%v: %v", concreteTypeStr, t.String(), err)
			// 	TypeTracker[concreteTypeStr] = true
			// }
		}

		return newSerdeErrorf("attempted to serialize unregistered type repr=%q type=%v: %v", concreteTypeStr, t.String(), err), true
	}
	return nil, false
}

func (ser *Serializer) packAllActions(val any, pathFmt string, idx int) (PackedRawData, *serdeError, bool) {
	// orig is the original reflect.Value we received from the interface
	orig := reflect.ValueOf(val)
	wasPtr := false

	if !orig.IsValid() {
		return PackedRawData{}, newSerdeErrorf("invalid value passed to packActionCommon").wrapPath(fmt.Sprintf(pathFmt, idx)), false
	}

	// If the interface holds a pointer, remember that and work with the Elem()
	if orig.Kind() == reflect.Ptr {
		wasPtr = true
		if orig.IsNil() {
			// original was a nil pointer: we consider this "skipped" (same behaviour as before)
			return PackedRawData{}, nil, true
		}
		orig = orig.Elem()
	}

	// v is the non-pointer concrete value we actually pack
	v := orig

	concreteTypeStr := getPkgPathAndTypeNameIndirect(v.Interface())
	if serr, skipped := checkRegisteredOrWarn(concreteTypeStr, v.Type()); serr != nil || skipped {
		return PackedRawData{}, serr, true
	}
	typeIdx := ser.packTypeIndex(concreteTypeStr)

	switch v.Kind() {
	case reflect.Struct:
		obj, err := ser.PackStructObject(v)
		if err != nil {
			return PackedRawData{}, err.wrapPath(fmt.Sprintf(pathFmt, idx)), false
		}
		return PackedRawData{
			ConcreteType:  typeIdx,
			ConcreteValue: obj,
			WasPointer:    wasPtr,
		}, nil, false

	case reflect.Slice, reflect.Array:
		// logrus.Printf("Concrete type while packing action: %v", concreteTypeStr)
		obj, err := ser.PackArrayOrSlice(v)
		if err != nil {
			return PackedRawData{}, err.wrapPath(fmt.Sprintf(pathFmt, idx)), false
		}
		return PackedRawData{
			ConcreteType:  typeIdx,
			ConcreteValue: obj,
			WasPointer:    wasPtr,
		}, nil, false

	default:
		return PackedRawData{}, newSerdeErrorf("invalid action type: %v, expected struct or slice", v.Type()).wrapPath(fmt.Sprintf(pathFmt, idx)), false
	}
}

func (ser *Serializer) packProverAction(val wizard.ProverAction, pathFmt string, idx int) (PackedRawData, *serdeError, bool) {
	return ser.packAllActions(val, pathFmt, idx)
}
func (ser *Serializer) packVerifierAction(val wizard.VerifierAction, pathFmt string, idx int) (PackedRawData, *serdeError, bool) {
	return ser.packAllActions(val, pathFmt, idx)
}

// Query packers per register (no closures inside functions)
func (ser *Serializer) packAllQueries(reg *wizard.ByRoundRegister[ifaces.QueryID, ifaces.Query], qIDs []ifaces.QueryID,
	contextLabel string) ([]PackedQuery, *serdeError) {

	if len(qIDs) == 0 {
		return nil, nil
	}

	out := make([]PackedQuery, len(qIDs))
	for i, id := range qIDs {
		q := reg.Data(id)
		backRef, err := ser.PackQuery(q)
		if err != nil {
			return nil, err.wrapPath("(ser-compiled-IOP-queries-" + contextLabel + ")")
		}
		v := reflect.ValueOf(q)
		ct := getPkgPathAndTypeNameIndirect(v.Interface())
		if serr, _ := checkRegisteredOrWarn(ct, v.Type()); serr != nil {
			return nil, serr
		}
		typeIdx := ser.packTypeIndex(ct)
		out[i] = PackedQuery{
			BackReference:                   backRef,
			ConcreteType:                    typeIdx,
			Round:                           reg.Round(id),
			IsIgnored:                       reg.IsIgnored(id),
			IsSkippedFromProverTranscript:   reg.IsSkippedFromProverTranscript(id),
			IsSkippedFromVerifierTranscript: reg.IsSkippedFromVerifierTranscript(id),
		}
	}
	return out, nil
}

// -------------------- query/action unpack helpers --------------------

// Query unpackers per register (no closures inside functions)
func (de *Deserializer) unpackAllQueries(reg *wizard.ByRoundRegister[ifaces.QueryID, ifaces.Query],
	raws []PackedQuery, contextLabel string) *serdeError {

	for _, rq := range raws {
		ctStr := de.PackedObject.Types[rq.ConcreteType]
		ct, err := findRegisteredImplementation(ctStr)
		if err != nil {
			return newSerdeErrorf("could not find registered implementation for concrete type: %w", err)
		}

		val, se := de.UnpackQuery(rq.BackReference, ct)
		if se != nil {
			return se.wrapPath("(deser compiled-IOP-queries-" + contextLabel + ")")
		}

		q, ok := val.Interface().(ifaces.Query)
		if !ok {
			return newSerdeErrorf("illegal cast to ifaces.Query")
		}

		qID := q.Name()
		reg.AddToRound(rq.Round, qID, q)
		if rq.IsIgnored {
			reg.MarkAsIgnored(qID)
		}
		if rq.IsSkippedFromProverTranscript {
			reg.MarkAsSkippedFromProverTranscript(qID)
		}
		if rq.IsSkippedFromVerifierTranscript {
			reg.MarkAsSkippedFromVerifierTranscript(qID)
		}
	}
	return nil
}

// Go compiler automatically infers the type `T` from the caller signature
func unpackAllActions[T any](d *Deserializer, context string,
	out [][]T, actions2D [][]PackedRawData) *serdeError {

	for round, actions := range actions2D {
		if len(actions) == 0 {
			out[round] = nil
			continue
		}
		out[round] = make([]T, len(actions))

		var wg sync.WaitGroup
		errCh := make(chan *serdeError, 1)

		for idx, action := range actions {
			wg.Add(1)
			go func(idx int, action PackedRawData) {
				defer wg.Done()

				// These must be local to avoid races
				ctStr := d.PackedObject.Types[action.ConcreteType]
				ct, err := findRegisteredImplementation(ctStr)
				if err != nil {
					select {
					case errCh <- newSerdeErrorf(
						"could not find registered implementation for %s action: %w",
						context, err,
					):
					default:
					}
					return
				}

				var (
					res          reflect.Value
					se           *serdeError
					valInterface any
				)

				switch ct.Kind() {
				case reflect.Struct:
					res, se = d.UnpackStructObject(action.ConcreteValue, ct)
					if se != nil {
						select {
						case errCh <- se.wrapPath(fmt.Sprintf("(deser struct compiled-IOP-%s-actions)", context)):
						default:
						}
						return
					}
					if action.WasPointer {
						valInterface = res.Addr().Interface()
					} else {
						valInterface = res.Interface()
					}

				case reflect.Slice, reflect.Array:
					res, se = d.UnpackArrayOrSlice(action.ConcreteValue, ct)
					if se != nil {
						select {
						case errCh <- se.wrapPath(fmt.Sprintf("(deser slice/array compiled-IOP-%s-actions)", context)):
						default:
						}
						return
					}
					if action.WasPointer {
						ptr := reflect.New(res.Type())
						ptr.Elem().Set(res)
						valInterface = ptr.Interface()
					} else {
						valInterface = res.Interface()
					}

				default:
					select {
					case errCh <- newSerdeErrorf("unsupported kind:%v for %s action", ct.Kind(), context):
					default:
					}
					return
				}

				v, ok := valInterface.(T)
				if !ok {
					select {
					case errCh <- newSerdeErrorf(
						"illegal cast of type %v with string rep %s to %s action",
						ct, ctStr, context,
					):
					default:
					}
					return
				}

				out[round][idx] = v
			}(idx, action)
		}

		wg.Wait()

		// check if any goroutine reported error
		select {
		case e := <-errCh:
			return e
		default:
		}
	}

	return nil
}

func (de *Deserializer) unpackAllProverActions(deCompProverActions [][]wizard.ProverAction, actions2D [][]PackedRawData) *serdeError {
	return unpackAllActions(de, "prover", deCompProverActions, actions2D)
}

func (de *Deserializer) unpackAllVerifierActions(deCompVerifierActions [][]wizard.VerifierAction, actions2D [][]PackedRawData) *serdeError {
	return unpackAllActions(de, "verifier", deCompVerifierActions, actions2D)
}

func (de *Deserializer) unpackAllFSHooksPreSampling(deCompFSHooksPreSampling [][]wizard.VerifierAction, actions2D [][]PackedRawData) *serdeError {
	return unpackAllActions(de, "fshook", deCompFSHooksPreSampling, actions2D)
}

type printdata struct {
	numRounds               int
	coinsLen                int
	queryParamsOuterLen     int
	queryParamsInnerLen     []int
	queryNoParamsOuterLen   int
	queryNoParamsInnerLen   []int
	proverActionsOuterLen   int
	proverActionInnerLen    []int
	verifierActionsOuterLen int
	verifierActionsInnerLen []int
	fshooksOuterLen         int
	fshooksInnerLen         []int
}

// logCompiledIOPMetadata logs metadata about a CompiledIOP
func logCompiledIOPMetadata(comp *wizard.CompiledIOP, contextLabel string) {
	printdata1 := printdata{
		numRounds:               comp.NumRounds(),
		coinsLen:                len(comp.Coins.AllKeys()),
		queryParamsOuterLen:     len(comp.QueriesParams.ByRounds.Inner),
		queryNoParamsOuterLen:   len(comp.QueriesNoParams.ByRounds.Inner),
		proverActionsOuterLen:   len(comp.SubProvers.Inner),
		verifierActionsOuterLen: len(comp.SubVerifiers.Inner),
		fshooksOuterLen:         len(comp.FiatShamirHooksPreSampling.Inner),
	}

	for i := 0; i < printdata1.queryParamsOuterLen; i++ {
		printdata1.queryParamsInnerLen = append(printdata1.queryParamsInnerLen, len(comp.QueriesParams.ByRounds.Inner[i]))
	}

	for i := 0; i < printdata1.queryNoParamsOuterLen; i++ {
		printdata1.queryNoParamsInnerLen = append(printdata1.queryNoParamsInnerLen, len(comp.QueriesNoParams.ByRounds.Inner[i]))
	}

	for i := 0; i < printdata1.proverActionsOuterLen; i++ {
		printdata1.proverActionInnerLen = append(printdata1.proverActionInnerLen, len(comp.SubProvers.Inner[i]))
	}

	for i := 0; i < printdata1.verifierActionsOuterLen; i++ {
		printdata1.verifierActionsInnerLen = append(printdata1.verifierActionsInnerLen, len(comp.SubVerifiers.Inner[i]))
	}

	for i := 0; i < printdata1.fshooksOuterLen; i++ {
		printdata1.fshooksInnerLen = append(printdata1.fshooksInnerLen, len(comp.FiatShamirHooksPreSampling.Inner[i]))
	}

	logrus.Printf("%s comp metadata: %+v", contextLabel, printdata1)
}

/*
// PackCompiledIOP serializes a wizard.CompiledIOP, returning a BackReference to its index in PackedObject.CompiledIOP.
func (ser *Serializer) PackCompiledIOP(comp *wizard.CompiledIOP) (any, *serdeError) {
	if _, ok := ser.compiledIOPsFast[comp]; !ok {
		// We can have recursive references to compiled IOPs, so we need to
		// reserve the back-reference before attempting at unpacking it. That
		// way, the recursive attempts at packing will cache-hit without
		// creating an infinite loop.
		n := len(ser.PackedObject.CompiledIOPFast)
		ser.compiledIOPsFast[comp] = n
		ser.PackedObject.CompiledIOPFast = append(ser.PackedObject.CompiledIOPFast, nil)

		obj, err := ser.PackStructObject(reflect.ValueOf(*comp))
		if err != nil {
			return nil, err.wrapPath("(compiled-IOP)")
		}

		ser.PackedObject.CompiledIOPFast[n] = obj
	}

	return BackReference(ser.compiledIOPsFast[comp]), nil
}

// UnpackCompiledIOP deserializes a wizard.CompiledIOP from a BackReference, caching the result.
func (de *Deserializer) UnpackCompiledIOP(v BackReference) (reflect.Value, *serdeError) {
	if v < 0 || int(v) >= len(de.PackedObject.CompiledIOPFast) {
		return reflect.Value{}, newSerdeErrorf("invalid compiled-IOP backreference: %v", v)
	}

	if de.CompiledIOPsFast[v] == nil {

		// Something to be aware of is that CompiledIOPs usually contains
		// reference to themselves internally. Thus, if we don't cache a pointer
		// to the compiledIOP, the deserialization will go into an infinite loop.
		// To prevent that, we set a pointer to a zero value and it will be
		// cached when the compiled IOP is unpacked. The pointed value is then
		// assigned after the unpacking. With this approach, the ptr to the
		// compiledIOP can immediately be returned for the recursive calls.
		ptr := &wizard.CompiledIOP{}
		de.CompiledIOPsFast[v] = ptr

		packedCompiledIOP := de.PackedObject.CompiledIOPFast[v]
		compiledIOP, err := de.UnpackStructObject(packedCompiledIOP, TypeOfCompiledIOP)
		if err != nil {
			return reflect.Value{}, err.wrapPath("(compiled-IOP)")
		}

		c := compiledIOP.Interface().(wizard.CompiledIOP)
		*ptr = c
	}

	return reflect.ValueOf(de.CompiledIOPsFast[v]), nil
}

*/
