package ringsis

import (
	"runtime"
	"sync"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/sis"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
	"github.com/consensys/linea-monorepo/prover/utils/parallel/pool"
)

// TransversalHash evaluates SIS hashes transversally over a list of smart-vectors.
// Each smart-vector is seen as the row of a matrix. All rows must have the same
// size or panic. The function returns the hash of the columns. The column hashes
// are concatenated into a single array.
func (s *Key) TransversalHash(v []smartvectors.SmartVector) []field.Element {

	// nbRows stores the number of rows in the matrix to hash it must be
	// strictly positive and be within the bounds of MaxNumFieldHashable.
	nbRows := len(v)

	if nbRows == 0 || nbRows > s.MaxNumFieldHashable() {
		utils.Panic("Attempted to hash %v rows, must be in [1:%v]", nbRows, s.MaxNumFieldHashable())
	}

	// nbCols stores the number of columns in the matrix to hash et must be
	// positive and all the rows must have the same size.
	nbCols := v[0].Len()

	if nbCols == 0 {
		utils.Panic("Provided a 0-column matrix")
	}

	for i := range v {
		if v[i].Len() != nbCols {
			utils.Panic("Unexpected : all inputs smart-vectors should have the same length the first one has length %v, but #%v has length %v",
				nbCols, i, v[i].Len())
		}
	}

	/*
		v contains a list of rows. We want to hash the columns, in a cache-friendly
		manner.


		for example, if we consider the matrix
		v[0] -> [ 1  2  3  4  ]
		v[1] -> [ 5  6  7  8  ]
		v[2] -> [ 9  10 11 12 ]
		v[3] -> [ 13 14 15 16 ]

		we want to compute
		res = [ H(1,5,9,13) H(2,6,10,14) H(3,7,11,15) H(4,8,12,16) ]

		note that the output size of the hash is s.OutputSize() (i.e it's a slice)
		and that we will decompose the columns in "Limbs" of size s.LogTwoBound;
		this limbs are then interpreted as a slice of coefficients of
		a polynomial of size s.OutputSize()

		that is, we can decompose H(1,5,9,13) as;
		k0 := limbs(1,5) 	= [a b c d e f g h]
		k1 := limbs(9,13) 	= [i j k l m n o p]

		In practice, s.OutputSize() is a reasonable size (< 1024) so we can slide our tiles
		over the partial columns and compute the hash of the columns in parallel.

	*/

	nbBytePerLimb := s.LogTwoBound / 8
	nbLimbsPerField := field.Bytes / nbBytePerLimb
	nbFieldPerPoly := s.modulusDegree() / nbLimbsPerField

	N := s.OutputSize()

	nbPolys := utils.DivCeil(len(v), nbFieldPerPoly)
	res := make(field.Vector, nbCols*N)

	// First we take care of the constant rows;
	// since they repeat the same value, we can compute them once for the matrix (instead of once per column)
	// and accumulate in res

	// indicates if a block of N rows is constant: in that case we can skip the computation
	// of all the columns sub-hashes in that block.
	// more over; we set the bit of a mask if the row is NOT constant, and exploit the mask
	// to minimize the number of operations we do (partial FFT)
	masks := make([]uint64, nbPolys)

	// we will accumulate the constant rows in a separate vector
	constPoly := make(field.Vector, N)
	constLock := sync.Mutex{}

	// we parallelize by the "height" of the matrix here, since we only care about the constants
	// and don't iterate over the columns.
	parallel.Execute(nbPolys, func(start, stop int) {
		startRow := start * nbFieldPerPoly
		stopRow := stop * nbFieldPerPoly
		if stopRow > len(v) {
			stopRow = len(v)
		}
		localRes := make([]field.Element, N)

		itM := s.newMatrixIterator(v)

		k := make([]field.Element, N)
		kz := make([]field.Element, N)

		for polID := start; polID < stop; polID++ {
			mConst := uint64(0)
			for row := startRow; row < stopRow; row++ {
				if _, ok := v[row].(*smartvectors.Constant); !ok {
					// mark the row as non-constant in the mask for this poly
					masks[polID] |= 1 << (row % nbFieldPerPoly)
				} else {
					// mark the row as constant
					mConst |= 1 << (row % nbFieldPerPoly)
				}
			}

			itM.reset(startRow, stopRow, 0, true)
			s.gnarkInternal.InnerHash(itM.lit, localRes, k, kz, polID, mConst)
		}

		constLock.Lock()
		constPoly.Add(constPoly, localRes)
		constLock.Unlock()
	})

	nbCpus := runtime.NumCPU()

	nbColPerTile := 16
	nbJobs := utils.DivCeil(nbCols, nbColPerTile)

	if nbCols < nbCpus {
		nbJobs = nbCols
		nbColPerTile = 1
	}

	for nbJobs < nbCpus && nbColPerTile > 1 {
		nbColPerTile--
		nbJobs = utils.DivCeil(nbCols, nbColPerTile)
	}

	pool.ExecutePoolChunky(nbJobs, func(jobID int) {
		startCol := jobID * nbColPerTile
		stopCol := startCol + nbColPerTile
		stopCol = min(stopCol, nbCols)

		// each go routine will iterate over a range of columns; we will hash the columns in parallel
		// and accumulate the result in res (no conflict since each go routine writes to a different range of res)

		// init res with const poly
		for colID := startCol; colID < stopCol; colID++ {
			copy(res[colID*N:(colID+1)*N], constPoly)
		}

		itM := s.newMatrixIterator(v)
		k := make([]field.Element, N)
		kz := make([]field.Element, N)

		for startRow := 0; startRow < len(v); startRow += nbFieldPerPoly {
			polID := startRow / nbFieldPerPoly

			// if it's a constant block, we can skip.
			if masks[polID] == 0 {
				continue
			}

			stopRow := startRow + nbFieldPerPoly
			stopRow = min(stopRow, len(v))

			// hash the subcolumns.
			for colID := startCol; colID < stopCol; colID++ {
				itM.reset(startRow, stopRow, colID, false)
				s.gnarkInternal.InnerHash(itM.lit, res[colID*N:colID*N+N], k, kz, polID, masks[polID])
			}

		}

		// mod X^n - 1
		for colID := startCol; colID < stopCol; colID++ {
			s.gnarkInternal.Domain.FFTInverse(res[colID*N:(colID+1)*N], fft.DIT, fft.OnCoset(), fft.WithNbTasks(1))
		}

	})

	return res
}

// matrixIterator helps allocate resources per go routine
// and iterate over the columns of a matrix (defined by a list of rows: smart-vectors)
type matrixIterator struct {
	it  columnIterator
	lit *sis.LimbIterator
}

func (s *Key) newMatrixIterator(v []smartvectors.SmartVector) matrixIterator {
	w := matrixIterator{
		it: columnIterator{
			v: v,
		},
	}
	w.lit = sis.NewLimbIterator(&w.it, s.LogTwoBound/8)
	return w
}

func (w *matrixIterator) reset(startRow, stopRow, colIndex int, constIT bool) {
	w.it.startRow = startRow
	w.it.endRow = stopRow
	w.it.colIndex = colIndex
	w.it.isConstIT = constIT
	w.lit.Reset(&w.it)
}

// columnIterator is a helper struct to iterate over the columns of a matrix
// it implements the sis.ElementIterator interface
type columnIterator struct {
	v                []smartvectors.SmartVector
	startRow, endRow int
	colIndex         int
	isConstIT        bool
}

func (it *columnIterator) Next() (field.Element, bool) {
	if it.endRow == it.startRow {
		return field.Element{}, false
	}
	row := it.v[it.startRow]
	_, constRow := row.(*smartvectors.Constant)
	it.startRow++

	// for a const iterator; we only return constant rows.
	// for a non-const iterator; we filter out constant rows.
	if (it.isConstIT && constRow) || (!it.isConstIT && !constRow) {
		return row.Get(it.colIndex), true
	}
	return field.Element{}, true
}
