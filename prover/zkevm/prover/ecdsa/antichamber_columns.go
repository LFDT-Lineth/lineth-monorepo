package ecdsa

import "github.com/consensys/linea-monorepo/prover/protocol/ifaces"

func (ac *antichamber) cols(withActive bool) []ifaces.Column {
	res := []ifaces.Column{
		ac.ID,
		ac.IsPushing,
		ac.IsFetching,
		ac.Source,
	}
	if withActive {
		res = append(res, ac.IsActive)
	}
	return res
}

func (ec *EcRecover) cols() []ifaces.Column {
	return append(
		[]ifaces.Column{ec.EcRecoverID, ec.AuxProjectionMask},
		append(
			ec.Limb.GetLimbs(),
			ec.SuccessBit, ec.EcRecoverIndex, ec.EcRecoverIsData, ec.EcRecoverIsRes,
		)...,
	)
}

func (ad *Addresses) cols() []ifaces.Column {
	return append(ad.AddressLo.GetLimbs(), ad.AddressHiUntrimmed.GetLimbs()...)
}

func (ts *TxSignature) cols() []ifaces.Column {
	return append([]ifaces.Column{ts.IsTxHash}, ts.TxHash.GetLimbs()...)
}

func (ugd *UnalignedGnarkData) cols() []ifaces.Column {
	return append(
		[]ifaces.Column{
			ugd.IsPublicKey,
			ugd.GnarkIndex,
			ugd.GnarkPublicKeyIndex,
			ugd.IsEcrecoverAndFetching,
			ugd.IsNotPublicKeyAndPushing,
		},
		ugd.GnarkData.GetLimbs()...,
	)
}

func (ac *antichamber) unalignedGnarkDataSource() *unalignedGnarkDataSource {
	txHashHi, txHashLo := ac.TxHash.SplitOnBit(128)
	return &unalignedGnarkDataSource{
		IsActive:   ac.IsActive,
		IsPushing:  ac.IsPushing,
		IsFetching: ac.IsFetching,
		Source:     ac.Source,
		Limb:       ac.EcRecover.Limb,
		SuccessBit: ac.EcRecover.SuccessBit,
		IsData:     ac.EcRecover.EcRecoverIsData,
		IsRes:      ac.EcRecover.EcRecoverIsRes,
		TxHashHi:   txHashHi.AssertUint128(),
		TxHashLo:   txHashLo.AssertUint128(),
	}
}
