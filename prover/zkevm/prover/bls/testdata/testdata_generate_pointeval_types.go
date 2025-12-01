package main

type pointEvalInputType int
type pointEvalInputHashType int
type pointEvalInputScalarType int
type pointEvalInputG1ElementType int
type pointEvalInputExpectedBlobSizeType int
type pointEvalInputExpectedModulusType int

const (
	validPointEval     pointEvalInputType = iota // claimed value is correct
	validPointEvalZero                           // the polynomial is all zero
	invalidPointEval                             // in case that all inputs are valid but claimed value is incorrect
)

var validPointEvals = []pointEvalInputType{
	validPointEval,
	validPointEvalZero,
}
var invalidPointEvals = []pointEvalInputType{
	invalidPointEval,
}
var allPointEvals = append(validPointEvals, invalidPointEvals...)

const (
	validPointEvalHash                pointEvalInputHashType = iota // valid point evaluation hash
	invalidPointEvalHash_WrongVersion                               // valid point evaluation hash but wrong version
	invalidPointEvalHash_WrongHash                                  // wrong hash for the hash of claimed value
)

var validPointEvalHashes = []pointEvalInputHashType{
	validPointEvalHash,
}
var invalidPointEvalHashes = []pointEvalInputHashType{
	invalidPointEvalHash_WrongVersion,
	invalidPointEvalHash_WrongHash,
}
var allPointEvalHashes = append(validPointEvalHashes, invalidPointEvalHashes...)

const (
	validPointEvalScalar            pointEvalInputScalarType = iota // valid point evaluation scalar
	validPointEvalScalar_Zero                                       // valid point evaluation scalar but zero
	invalidPointEvalScalar_Overflow                                 // invalid point evaluation scalar, overflows. Checked by the arithmetization.
)

var validPointEvalScalars = []pointEvalInputScalarType{
	validPointEvalScalar,
	validPointEvalScalar_Zero,
}
var invalidPointEvalScalars = []pointEvalInputScalarType{invalidPointEvalScalar_Overflow}
var allPointEvalScalars = append(validPointEvalScalars, invalidPointEvalScalars...)

const (
	validPointEvalG1_ElementSmallY              pointEvalInputG1ElementType = iota // valid compressed point with small y
	validPointEvalG1_ElementLargeY                                                 // valid compressed point with large y
	validPointEvalG1_ElementInfinity                                               // valid compressed point at infinity
	invalidPointEvalG1_MaskYLarge_YSmall                                           // mask for large y, but encoded small y
	invalidPointEvalG1_MaskYSmall_YLarge                                           // mask for small y, but encoded large y
	invalidPointEvalG1_MaskInfinity_NotInfinity                                    // mask for infinity, but encoded not infinity
	invalidPointEvalG1_MaskSmallY_Infinity                                         // mask for small y, but encoded infinity
	invalidPointEvalG1_MaskLargeY_Infinity                                         // mask for large y, but encoded infinity
	invalidPointEvalG1_MaskValid_NotInGroup                                        // mask for valid point, but not in the group
	invalidPointEvalG1_MaskValid_NotOnCurve                                        // mask for valid point, but not on the curve
	invalidPointEvalG1_Mask0b000_Random                                            // mask for invalid mask 0b000, encoded random point
	invalidPointEvalG1_Mask0b010_Random                                            // mask for invalid 0b010, encoded random point
	invalidPointEvalG1_Mask0b010_Infinity                                          // mask for invalid 0b010, encoded infinity point
	invalidPointEvalG1_Mask0b001_Random                                            // mask for invalid 0b001, encoded random point
	invalidPointEvalG1_Mask0b001_Infinity                                          // mask for invalid 0b001, encoded infinity point
	invalidPointEvalG1_Mask0b011_Random                                            // mask for invalid 0b011, encoded random point
	invalidPointEvalG1_Mask0b011_Infinity                                          // mask for invalid 0b011, encoded infinity point
	invalidPointEvalG1_Mask0b111_Random                                            // mask for invalid 0b111, encoded random point
	invalidPointEvalG1_Mask0b111_Infinity                                          // mask for invalid 0b111, encoded infinity point
)

var validPointEvalG1Elements = []pointEvalInputG1ElementType{
	validPointEvalG1_ElementSmallY,
	validPointEvalG1_ElementLargeY,
	validPointEvalG1_ElementInfinity,
}
var invalidPointEvalG1Elements = []pointEvalInputG1ElementType{
	invalidPointEvalG1_MaskYLarge_YSmall,
	invalidPointEvalG1_MaskYSmall_YLarge,
	invalidPointEvalG1_MaskInfinity_NotInfinity,
	invalidPointEvalG1_MaskSmallY_Infinity,
	invalidPointEvalG1_MaskLargeY_Infinity,
	invalidPointEvalG1_MaskValid_NotInGroup,
	invalidPointEvalG1_MaskValid_NotOnCurve,
	invalidPointEvalG1_Mask0b000_Random,
	invalidPointEvalG1_Mask0b010_Random,
	// invalidPointEvalG1_Mask0b010_Infinity,
	invalidPointEvalG1_Mask0b001_Random,
	// invalidPointEvalG1_Mask0b001_Infinity,
	invalidPointEvalG1_Mask0b011_Random,
	// invalidPointEvalG1_Mask0b011_Infinity,
	invalidPointEvalG1_Mask0b111_Random,
	// invalidPointEvalG1_Mask0b111_Infinity,
}
var allPointEvalG1Elements = append(validPointEvalG1Elements, invalidPointEvalG1Elements...)

const (
	validPointEvalExpectedBlobSize                    pointEvalInputExpectedBlobSizeType = iota // valid expected blob size
	invalidPointEvalExpectedBlobSize_Zero                                                       // invalid expected blob size, zero. Checked by arithmetization
	invalidPointEvalExpectedBlobSize_TooSmall                                                   // invalid expected blob size, too small. Checked by arithmetization
	invalidPointEvalExpectedBlobSize_TooLarge                                                   // invalid expected blob size, too large. Checked by arithmetization
	invalidPointEvalExpectedBlobSize_TooLargeTwoWords                                           // invalid expected blob size, too larg value encoded on two words. Checked by arithmetization
)

var validPointEvalExpectedBlobSizes = []pointEvalInputExpectedBlobSizeType{
	validPointEvalExpectedBlobSize,
}
var invalidPointEvalExpectedBlobSizes = []pointEvalInputExpectedBlobSizeType{
	// invalidPointEvalExpectedBlobSize_Zero,
	// invalidPointEvalExpectedBlobSize_TooSmall,
	// invalidPointEvalExpectedBlobSize_TooLarge,
	// invalidPointEvalExpectedBlobSize_TooLargeTwoWords,
}
var allPointEvalExpectedBlobSizes = append(validPointEvalExpectedBlobSizes, invalidPointEvalExpectedBlobSizes...)

const (
	validPointEvalExpectedModulus            pointEvalInputExpectedModulusType = iota // valid expected modulus
	invalidPointEvalExpectedModulus_Zero                                              // invalid expected modulus, zero. Checked by arithmetization
	invalidPointEvalExpectedModulus_TooSmall                                          // invalid expected modulus, too small. Checked by arithmetization
	invalidPointEvalExpectedModulus_TooLarge                                          // invalid expected modulus, too large. Checked by arithmetization
)

var validPointEvalExpectedModuli = []pointEvalInputExpectedModulusType{
	validPointEvalExpectedModulus,
}
var invalidPointEvalExpectedModuli = []pointEvalInputExpectedModulusType{
	// invalidPointEvalExpectedModulus_Zero,
	// invalidPointEvalExpectedModulus_TooSmall,
	// invalidPointEvalExpectedModulus_TooLarge,
}
var allPointEvalExpectedModuli = append(validPointEvalExpectedModuli, invalidPointEvalExpectedModuli...)
