package ecdsa

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commoncs "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
)

const (
	// addressHiBytes is the size of the leftover from trimmed addressHi part (in bytes).
	addressHiBytes = 4
	// addressHiColumns is the number of addressHi columns.
	addressHiColumns = addressHiBytes / common.LimbBytes
	// addressTrimmedBytes size of the trimmed address part (in bytes).
	addressTrimmedBytes = common.NbLimbU256 - addressHiBytes
	// addressTrimmedColumns number of columns that represent the trimmed address part.
	addressTrimmedColumns = addressTrimmedBytes / common.LimbBytes
)

// Addresses submodule is responsible for the columns holding the address of the sender,
// and checking their consistency with the claimed public key
// (since address is the truncated hash of public key).
//
// The addresses comes from two arithmetization modules txn-data and ec-data.
//
// The public-key comes from Gnark-Data.
type Addresses struct {

	// main columns
	AddressLo          limbs.Uint128Le
	AddressHiUntrimmed limbs.Uint128Le

	// filters over address columns
	IsAddress            ifaces.Column
	IsAddressFromEcRec   ifaces.Column
	IsAddressFromTxnData ifaces.Column

	// helper columns for intermediate computations/proofs

	// filter over ecRecover; indicating only the AddressHi limbs from EcRecoverIsRes
	// we need this columns just because projection query does not support expressions
	// as filter. The shifted version of this column by 1 indicates the addressLo from
	// ecrec.
	IsAddressHiEcRec ifaces.Column

	// IsAddressLoEcRec is a column that is explicitly constructed as the shifted version
	// of IsAddressLoEcRec. The reason we need to build this column is because projection
	// queries cannot be distributed if their selector is a shifted column. (There would
	// be a need to deal with the segment boundaries and it has not been figured out.)
	// So, to simplify, we create a column directly and leave the business of managing
	// the boundary values to a global constraint to which the feature is avaiable.
	IsAddressLoEcRec *dedicated.ManuallyShifted

	// a column of all 16 indicating that all 16 bytes of public key should be hashed.
	Col16 ifaces.Column

	// used as the hashID for hashing by keccak.
	HashNum ifaces.Column

	// providers for keccak, Providers contain the inputs and outputs of keccak hash.
	Provider generic.GenericByteModule
}

// Addresses returns the addresses in a limbs.Uint160Le
// in LE form.
func (addr *Addresses) Addresses() limbs.Uint160Le {

	_, address := limbs.FuseLimbs(
		addr.AddressHiUntrimmed.AsDynSize(),
		addr.AddressLo.AsDynSize(),
	).SplitOnBit(160)

	return address.AssertUint160()
}

// newAddress creates an Address struct, declaring native columns and the constraints among them.
func newAddress(comp *wizard.CompiledIOP, size int, ecRec *EcRecover, ac *antichamber, td *txnData) *Addresses {
	createCol := createColFn(comp, NAME_ADDRESSES, size)
	ecRecSize := ecRec.EcRecoverIsRes.Size()

	addressLo := limbs.NewUint128Le(
		comp,
		"ADDRESS_LO",
		size,
		pragmas.PragmaPair{Pragma: pragmas.RightPadded, Value: true},
	)

	addressHiUntrimmed := limbs.NewUint128Le(
		comp,
		"ADDRESS_HI_UNTRIMMED",
		size,
		pragmas.RightPaddedPair,
	)

	// declare the native columns
	addr := &Addresses{
		AddressLo:            addressLo,
		AddressHiUntrimmed:   addressHiUntrimmed,
		IsAddress:            createCol("IS_ADDRESS"),
		Col16:                verifiercol.NewConstantCol(field.NewElement(16), size, "ecdsa-col16"),
		IsAddressHiEcRec:     comp.InsertCommit(0, ifaces.ColIDf("ISADDRESS_HI_ECREC"), ecRecSize, true),
		IsAddressFromEcRec:   createCol("ISADDRESS_FROM_ECREC"),
		IsAddressFromTxnData: createCol("ISADDRESS_FROM_TXNDATA"),
		HashNum:              createCol("HASH_NUM"),
	}

	addr.IsAddressLoEcRec = dedicated.ManuallyShift(comp, addr.IsAddressHiEcRec, -1, "ISADDRESS_LO_ECREC")

	td.csTxnData(comp)

	// addresses are fetched from two arithmetization modules (ecRecover and txn-data)
	// IsAddress = IsAddressFromEcRec + IsAddressFromTxnData
	comp.InsertGlobal(0, ifaces.QueryIDf("Format_IsAddress"),
		sym.Sub(addr.IsAddress, sym.Add(addr.IsAddressFromEcRec, addr.IsAddressFromTxnData)))

	commoncs.MustBeBinary(comp, addr.IsAddress)
	commoncs.MustBeBinary(comp, addr.IsAddressFromEcRec)
	commoncs.MustBeBinary(comp, addr.IsAddressFromTxnData)
	commoncs.MustZeroWhenInactive(comp, ac.IsActive,
		addr.IsAddress,
		addr.HashNum,
	)

	// check that IsAddressHiEcRec is well-formed
	addr.csIsAddressHiEcRec(comp, ecRec)

	// projection from ecRecover to address columns
	// ecdata is already projected over our ecRecover. Thus, we only project from our ecrecover.

	// Check that first 6 elements (trimmed 12 bytes) of address higher part are all 0
	limbs.NewGlobal(
		comp,
		"Trimmed_Bytes_Zeros",
		sym.Mul(addr.IsAddressHiEcRec, addr.IsAddressFromEcRec, ecRec.Limb.SliceOnBit(0, 96)))

	comp.InsertProjection(ifaces.QueryIDf("Project_AddressHi_EcRec"),
		query.ProjectionInput{
			ColumnA: ecRec.Limb.SliceOnBit(96, 128).ToLittleEndianLimbs().GetLimbs(),
			ColumnB: addr.AddressHiUntrimmed.SliceOnBit(96, 128).ToLittleEndianLimbs().GetLimbs(),
			FilterA: addr.IsAddressHiEcRec,
			FilterB: addr.IsAddressFromEcRec,
		},
	)

	comp.InsertProjection(ifaces.QueryIDf("Project_AddressLo_EcRec"),
		query.ProjectionInput{
			ColumnA: ecRec.Limb.ToBigEndianLimbs().GetLimbs(),
			ColumnB: addr.AddressLo.ToBigEndianLimbs().GetLimbs(),
			FilterA: addr.IsAddressLoEcRec.Natural,
			FilterB: addr.IsAddressFromEcRec,
		},
	)

	// impose that hashNum = ac.ID + 1
	comp.InsertGlobal(0, ifaces.QueryIDf("Hash_NUM_IS_ID"),
		sym.Mul(ac.IsActive,
			sym.Sub(addr.HashNum, ac.ID, 1)),
	)

	// assign the keccak provider
	addr.Provider = addr.GetProvider(comp, addr.HashNum, ac.UnalignedGnarkData)

	return addr
}

// It checks the well-forming of IsAddressHiEcRec.
// IsAddressHiEcRec marks the first of the two consecutive result-rows in an
// ECRECOVER frame — but only when SUCCESS_BIT=1. For failed ECRECOVERs (the
// EVM precompile returned 0x00..00) the address-projection is skipped, so
// IsAddressHiEcRec must be 0 on those rows.
func (addr *Addresses) csIsAddressHiEcRec(comp *wizard.CompiledIOP, ecRec *EcRecover) {
	// if EcRecoverIsRes[i] == 1 and EcRecoverIsRes[i+1] == 1 and SuccessBit[i] == 1 ---> isAddressHiEcRec[i] = 1
	comp.InsertGlobal(0, ifaces.QueryIDf("Is_AddressHi_EcRec_1"),
		sym.Mul(ecRec.EcRecoverIsRes, column.Shift(ecRec.EcRecoverIsRes, 1), ecRec.SuccessBit,
			sym.Sub(1, addr.IsAddressHiEcRec)))

	// if EcRecoverIsRes[i] == 0  ---> isAddressHiEcRec[i] = 0
	comp.InsertGlobal(0, ifaces.QueryIDf("Is_AddressHi_EcRec_2"),
		sym.Mul(sym.Sub(1, ecRec.EcRecoverIsRes), addr.IsAddressHiEcRec))

	// if EcRecoverIsRes[i] == 1 and EcRecoverIsRes[i+1] == 0 ---> isAddressHiEcRec[i] = 0
	comp.InsertGlobal(0, ifaces.QueryIDf("Is_AddressHi_EcRec_3"),
		sym.Mul(ecRec.EcRecoverIsRes, sym.Sub(1, column.Shift(ecRec.EcRecoverIsRes, 1)),
			addr.IsAddressHiEcRec))

	// if EcRecoverIsRes[i] == 1 and EcRecoverIsRes[i+1] == 1 and SuccessBit[i] == 0 ---> isAddressHiEcRec[i] = 0
	comp.InsertGlobal(0, ifaces.QueryIDf("Is_AddressHi_EcRec_4"),
		sym.Mul(ecRec.EcRecoverIsRes, column.Shift(ecRec.EcRecoverIsRes, 1),
			sym.Sub(1, ecRec.SuccessBit),
			addr.IsAddressHiEcRec))
}

// It builds a provider from  public key extracted from Gnark-Data (as hash input) and addresses (as output).
// the consistency check is then deferred to the keccak module.
func (addr *Addresses) GetProvider(comp *wizard.CompiledIOP, id ifaces.Column, uaGnark *UnalignedGnarkData) generic.GenericByteModule {
	// generate a generic byte Module as keccak provider.
	provider := addr.buildGenericModule(id, uaGnark)
	return provider
}

// It builds a GenericByteModule from Address columns and Public-Key/GnarkData columns.
func (addr *Addresses) buildGenericModule(id ifaces.Column, uaGnark *UnalignedGnarkData) (pkModule generic.GenericByteModule) {

	pkModule.Data = generic.GenDataModule{
		HashNum: id,
		Limbs:   uaGnark.GnarkData.ToBigEndianUint(),

		// a column of all 16, since all the bytes of public key are used in hashing
		NBytes: addr.Col16,
		Index:  uaGnark.GnarkPublicKeyIndex,
		// IsPkHashFilter matches IsPublicKey everywhere except on pk-rows of
		// failed ECRECOVER frames (SUCCESS_BIT=0), where it is 0 to skip the
		// keccak invocation — see UnalignedGnarkData.IsPkHashFilter.
		ToHash: uaGnark.IsPkHashFilter,
	}

	pkModule.Info = generic.GenInfoModule{
		HashHi:   addr.AddressHiUntrimmed.ToBigEndianUint(),
		HashLo:   addr.AddressLo.ToBigEndianUint(),
		IsHashHi: addr.IsAddress,
		IsHashLo: addr.IsAddress,
	}
	return pkModule
}

// It assigns the native columns specific to the submodule.
func (addr *Addresses) assignAddress(
	run *wizard.ProverRuntime,
	nbEcRecover, size int,
	ac *antichamber,
	ecRec *EcRecover,
	uaGnark *UnalignedGnarkData,
	td *txnData,
) {
	// assign td.isFrom
	td.Pa_IsZero.Run(run)

	// assign HashNum
	var (
		one      = field.One()
		id       = ac.ID.GetColAssignment(run).IntoRegVecSaveAlloc()
		isActive = ac.IsActive.GetColAssignment(run).IntoRegVecSaveAlloc()
		hashNum  = common.NewVectorBuilder(addr.HashNum)
	)

	for row := range id {
		if isActive[row].IsOne() {
			f := *new(field.Element).Add(&id[row], &one)
			hashNum.PushField(f)
		} else {
			hashNum.PushInt(0)
		}
	}

	hashNum.PadAndAssign(run)
	addr.assignMainColumns(run, nbEcRecover, size, uaGnark, ecRec)
	addr.assignHelperColumns(run, ecRec)
}

// It assigns the main columns
func (addr *Addresses) assignMainColumns(
	run *wizard.ProverRuntime,
	nbEcRecover, size int,
	uaGnark *UnalignedGnarkData,
	ecRec *EcRecover,
) {

	var (
		pkModule             = addr.buildGenericModule(addr.HashNum, uaGnark)
		split                = totalNbEcRecoverRows(nbEcRecover)
		streams              = pkModule.Data.ScanStreams(run)
		permTrace            = keccak.GenerateTrace(streams)
		addressUntrimmedHi   = limbs.NewVectorBuilder(addr.AddressHiUntrimmed.AsDynSize())
		addressLo            = limbs.NewVectorBuilder(addr.AddressLo.AsDynSize())
		isHash               = common.NewVectorBuilder(addr.IsAddress)
		isAddressFromTxnData = common.NewVectorBuilder(addr.IsAddressFromTxnData)
		isAddressFromEcRec   = common.NewVectorBuilder(addr.IsAddressFromEcRec)
	)

	// Pre-compute SUCCESS_BIT per ECRECOVER frame. Frames are laid out
	// contiguously at offsets i*nbRowsPerEcRec (i in [0, nbEcRecover)).
	// SUCCESS_BIT is constant within a frame, so reading the first row is
	// sufficient. This is used below to skip the keccak digest for failed
	// ECRECOVERs so that IsAddressFromEcRec=0 on those rows and the
	// Project_Address{Hi,Lo}_EcRec projections stay balanced.
	successBits := ecRec.SuccessBit.GetColAssignment(run).IntoRegVecSaveAlloc()
	frameSuccessful := make([]bool, nbEcRecover)
	for i := 0; i < nbEcRecover; i++ {
		frameSuccessful[i] = successBits[i*nbRowsPerEcRec].IsOne()
	}

	// `permTrace.HashOutPut` holds one digest per hashed stream. With the
	// IsPkHashFilter gate in UnalignedGnarkData, failed-ECRECOVER frames
	// produce NO stream, so those frames do not appear here. Iterate the
	// ECRECOVER slots explicitly (consuming a digest only for successful
	// frames, and pushing zeros with isHash=0 for failed frames), then
	// iterate the remaining digests for TX-signature frames.
	digestIdx := 0
	for i := 0; i < nbEcRecover; i++ {
		if frameSuccessful[i] {
			digest := permTrace.HashOutPut[digestIdx]
			digestIdx++
			addressUntrimmedHi.PushRepeatBytes(digest[:16], nbRowsPerEcRec)
			addressLo.PushRepeatBytes(digest[16:], nbRowsPerEcRec)
			isHash.PushOne()
			isHash.PushSeqOfZeroes(nbRowsPerEcRec - 1)
		} else {
			// Failed ECRECOVER: no stream hashed, address row block is zero.
			addressUntrimmedHi.PushSeqOfZeroes(nbRowsPerEcRec)
			addressLo.PushSeqOfZeroes(nbRowsPerEcRec)
			isHash.PushSeqOfZeroes(nbRowsPerEcRec)
		}
	}
	for digestIdx < len(permTrace.HashOutPut) {
		digest := permTrace.HashOutPut[digestIdx]
		digestIdx++
		addressUntrimmedHi.PushRepeatBytes(digest[:16], nbRowsPerTxSign)
		addressLo.PushRepeatBytes(digest[16:], nbRowsPerTxSign)
		isHash.PushOne()
		isHash.PushSeqOfZeroes(nbRowsPerTxSign - 1)
	}

	isAddressFromEcRec.PushMany(isHash.Slice()[:split])
	isAddressFromTxnData.PushSeqOfZeroes(split)
	isAddressFromTxnData.PushMany(isHash.Slice()[split:])

	isAddressFromEcRec.PadAndAssign(run)
	isAddressFromTxnData.PadAndAssign(run)
	isHash.PadAndAssign(run)
	addressLo.PadAndAssignZero(run)
	addressUntrimmedHi.PadAndAssignZero(run)
}

// It assigns the helper columns
func (addr *Addresses) assignHelperColumns(run *wizard.ProverRuntime, ecRec *EcRecover) {

	// assign isAddressHiEcRec: mark the first of two consecutive IsRes rows,
	// but only for frames whose SUCCESS_BIT is 1 — failed ECRECOVERs are
	// excluded from the address projection.
	isRes := ecRec.EcRecoverIsRes.GetColAssignment(run).IntoRegVecSaveAlloc()
	successBit := ecRec.SuccessBit.GetColAssignment(run).IntoRegVecSaveAlloc()
	col := make([]field.Element, len(isRes))
	for i := 0; i < len(isRes); i++ {
		if i < len(isRes)-1 && isRes[i].IsOne() && isRes[i+1].IsOne() && successBit[i].IsOne() {
			col[i] = field.One()
			col[i+1] = field.Zero()
			i = i + 1
		}
	}

	run.AssignColumn(addr.IsAddressHiEcRec.GetColID(), smartvectors.NewRegular(col))

	// this calls the auto-assign feature of the
	_ = addr.IsAddressLoEcRec.GetColAssignment(run)
}

// It indicates the row where ecrecover and txSignature are split.
func totalNbEcRecoverRows(nbEcRecover int) int {
	return nbEcRecover * nbRowsPerEcRec
}

func (td *txnData) csTxnData(comp *wizard.CompiledIOP) {

	//  isFrom == 1 iff ct==0 && User==1 && HubSelector==1
	//  (Ct-0)^2 + (User-1)^2 + (HubSelector-1)^2 = 0
	condition := sym.Add(
		sym.Square(sym.Sub(td.Ct, 0)),
		sym.Square(sym.Sub(td.User, 1)),
		sym.Square(sym.Sub(td.Selector, 1)),
	)

	td.IsFrom, td.Pa_IsZero = dedicated.IsZero(comp, condition).GetColumnAndProverAction()

}

// txndata represents the txn_data module from the arithmetization side.
type txnData struct {
	From limbs.Uint256Le
	Ct   ifaces.Column

	// helper column
	IsFrom    ifaces.Column
	Pa_IsZero wizard.ProverAction

	User     ifaces.Column
	Selector ifaces.Column
}
