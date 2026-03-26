package keccak

// Dimension of the keccak state matrix. The State is structured
// as an array of 5 x 5 uint64.
const Dim = 5           // size of the keccak state matrix
const NumRound int = 24 // number of rounds in the keccak permutation

// LR collects the triangular numbers for the \pi and \rho step
var LR = [Dim][Dim]int{
	{0, 36, 3, 41, 18},
	{1, 44, 10, 45, 2},
	{62, 6, 43, 15, 61},
	{28, 55, 25, 21, 56},
	{27, 20, 39, 8, 14},
}

// RC stores the rounds constants for the \iota steps
var RC = [NumRound]uint64{
	0x0000000000000001,
	0x0000000000008082,
	0x800000000000808A,
	0x8000000080008000,
	0x000000000000808B,
	0x0000000080000001,
	0x8000000080008081,
	0x8000000000008009,
	0x000000000000008A,
	0x0000000000000088,
	0x0000000080008009,
	0x000000008000000A,
	0x000000008000808B,
	0x800000000000008B,
	0x8000000000008089,
	0x8000000000008003,
	0x8000000000008002,
	0x8000000000000080,
	0x000000000000800A,
	0x800000008000000A,
	0x8000000080008081,
	0x8000000000008080,
	0x0000000080000001,
	0x8000000080008008,
}

// Permute applies the Keccakf permutation over the state and appends an entry
// to the PermTraces function.
func (a *State) Permute(traces *PermTraces) {

	// Save the input of the permutation
	if traces != nil {
		traces.KeccakFInps = append(traces.KeccakFInps, *a)
	}

	for round := 0; round < NumRound; round++ {
		a.ApplyKeccakfRound(round)
	}

	// Save the output of the permutation
	if traces != nil {
		traces.KeccakFOuts = append(traces.KeccakFOuts, *a)
	}
}

// ApplyKeccakfRound applies one round of the keccakf permutation
func (a *State) ApplyKeccakfRound(round int) {
	a.Theta()
	a.Rho()
	b := a.Pi()
	a.Chi(&b)
	a.Iota(round)
}

// Applies the theta step over the state
// See https://keccak.team/keccak_specs_summary.html
//
//	C[x] = A[x,0] xor A[x,1] xor A[x,2] xor A[x,3] xor A[x,4],   for x in 0…4
//	D[x] = C[x-1] xor rot(C[x+1],1),                             for x in 0…4
//	A[x,y] = A[x,y] xor D[x],                           for (x,y) in (0…4,0…4)
func (a *State) Theta() {
	var c, d [Dim]uint64

	// populate c first because populating d requires accessing to
	// c + 1 so we need two loops.
	for x := range c {
		c[x] = a[x][0] ^ a[x][1] ^ a[x][2] ^ a[x][3] ^ a[x][4]
	}

	for x := range d {
		// populate d
		d[x] = c[mod5(x-1)] ^ cycShf(c[mod5(x+1)], 1)
		// and xor it back into a
		for y := range d {
			a[x][y] ^= d[x]
		}
	}
}

// Applies the \rho and \pi steps
//
// B[y,2*x+3*y] = rot(A[x,y], r[x,y]),  for (x,y) in (0…4,0…4)
func (a *State) Rho() {
	for x := 0; x < Dim; x++ {
		for y := 0; y < Dim; y++ {
			a[x][y] = cycShf(a[x][y], LR[x][y])
		}
	}
}

// Applies the \pi steps
func (a *State) Pi() (b State) {
	for x := 0; x < Dim; x++ {
		for y := 0; y < Dim; y++ {
			b[y][mod5(2*x+3*y)] = a[x][y]
		}
	}
	return b
}

// Applies the Chi step
//
// A[x,y] = B[x,y] xor ((not B[x+1,y]) and B[x+2,y]),  for (x,y) in (0…4,0…4)
func (a *State) Chi(b *State) {
	for x := 0; x < Dim; x++ {
		for y := 0; y < Dim; y++ {
			a[x][y] = b[x][y] ^ (^b[mod5(x+1)][y] & b[mod5(x+2)][y])
		}
	}
}

// Applies the iota step to the keccak state
// A[0,0] <- A[0,0] xor RC
func (a *State) Iota(round int) {
	a[0][0] ^= RC[round]
}
