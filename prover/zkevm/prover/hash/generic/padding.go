package generic

var (

	// UnspecifiedHashingUsecase is the zero value for the packing use-case and
	// it should not be used unless for testing that the caller of some function
	// does not pass it.
	UnspecifiedHashingUsecase = HashingUsecase{}

	// MiMCUsecase represents using MiMC with the Miyaguchi-Preneel construction
	// over a single field.
	MiMCUsecase = HashingUsecase{
		PaddingStrat:       zeroPadding,
		LaneSizeBytes_:     31,
		NbOfLanesPerBlock_: 1,
	}

	// KeccakUsecase represents using the Keccak hash function as in Ethereum
	KeccakUsecase = HashingUsecase{
		PaddingStrat:       keccakPadding,
		LaneSizeBytes_:     8,
		NbOfLanesPerBlock_: 17,
	}

	// Sha2Usecase represents using the Sha2 hash function.
	Sha2Usecase = HashingUsecase{
		PaddingStrat:       sha2Padding,
		LaneSizeBytes_:     4,
		NbOfLanesPerBlock_: 16,
	}
)

type paddingStrat int

const (
	zeroPadding paddingStrat = iota
	keccakPadding
	sha2Padding
)

type HashingUsecase struct {
	PaddingStrat       paddingStrat
	LaneSizeBytes_     int
	NbOfLanesPerBlock_ int
}

func (huc HashingUsecase) BlockSizeBytes() int {
	return huc.LaneSizeBytes_ * huc.NbOfLanesPerBlock_
}

func (h HashingUsecase) LaneSizeBytes() int {
	return h.LaneSizeBytes_
}

func (h HashingUsecase) NbOfLanesPerBlock() int {
	return h.NbOfLanesPerBlock_
}
