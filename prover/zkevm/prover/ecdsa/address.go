package ecdsa

import (
	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commoncs "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
)

// Address submodule is responsible for the columns holding the address of the sender,
// and checking their consistency with the claimed public key
// (since address is the truncated hash of public key).
//
// The addresses comes from two arithmetization modules txn-data and ec-data.
//
// The public-key comes from Gnark-Data.
type Addresses struct {

	// main columns
	AddressHiUntrimmed ifaces.Column
	AddressHi          ifaces.Column
	AddressLo          ifaces.Column

	// filters over address columns
	IsAddress            ifaces.Column
	IsAddressFromEcRec   ifaces.Column
	IsAddressFromTxnData ifaces.Column

	// helper columns for intermediate computations/proofs

	// filter over ecRecover; indicating only the AddressHi from EcRecoverIsRes
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

	// used as the hassID for hashing by keccak.
	HashNum ifaces.Column

	// columns for decomposition and trimming the HashHi to AddressHi
	LimbColumnsUntrimmed        byte32cmp.LimbColumns
	ComputeLimbColumnsUntrimmed wizard.ProverAction

	// providers for keccak, Providers contain the inputs and outputs of keccak hash.
	Provider generic.GenericByteModule
}

// AddressHi is the trimming of HashHi, taking its last 4bytes.
const trimmingSize = 4

// newAddress creates an Address struct, declaring native columns and the constraints among them.
func newAddress(comp *wizard.CompiledIOP, size int, ecRec *EcRecover, ac *antichamber, td *txnData) *Addresses {
	createCol := createColFn(comp, NAME_ADDRESSES, size)
	ecRecSize := ecRec.EcRecoverIsRes.Size()
	// declare the native columns
	addr := &Addresses{
		AddressHi:            createCol("ADDRESS_HI"),
		AddressLo:            createCol("ADDRESS_LO"),
		IsAddress:            createCol("IS_ADDRESS"),
		AddressHiUntrimmed:   createCol("ADRESSHI_UNTRIMMED"),
		Col16:                verifiercol.NewConstantCol(field.NewElement(16), size, "ecdsa-col16"),
		IsAddressHiEcRec:     comp.InsertCommit(0, ifaces.ColIDf("ISADRESS_HI_ECREC"), ecRecSize),
		IsAddressFromEcRec:   createCol("ISADRESS_FROM_ECREC"),
		IsAddressFromTxnData: createCol("ISADRESS_FROM_TXNDATA"),
		HashNum:              createCol("HASH_NUM"),
	}

	addr.IsAddressLoEcRec = dedicated.ManuallyShift(comp, addr.IsAddressHiEcRec, -1, "ISADRESS_LO_ECREC")

	td.csTxnData(comp)

	// addresses are fetched from two arithmetization modules (ecRecover and txn-data)
	// IsAddress = IsAdressFromEcRec + IsAdressFromTxnData
	comp.InsertGlobal(0, ifaces.QueryIDf("Format_IsAddress"),
		sym.Sub(addr.IsAddress, sym.Add(addr.IsAddressFromEcRec, addr.IsAddressFromTxnData)))

	commoncs.MustBeBinary(comp, addr.IsAddress)
	commoncs.MustBeBinary(comp, addr.IsAddressFromEcRec)
	commoncs.MustBeBinary(comp, addr.IsAddressFromTxnData)
	commoncs.MustZeroWhenInactive(comp, ac.IsActive,
		addr.IsAddress,
		addr.HashNum,
	)

	// check the  trimming of hashHi  to the addressHi
	addr.csAddressTrimming(comp)

	// check that IsAddressHiEcRec is well-formed
	addr.csIsAddressHiEcRec(comp, ecRec)

	// projection from ecRecover to address columns
	// ecdata is already projected over our ecRecover. Thus, we only project from our ecrecover.
	comp.InsertProjection(ifaces.QueryIDf("Project_AddressHi_EcRec"), query.ProjectionInput{ColumnA: []ifaces.Column{ecRec.Limb},
		ColumnB: []ifaces.Column{addr.AddressHi},
		FilterA: addr.IsAddressHiEcRec,
		FilterB: addr.IsAddressFromEcRec})

	comp.InsertProjection(ifaces.QueryIDf("Project_AddressLo_EcRec"),
		query.ProjectionInput{ColumnA: []ifaces.Column{ecRec.Limb},
			ColumnB: []ifaces.Column{addr.AddressLo},
			FilterA: addr.IsAddressLoEcRec.Natural,
			FilterB: addr.IsAddressFromEcRec})

	// projection from txn-data to address columns
	comp.InsertProjection(ifaces.QueryIDf("Project_AddressHi_TxnData"),
		query.ProjectionInput{ColumnA: []ifaces.Column{td.FromHi},
			ColumnB: []ifaces.Column{addr.AddressHi},
			FilterA: td.IsFrom,
			FilterB: addr.IsAddressFromTxnData})

	comp.InsertProjection(ifaces.QueryIDf("Project_AddressLO_TxnData"),
		query.ProjectionInput{ColumnA: []ifaces.Column{td.FromLo},
			ColumnB: []ifaces.Column{addr.AddressLo},
			FilterA: td.IsFrom,
			FilterB: addr.IsAddressFromTxnData})

	// impose that hashNum = ac.ID + 1
	comp.InsertGlobal(0, ifaces.QueryIDf("Hash_NUM_IS_ID"),
		sym.Mul(ac.IsActive,
			sym.Sub(addr.HashNum, ac.ID, 1)),
	)
	// assign the keccak provider
	addr.Provider = addr.GetProvider(comp, addr.HashNum, ac.UnalignedGnarkData)

	return addr
}

// It checks the well-forming of IsAddressHiEcRec
func (addr *Addresses) csIsAddressHiEcRec(comp *wizard.CompiledIOP, ecRec *EcRecover) {
	// if EcRecoverIsRes[i] == 1 and EcRecover[i+1] == 1 ---> isAddressHiEcRec[i] = 1
	comp.InsertGlobal(0, ifaces.QueryIDf("Is_AddressHi_EcRec_1"),
		sym.Mul(ecRec.EcRecoverIsRes, column.Shift(ecRec.EcRecoverIsRes, 1),
			sym.Sub(1, addr.IsAddressHiEcRec)))

	// if EcRecoverIsRes[i] == 0  ---> isAddressHiEcRec[i] = 0
	comp.InsertGlobal(0, ifaces.QueryIDf("Is_AddressHi_EcRec_2"),
		sym.Mul(sym.Sub(1, ecRec.EcRecoverIsRes), addr.IsAddressHiEcRec))

	// if EcRecoverIsRes[i] == 1 and EcRecover[i+1] == 0 ---> isAddressHiEcRec[i] = 0
	comp.InsertGlobal(0, ifaces.QueryIDf("Is_AddressHi_EcRec_3"),
		sym.Mul(ecRec.EcRecoverIsRes, sym.Sub(1, column.Shift(ecRec.EcRecoverIsRes, 1)),
			addr.IsAddressHiEcRec))
}

// The constraints for trimming the HashHi to AddressHi
func (addr *Addresses) csAddressTrimming(comp *wizard.CompiledIOP) {

	bitPerLimbs := 16
	addr.LimbColumnsUntrimmed, addr.ComputeLimbColumnsUntrimmed = byte32cmp.Decompose(comp, addr.AddressHiUntrimmed, 8, bitPerLimbs)
	// addr.LimbColumnsTrimmed, addr.computeLimbColumnsTrimmed = byte32cmp.Decompose(comp, addr.AddressHi, 2, 16)

	// recompose two first limbs to get AddressHi
	// since decomposition is in little-endian, but address is in big-endian, we get the first limbs.
	a := addr.LimbColumnsUntrimmed.Limbs[0]
	b := addr.LimbColumnsUntrimmed.Limbs[1]
	pow2 := sym.NewConstant(1 << bitPerLimbs)
	expr := sym.Add(a, sym.Mul(pow2, b))

	comp.InsertGlobal(0, ifaces.QueryIDf("Address_Trimming"), sym.Sub(expr, addr.AddressHi))
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
		Limb:    uaGnark.GnarkData,

		// a column of all 16, since all the bytes of public key are used in hashing
		NBytes: addr.Col16,
		Index:  uaGnark.GnarkPublicKeyIndex,
		ToHash: uaGnark.IsPublicKey,
	}

	pkModule.Info = generic.GenInfoModule{
		HashHi:   addr.AddressHiUntrimmed,
		HashLo:   addr.AddressLo,
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
	addr.assignMainColumns(run, nbEcRecover, size, uaGnark)
	addr.assignHelperColumns(run, ecRec)
}

// It assigns the main columns
func (addr *Addresses) assignMainColumns(
	run *wizard.ProverRuntime,
	nbEcRecover, size int,
	uaGnark *UnalignedGnarkData,
) {
	pkModule := addr.buildGenericModule(addr.HashNum, uaGnark)

	split := splitAt(nbEcRecover)
	n := nbRowsPerEcRec

	permTrace := keccak.GenerateTrace(pkModule.Data.ScanStreams(run))

	hashHi := make([]field.Element, 0, len(permTrace.HashOutPut))
	hashLo := make([]field.Element, 0, len(permTrace.HashOutPut))
	isHash := make([]field.Element, 0, len(permTrace.HashOutPut))
	trimmedHi := make([]field.Element, 0, len(permTrace.HashOutPut))

	var v, w, u field.Element
	for _, digest := range permTrace.HashOutPut {

		hi := digest[:halfDigest]
		lo := digest[halfDigest:]
		trimmed := hi[halfDigest-trimmingSize:]

		v.SetBytes(hi[:])
		w.SetBytes(lo[:])
		u.SetBytes(trimmed[:])

		if len(hashHi) == split {
			n = nbRowsPerTxSign
		}
		repeatLO := vector.Repeat(w, n)
		repeatHi := vector.Repeat(v, n)
		repeatTrimmedHi := vector.Repeat(u, n)
		repeatIsTxHash := vector.Repeat(field.Zero(), n-1)

		hashHi = append(hashHi, repeatHi...)
		hashLo = append(hashLo, repeatLO...)
		isHash = append(isHash, field.One())
		isHash = append(isHash, repeatIsTxHash...)
		trimmedHi = append(trimmedHi, repeatTrimmedHi...)
	}

	isFromEcRec := isHash[:split]
	isFromTxnData := vector.Repeat(field.Zero(), split)
	isFromTxnData = append(isFromTxnData, isHash[split:]...)

	run.AssignColumn(addr.AddressHiUntrimmed.GetColID(), smartvectors.RightZeroPadded(hashHi, size))
	run.AssignColumn(addr.AddressLo.GetColID(), smartvectors.RightZeroPadded(hashLo, size))
	run.AssignColumn(addr.IsAddress.GetColID(), smartvectors.RightZeroPadded(isHash, size))
	run.AssignColumn(addr.AddressHi.GetColID(), smartvectors.RightZeroPadded(trimmedHi, size))
	run.AssignColumn(addr.IsAddressFromEcRec.GetColID(), smartvectors.RightZeroPadded(isFromEcRec, size))
	run.AssignColumn(addr.IsAddressFromTxnData.GetColID(), smartvectors.RightZeroPadded(isFromTxnData, size))

}

// It assigns the helper columns
func (addr *Addresses) assignHelperColumns(run *wizard.ProverRuntime, ecRec *EcRecover) {

	// assign LimbColumns from decomposition via proverAction
	addr.ComputeLimbColumnsUntrimmed.Run(run)

	// assign isAddressHiEcRec
	isRes := ecRec.EcRecoverIsRes.GetColAssignment(run).IntoRegVecSaveAlloc()
	col := make([]field.Element, len(isRes))
	for i := 0; i < len(isRes); i++ {
		if i < len(isRes)-1 && isRes[i].IsOne() && isRes[i+1].IsOne() {
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
func splitAt(nbEcRecover int) int {
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
	FromHi ifaces.Column
	FromLo ifaces.Column
	Ct     ifaces.Column

	// helper column
	IsFrom    ifaces.Column
	Pa_IsZero wizard.ProverAction

	User     ifaces.Column
	Selector ifaces.Column
}
