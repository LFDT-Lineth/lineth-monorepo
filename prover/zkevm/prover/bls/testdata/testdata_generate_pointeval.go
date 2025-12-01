package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"iter"
	"math/big"
	"os"
	"slices"
	"strconv"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	fp_bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fp"
	fr_bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	kzg_bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381/kzg"
)

const (
	mMask                 byte = 0b111 << 5
	mUncompressed         byte = 0b000 << 5
	_                     byte = 0b001 << 5 // invalid
	mUncompressedInfinity byte = 0b010 << 5
	_                     byte = 0b011 << 5 // invalid
	mCompressedSmallest   byte = 0b100 << 5
	mCompressedLargest    byte = 0b101 << 5
	mCompressedInfinity   byte = 0b110 << 5
	_                     byte = 0b111 << 5 // invalid
)

const (
	// blobCommitmentVersionKZG is the version byte for the KZG point evaluation precompile.
	blobCommitmentVersionKZG uint8 = 0x01
	// evmBlockSize is the size of the SRS used in the KZG precompile. This
	// defines the polynomial degree and therefore the size of the blob. It is also the expected
	// return value of the POINTEVAL precompile.
	evmBlockSize = 4096
)

var (
	evmBlsModulus = fr_bls12381.Modulus()
)

type pointEvalInput struct {
	versionedHashCase      pointEvalInputHashType
	evaluationPointCase    pointEvalInputScalarType
	claimedValueCase       pointEvalInputScalarType
	commitmentCase         pointEvalInputG1ElementType
	proofCase              pointEvalInputG1ElementType
	expectedBlobSizeCase   pointEvalInputExpectedBlobSizeType
	expectedBlsModulusCase pointEvalInputExpectedModulusType

	inputType pointEvalInputType // it is usually invalid, only in case where previous fields are valid in which case we toggle this value

	VersionedHash      [32]byte
	EvaluationPoint    *big.Int
	ClaimedValue       *big.Int
	Commitment         [bls12381.SizeOfG1AffineCompressed]byte
	Proof              [bls12381.SizeOfG1AffineCompressed]byte
	ExpectedBlobSize   *big.Int
	ExpectedBlsModulus *big.Int

	// arithmetization determines where to send the input to. In case evaluation
	// point or claimed value overflows, or the expected results are incorrect, it is sent to neither.
	ToPointEvalCircuit        bool // if the input is valid, we send it to the point evaluation circuit
	ToPointEvalInvalidCircuit bool // if the input is invalid, we send it to the invalid circuit
}

func generateInvalidCompressedPoint(c pointEvalInputG1ElementType) [bls12381.SizeOfG1AffineCompressed]byte {
	var res [bls12381.SizeOfG1AffineCompressed]byte
	switch c {
	case invalidPointEvalG1_MaskYLarge_YSmall:
		var p bls12381.G1Affine
		var s fr_bls12381.Element
		b := new(big.Int)
		_, _, g, _ := bls12381.Generators()
		for {
			s.MustSetRandom()
			p.ScalarMultiplication(&g, s.BigInt(b))
			if !p.Y.LexicographicallyLargest() {
				break
			}
		}
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&mMask != mCompressedSmallest {
			panic("mask should be for smallest y coordinate")
		}
		// swap out the mask
		pMarshalled[0] = (pMarshalled[0] &^ mMask) | mCompressedLargest // modify the mask
		res = pMarshalled
	case invalidPointEvalG1_MaskYSmall_YLarge:
		var p bls12381.G1Affine
		var s fr_bls12381.Element
		b := new(big.Int)
		_, _, g, _ := bls12381.Generators()
		for {
			s.MustSetRandom()
			p.ScalarMultiplication(&g, s.BigInt(b))
			if p.Y.LexicographicallyLargest() {
				break
			}
		}
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&mMask != mCompressedLargest {
			panic("mask should be for largest y coordinate")
		}
		// swap out the mask
		pMarshalled[0] = (pMarshalled[0] &^ mMask) | mCompressedSmallest // modify the mask
		res = pMarshalled
	case invalidPointEvalG1_MaskInfinity_NotInfinity:
		var p bls12381.G1Affine
		var s fr_bls12381.Element
		b := new(big.Int)
		_, _, g, _ := bls12381.Generators()
		s.MustSetRandom()
		p.ScalarMultiplication(&g, s.BigInt(b))
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&(0b110<<5) != byte(0b100)<<5 {
			panic("mask should be for compressed point")
		}
		// swap out the mask
		pMarshalled[0] = (pMarshalled[0] &^ mMask) | mCompressedInfinity // modify the mask
		res = pMarshalled
	case invalidPointEvalG1_MaskSmallY_Infinity:
		var p bls12381.G1Affine
		p.SetInfinity()
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&mMask != mCompressedInfinity {
			panic("mask should be for compressed infinity point")
		}
		// swap out the mask
		pMarshalled[0] = (pMarshalled[0] &^ mMask) | mCompressedSmallest // modify the mask
		res = pMarshalled
	case invalidPointEvalG1_MaskLargeY_Infinity:
		var p bls12381.G1Affine
		p.SetInfinity()
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&mMask != mCompressedInfinity {
			panic("mask should be for compressed infinity point")
		}
		// swap out the mask
		pMarshalled[0] = (pMarshalled[0] &^ mMask) | mCompressedLargest // modify the mask
		res = pMarshalled
	case invalidPointEvalG1_MaskValid_NotInGroup:
		var p bls12381.G1Affine
		var s fp_bls12381.Element
		s.MustSetRandom()
		pj := bls12381.GeneratePointNotInG1(s)
		p.FromJacobian(&pj)
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&(0b110<<5) != byte(0b100)<<5 {
			panic("mask should be for compressed regular point")
		}
		res = pMarshalled
	case invalidPointEvalG1_MaskValid_NotOnCurve:
		var p bls12381.G1Affine
		for {
			p.X.SetRandom()
			p.Y.SetRandom()
			if !p.IsOnCurve() {
				break
			}
		}
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&(0b110<<5) != byte(0b100)<<5 {
			panic("mask should be for compressed regular point")
		}
		res = pMarshalled
	case invalidPointEvalG1_Mask0b000_Random:
		var p bls12381.G1Affine
		var s fr_bls12381.Element
		b := new(big.Int)
		_, _, g, _ := bls12381.Generators()
		s.MustSetRandom()
		p.ScalarMultiplication(&g, s.BigInt(b))
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&(0b110<<5) != byte(0b100)<<5 {
			panic("mask should be for compressed regular point")
		}
		// swap out the mask
		pMarshalled[0] = (pMarshalled[0] &^ mMask) | mUncompressed // modify the mask
		res = pMarshalled
	case invalidPointEvalG1_Mask0b010_Random:
		var p bls12381.G1Affine
		var s fr_bls12381.Element
		b := new(big.Int)
		_, _, g, _ := bls12381.Generators()
		s.MustSetRandom()
		p.ScalarMultiplication(&g, s.BigInt(b))
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&(0b110<<5) != byte(0b100)<<5 {
			panic("mask should be for compressed regular point")
		}
		// swap out the mask
		pMarshalled[0] = (pMarshalled[0] &^ mMask) | mUncompressedInfinity // modify the mask
		res = pMarshalled
	case invalidPointEvalG1_Mask0b001_Random:
		var p bls12381.G1Affine
		var s fr_bls12381.Element
		b := new(big.Int)
		_, _, g, _ := bls12381.Generators()
		s.MustSetRandom()
		p.ScalarMultiplication(&g, s.BigInt(b))
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&(0b110<<5) != byte(0b100)<<5 {
			panic("mask should be for compressed regular point")
		}
		// swap out the mask
		pMarshalled[0] = (pMarshalled[0] &^ mMask) | (0b001 << 5) // modify the mask
		res = pMarshalled
	case invalidPointEvalG1_Mask0b011_Random:
		var p bls12381.G1Affine
		var s fr_bls12381.Element
		b := new(big.Int)
		_, _, g, _ := bls12381.Generators()
		s.MustSetRandom()
		p.ScalarMultiplication(&g, s.BigInt(b))
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&(0b110<<5) != byte(0b100)<<5 {
			panic("mask should be for compressed regular point")
		}
		// swap out the mask
		pMarshalled[0] = (pMarshalled[0] &^ mMask) | (0b011 << 5) // modify the mask
		res = pMarshalled
	case invalidPointEvalG1_Mask0b111_Random:
		var p bls12381.G1Affine
		var s fr_bls12381.Element
		b := new(big.Int)
		_, _, g, _ := bls12381.Generators()
		s.MustSetRandom()
		p.ScalarMultiplication(&g, s.BigInt(b))
		pMarshalled := p.Bytes()
		// check that mask is what we expect
		if pMarshalled[0]&(0b110<<5) != byte(0b100)<<5 {
			panic("mask should be for compressed regular point")
		}
		// swap out the mask
		pMarshalled[0] = (pMarshalled[0] &^ mMask) | (0b111 << 5) // modify the mask
		res = pMarshalled
	// case invalidPointEvalG1_Mask0b010_Infinity:
	// case invalidPointEvalG1_Mask0b001_Infinity:
	// case invalidPointEvalG1_Mask0b011_Infinity:
	// case invalidPointEvalG1_Mask0b111_Infinity:
	default:
		panic("unhandled case" + strconv.Itoa(int(c)))
	}
	return res
}

func (i *pointEvalInput) generate() (bool, error) {
	var err error
	// we first generate a valid input, then modify the values according to the input type
	randPoly := make(fr_bls12381.Vector, evmBlockSize)
	var evaluationPoint fr_bls12381.Element
	var kzgCommitment bls12381.G1Affine
	var kzgProof kzg_bls12381.OpeningProof
	for {
		switch i.inputType {
		case validPointEval:
			randPoly.MustSetRandom()
		case validPointEvalZero:
			// when poly is zero, then everything is zero
			if i.commitmentCase != validPointEvalG1_ElementInfinity {
				return false, nil
			}
			if i.proofCase != validPointEvalG1_ElementInfinity {
				return false, nil
			}
			for i := range randPoly {
				randPoly[i].SetZero()
			}
		case invalidPointEval:
			// we generate a random poly, we will modify the values later
			randPoly.MustSetRandom()
		}
		kzgCommitment, err = kzg_bls12381.Commit(randPoly, *srsPk)
		if err != nil {
			return false, fmt.Errorf("failed to commit random polynomial: %w", err)
		}
		// -- check that the commitment value is according to the test case, otherwise regenerate
		i.Commitment = kzgCommitment.Bytes()
		switch i.commitmentCase {
		case validPointEvalG1_ElementSmallY:
			if i.Commitment[0]&mMask != mCompressedSmallest {
				continue
			}
		case validPointEvalG1_ElementLargeY:
			if i.Commitment[0]&mMask != mCompressedLargest {
				continue
			}
		default:
			// we don't care about other cases, we will modify the bytes later
		}
		switch i.evaluationPointCase {
		case validPointEvalScalar:
			evaluationPoint.MustSetRandom()
			i.EvaluationPoint = evaluationPoint.BigInt(new(big.Int))
		case validPointEvalScalar_Zero:
			evaluationPoint.SetZero()
			i.EvaluationPoint = big.NewInt(0)
		case invalidPointEvalScalar_Overflow:
			i.EvaluationPoint = generateScalar(msmScalarBig)
			evaluationPoint.SetBigInt(i.EvaluationPoint)
		}
		// -- check that the proof value is according to the test case, otherwise regenerate
		kzgProof, err = kzg_bls12381.Open(randPoly, evaluationPoint, *srsPk)
		if err != nil {
			return false, fmt.Errorf("failed to open random polynomial: %w", err)
		}
		i.Proof = kzgProof.H.Bytes()
		switch i.proofCase {
		case validPointEvalG1_ElementSmallY:
			if i.Proof[0]&mMask != mCompressedSmallest {
				continue
			}
		case validPointEvalG1_ElementLargeY:
			if i.Proof[0]&mMask != mCompressedLargest {
				continue
			}
		default:
			// we don't care about other cases, we will modify the bytes later
		}
		break
	}
	// sanity check
	if err = kzg_bls12381.Verify(&kzgCommitment, &kzgProof, evaluationPoint, *srsVk); err != nil {
		return false, fmt.Errorf("failed to verify KZG proof: %w", err)
	}
	switch i.inputType {
	case invalidPointEval:
		i.ClaimedValue = generateScalar(msmScalarRange)
	default:
		i.ClaimedValue = kzgProof.ClaimedValue.BigInt(new(big.Int))
	}
	switch i.claimedValueCase {
	case validPointEvalScalar:
		// do nothing, it is already valid
	case validPointEvalScalar_Zero:
		// set the claimed value to zero. This invalidates the proof
		i.ClaimedValue = big.NewInt(0)
	case invalidPointEvalScalar_Overflow:
		// add modulus to the claimed value to ensure it always overflows
		i.ClaimedValue.Add(i.ClaimedValue, evmBlsModulus)
	}

	switch i.commitmentCase {
	case validPointEvalG1_ElementSmallY:
		// do nothing, it is already valid
	case validPointEvalG1_ElementLargeY:
		// do nothing, it is already valid
	case validPointEvalG1_ElementInfinity:
		// do nothing, it is already valid
	default:
		i.Commitment = generateInvalidCompressedPoint(i.commitmentCase)
	}
	switch i.proofCase {
	case validPointEvalG1_ElementSmallY:
		// do nothing, it is already valid
	case validPointEvalG1_ElementLargeY:
		// do nothing, it is already valid
	case validPointEvalG1_ElementInfinity:
		// do nothing, it is already valid
	default:
		i.Proof = generateInvalidCompressedPoint(i.proofCase)
	}

	i.VersionedHash = sha256.Sum256(i.Commitment[:])
	i.VersionedHash[0] = blobCommitmentVersionKZG
	var hashModifiers [2]byte
	for {
		_, err = rand.Read(hashModifiers[:])
		if err != nil {
			return false, fmt.Errorf("failed to read random bytes: %w", err)
		}
		if hashModifiers[1] != 0 {
			break
		}
	}
	switch i.versionedHashCase {
	case validPointEvalHash:
		// do nothing, it is already valid
	case invalidPointEvalHash_WrongVersion:
		i.VersionedHash[0] ^= hashModifiers[1]
	case invalidPointEvalHash_WrongHash:
		i.VersionedHash[(hashModifiers[0]%31)+1] ^= hashModifiers[1]
	}

	switch i.expectedBlobSizeCase {
	case validPointEvalExpectedBlobSize:
		i.ExpectedBlobSize = big.NewInt(evmBlockSize)
	case invalidPointEvalExpectedBlobSize_Zero:
		i.ExpectedBlobSize = big.NewInt(0)
	case invalidPointEvalExpectedBlobSize_TooSmall:
		i.ExpectedBlobSize, err = rand.Int(rand.Reader, big.NewInt(evmBlockSize))
		if err != nil {
			return false, fmt.Errorf("failed to generate random expected blob size: %w", err)
		}
	case invalidPointEvalExpectedBlobSize_TooLarge:
		bound := new(big.Int).Lsh(big.NewInt(1), 128)
		i.ExpectedBlobSize, err = rand.Int(rand.Reader, bound)
		if err != nil {
			return false, fmt.Errorf("failed to generate random expected blob size: %w", err)
		}
	case invalidPointEvalExpectedBlobSize_TooLargeTwoWords:
		bound := new(big.Int).Lsh(big.NewInt(1), 256)
		i.ExpectedBlobSize, err = rand.Int(rand.Reader, bound)
		if err != nil {
			return false, fmt.Errorf("failed to generate random expected blob size: %w", err)
		}
	}
	switch i.expectedBlsModulusCase {
	case validPointEvalExpectedModulus:
		i.ExpectedBlsModulus = new(big.Int).Set(evmBlsModulus)
	case invalidPointEvalExpectedModulus_Zero:
		i.ExpectedBlsModulus = big.NewInt(0)
	case invalidPointEvalExpectedModulus_TooSmall:
		i.ExpectedBlsModulus, err = rand.Int(rand.Reader, evmBlsModulus)
		if err != nil {
			return false, fmt.Errorf("failed to generate random expected modulus: %w", err)
		}
	case invalidPointEvalExpectedModulus_TooLarge:
		i.ExpectedBlsModulus, err = rand.Int(rand.Reader, evmBlsModulus)
		if err != nil {
			return false, fmt.Errorf("failed to generate random expected modulus: %w", err)
		}
		i.ExpectedBlsModulus.Add(i.ExpectedBlsModulus, evmBlsModulus)
	}

	// in case the claimed value or evaluation point overflow, we cannot send it to any circuit
	if i.evaluationPointCase == invalidPointEvalScalar_Overflow || i.claimedValueCase == invalidPointEvalScalar_Overflow {
		return true, nil
	}
	// we send the input to the success circuit only if the field are valid and we haven't invalidated the result
	if slices.Contains(validPointEvalHashes, i.versionedHashCase) &&
		slices.Contains(validPointEvalScalars, i.evaluationPointCase) &&
		(i.claimedValueCase == validPointEvalScalar ||
			(i.claimedValueCase == validPointEvalScalar_Zero &&
				i.inputType == validPointEvalZero)) &&
		slices.Contains(validPointEvalG1Elements, i.commitmentCase) &&
		slices.Contains(validPointEvalG1Elements, i.proofCase) &&
		slices.Contains(validPointEvalExpectedBlobSizes, i.expectedBlobSizeCase) &&
		slices.Contains(validPointEvalExpectedModuli, i.expectedBlsModulusCase) &&
		slices.Contains(validPointEvals, i.inputType) {
		i.ToPointEvalCircuit = true
		return true, nil
	}
	// otherwise we send it to the invalid circuit
	i.ToPointEvalInvalidCircuit = true
	return true, nil
}

func generatePointEvalInput() iter.Seq2[int, pointEvalInput] {
	return func(yield func(int, pointEvalInput) bool) {
		var id int
		for _, versionedHashCase := range allPointEvalHashes {
			for _, evaluationPointCase := range allPointEvalScalars {
				for _, claimedValueCase := range allPointEvalScalars {
					for _, commitmentCase := range allPointEvalG1Elements {
						for _, proofCase := range allPointEvalG1Elements {
							for _, expectedBlobSizeCase := range allPointEvalExpectedBlobSizes {
								for _, expectedBlsModulusCase := range allPointEvalExpectedModuli {
									for _, inputType := range allPointEvals {
										input := pointEvalInput{
											versionedHashCase:      versionedHashCase,
											evaluationPointCase:    evaluationPointCase,
											claimedValueCase:       claimedValueCase,
											commitmentCase:         commitmentCase,
											proofCase:              proofCase,
											expectedBlobSizeCase:   expectedBlobSizeCase,
											expectedBlsModulusCase: expectedBlsModulusCase,
											inputType:              inputType,
										}
										ok, err := input.generate()
										if err != nil {
											panic(fmt.Sprintf("failed to generate point evaluation input: %v", err))
										}
										if !ok {
											continue
										}

										if !yield(id, input) {
											return
										}
										id++
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func (i *pointEvalInput) WriteCSV(w *csv.Writer, id int) error {
	dataLimbs := slices.Concat(
		splitVersionedHashToLimbs(i.VersionedHash),
		splitScalarToLimbs(i.EvaluationPoint),
		splitScalarToLimbs(i.ClaimedValue),
		splitG1CompressedLimbs(i.Commitment),
		splitG1CompressedLimbs(i.Proof),
	)
	resultLimbs := slices.Concat(
		splitScalarToLimbs(i.ExpectedBlobSize),
		splitScalarToLimbs(i.ExpectedBlsModulus),
	)

	for index, v := range dataLimbs {
		if err := w.Write([]string{
			strconv.Itoa(id),
			"1",                                   // DATA
			"0",                                   // RSTL
			strconv.Itoa(index),                   // INDEX
			strconv.Itoa(index),                   // CT
			"1",                                   // IS_FIRST_INPUT
			"0",                                   // IS_SECOND_INPUT
			formatBoolAsInt(i.ToPointEvalCircuit), // CIRCUIT_SELECTOR_POINTEVAL
			formatBoolAsInt(i.ToPointEvalInvalidCircuit), // CIRCUIT_SELECTOR_POINTEVAL_FAILURE
			v, // LIMB
		}); err != nil {
			return fmt.Errorf("write versioned hash limb: %w", err)
		}
		index++
	}
	for index, v := range resultLimbs {
		if err := w.Write([]string{
			strconv.Itoa(id),
			"0",                                   // DATA
			"1",                                   // RSTL
			strconv.Itoa(index),                   // INDEX
			strconv.Itoa(index),                   // CT
			"0",                                   // IS_FIRST_INPUT
			"0",                                   // IS_SECOND_INPUT
			formatBoolAsInt(i.ToPointEvalCircuit), // CIRCUIT_SELECTOR_POINTEVAL
			formatBoolAsInt(i.ToPointEvalInvalidCircuit), // CIRCUIT_SELECTOR_POINTEVAL_FAILURE
			v, // LIMB
		}); err != nil {
			return fmt.Errorf("write expected blob size limb: %w", err)
		}
		index++
	}
	return nil
}

func headerPointEval() []string {
	return []string{
		"ID",
		"DATA_POINT_EVALUATION",
		"RSLT_POINT_EVALUATION",
		"INDEX",
		"CT",
		"IS_FIRST_INPUT",
		"IS_SECOND_INPUT",
		"CIRCUIT_SELECTOR_POINT_EVALUATION",
		"CIRCUIT_SELECTOR_POINT_EVALUATION_FAILURE",
		"LIMB",
	}
}

func mainPointEval() error {
	id := 0
	fileNr := 0
	testcases := generatePointEvalInput()
	var f *os.File
	var w *csv.Writer
	var err error
	for _, input := range testcases {
		if id%nbPointEvalPerOutput == 0 {
			if f != nil {
				w.Flush()
				f.Close()
			}
			f, err = os.Create(fmt.Sprintf(path_pointeval, fileNr))
			if err != nil {
				return fmt.Errorf("create file: %w", err)
			}
			w = csv.NewWriter(f)
			if err := w.Write(headerPointEval()); err != nil {
				return fmt.Errorf("write header: %w", err)
			}
			fileNr++
		}
		if err := input.WriteCSV(w, id); err != nil {
			return fmt.Errorf("write point evaluation input: %w", err)
		}
		id++
	}
	if f != nil {
		w.Flush()
		f.Close()
	}
	return nil
}
