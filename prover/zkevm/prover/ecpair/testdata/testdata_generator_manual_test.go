package testdata

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fp"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"github.com/consensys/gnark/test"
)

func TestPairingCheckData(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	assert := test.NewAssert(t)
	_, _, p, q := bn254.Generators()

	var u, v fr.Element
	u.SetInt64(1234)
	v.SetInt64(5678)
	// u.SetRandom()
	// v.SetRandom()

	p.ScalarMultiplication(&p, u.BigInt(new(big.Int)))
	q.ScalarMultiplication(&q, v.BigInt(new(big.Int)))

	var p1, p2 bn254.G1Affine
	var q1, q2 bn254.G2Affine
	p1.Double(&p)
	p2.Neg(&p)
	q1.Set(&q)
	q2.Double(&q)

	ok, err := bn254.PairingCheck([]bn254.G1Affine{p1, p2}, []bn254.G2Affine{q1, q2})
	assert.NoError(err)
	assert.True(ok)

	px := p1.X.Bytes()
	py := p1.Y.Bytes()
	fmt.Printf("0x%x\n0x%x\n0x%x\n0x%x\n", px[0:16], px[16:32], py[0:16], py[16:32])
	qxre := q1.X.A0.Bytes()
	qxim := q1.X.A1.Bytes()
	qyre := q1.Y.A0.Bytes()
	qyim := q1.Y.A1.Bytes()
	fmt.Printf("0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n",
		qxre[0:16], qxre[16:32], qxim[0:16], qxim[16:32], qyre[0:16], qyre[16:32], qyim[0:16], qyim[16:32])

	p2x := p2.X.Bytes()
	p2y := p2.Y.Bytes()
	fmt.Printf("0x%x\n0x%x\n0x%x\n0x%x\n", p2x[0:16], p2x[16:32], p2y[0:16], p2y[16:32])
	q2xre := q2.X.A0.Bytes()
	q2xim := q2.X.A1.Bytes()
	q2yre := q2.Y.A0.Bytes()
	q2yim := q2.Y.A1.Bytes()
	fmt.Printf("0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n",
		q2xre[0:16], q2xre[16:32], q2xim[0:16], q2xim[16:32], q2yre[0:16], q2yre[16:32], q2yim[0:16], q2yim[16:32])
}

func TestPairingFailingCheckData(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	assert := test.NewAssert(t)
	var p2 bn254.G1Affine
	var q2 bn254.G2Affine
	_, _, p, q := bn254.Generators()

	var u, v fr.Element
	u.SetInt64(6235)
	v.SetInt64(76235)
	// u.SetRandom()
	// v.SetRandom()

	p.ScalarMultiplication(&p, big.NewInt(12390))
	q.ScalarMultiplication(&q, big.NewInt(12489))
	p2.ScalarMultiplicationBase(big.NewInt(79975))
	q2.ScalarMultiplicationBase(big.NewInt(48916))

	ok, err := bn254.PairingCheck([]bn254.G1Affine{p, p2}, []bn254.G2Affine{q, q2})
	assert.NoError(err)
	assert.False(ok)

	px := p.X.Bytes()
	py := p.Y.Bytes()
	fmt.Printf("0x%x\n0x%x\n0x%x\n0x%x\n", px[0:16], px[16:32], py[0:16], py[16:32])
	qxre := q.X.A0.Bytes()
	qxim := q.X.A1.Bytes()
	qyre := q.Y.A0.Bytes()
	qyim := q.Y.A1.Bytes()
	fmt.Printf("0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n",
		qxre[0:16], qxre[16:32], qxim[0:16], qxim[16:32], qyre[0:16], qyre[16:32], qyim[0:16], qyim[16:32])

	p2x := p2.X.Bytes()
	p2y := p2.Y.Bytes()
	fmt.Printf("0x%x\n0x%x\n0x%x\n0x%x\n", p2x[0:16], p2x[16:32], p2y[0:16], p2y[16:32])
	q2xre := q2.X.A0.Bytes()
	q2xim := q2.X.A1.Bytes()
	q2yre := q2.Y.A0.Bytes()
	q2yim := q2.Y.A1.Bytes()
	fmt.Printf("0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n",
		q2xre[0:16], q2xre[16:32], q2xim[0:16], q2xim[16:32], q2yre[0:16], q2yre[16:32], q2yim[0:16], q2yim[16:32])
}

func TestAddData(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	for i := 0; i < 4; i++ {
		var p, q bn254.G1Affine
		s1, err := rand.Int(rand.Reader, fr.Modulus())
		if err != nil {
			t.Fatal(err)
		}
		s2, err := rand.Int(rand.Reader, fr.Modulus())
		if err != nil {
			t.Fatal(err)
		}
		// s1 = big.NewInt(0)
		// s2 = big.NewInt(0)
		p.ScalarMultiplicationBase(s1)
		q.ScalarMultiplicationBase(s2)
		var r bn254.G1Affine
		r.Add(&p, &q)

		pxb := p.X.Bytes()
		pyb := p.Y.Bytes()
		qxb := q.X.Bytes()
		qyb := q.Y.Bytes()
		rxb := r.X.Bytes()
		ryb := r.Y.Bytes()
		fmt.Println(i)
		fmt.Printf("PXHI=0x%x\nPXLO=0x%x\nPYHI=0x%x\nPYLO=0x%x\nQXHI=0x%x\nQXLO=0x%x\nQYHI=0x%x\nQYLO=0x%x\nQXHI=0x%x\nQXLO=0x%x\nRYHI=0x%x\nRYLO=0x%x\n",
			pxb[0:16], pxb[16:32],
			pyb[0:16], pyb[16:32],
			qxb[0:16], qxb[16:32],
			qyb[0:16], qyb[16:32],
			rxb[0:16], rxb[16:32],
			ryb[0:16], ryb[16:32],
		)
	}
}

// small points
// 		malformed
// 			large coordinates
// 			coordinates in range but not satisfying the curve equation
// 		well-formed
// 			just a few points of the C1 curve drawn at random
// large points
// 	malformed
// 		large coordinates
// 		small coordinates but not satisfying the curve equation
// 		small coordinates, satisfying the curve equation but not belonging to the G2 subgroup
// 	well-formed
// 		just a few points on G2 drawn at random

func TestDataSmallMalformedLarge(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	fmt.Println("fp = ", fp.Modulus())
	_, _, p, _ := bn254.Generators()
	// var px [32]byte
	// p.X.BigInt(new(big.Int)).FillBytes(px[:])
	// pb := p.X.Bytes()
	// if !bytes.Equal(pb[:], px[:]) {
	// 	t.Fatal("failed to marshal point")
	// }

	// bound 2^256-modulus
	bound := new(big.Int).Lsh(big.NewInt(1), 256)
	bound.Sub(bound, fp.Modulus())

	var buf [32]byte

	// test 1, X large and Y in range
	x, _ := rand.Int(rand.Reader, bound)
	x.Add(x, fp.Modulus())
	y, _ := rand.Int(rand.Reader, fp.Modulus())
	fmt.Println("= X large and Y in range (not necessarily satisfying curve equation) =")
	x.FillBytes(buf[:])
	fmt.Printf("X = 0x%x\n", buf[:])
	y.FillBytes(buf[:])
	fmt.Printf("Y = 0x%x\n", buf[:])

	// test 2, X large and Y in range
	x, _ = rand.Int(rand.Reader, fp.Modulus())
	y, _ = rand.Int(rand.Reader, bound)
	y.Add(y, fp.Modulus())
	fmt.Println("= X in range and Y large (not necessarily satisfying curve equation) =")
	x.FillBytes(buf[:])
	fmt.Printf("X = 0x%x\n", buf[:])
	y.FillBytes(buf[:])
	fmt.Printf("Y = 0x%x\n", buf[:])

	// test 3, X large and Y large
	x, _ = rand.Int(rand.Reader, bound)
	x.Add(x, fp.Modulus())
	y, _ = rand.Int(rand.Reader, bound)
	y.Add(y, fp.Modulus())
	fmt.Println("= X large and Y large (not necessarily satisfying curve equation) =")
	x.FillBytes(buf[:])
	fmt.Printf("X = 0x%x\n", buf[:])
	y.FillBytes(buf[:])
	fmt.Printf("Y = 0x%x\n", buf[:])

	// test 4 - test 1 but satisfying curve equation
	s, _ := rand.Int(rand.Reader, fr.Modulus())
	p.ScalarMultiplicationBase(s)
	px := p.X.BigInt(new(big.Int))
	px.Add(px, fp.Modulus())
	fmt.Println("= X large and Y in range (satisfying curve equation) =")
	px.FillBytes(buf[:])
	fmt.Printf("X = 0x%x\n", buf[:])
	p.Y.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("Y = 0x%x\n", buf[:])

	// test 5 - test 2 but satisfying curve equation
	s, _ = rand.Int(rand.Reader, fr.Modulus())
	p.ScalarMultiplicationBase(s)
	py := p.X.BigInt(new(big.Int))
	py.Add(py, fp.Modulus())
	fmt.Println("= X in range and Y large (satisfying curve equation) =")
	p.X.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("X = 0x%x\n", buf[:])
	py.FillBytes(buf[:])
	fmt.Printf("Y = 0x%x\n", buf[:])

	// test 6 - test 3 but satisfying curve equation
	s, _ = rand.Int(rand.Reader, fr.Modulus())
	p.ScalarMultiplicationBase(s)
	px = p.X.BigInt(new(big.Int))
	px.Add(px, fp.Modulus())
	py = p.Y.BigInt(new(big.Int))
	py.Add(py, fp.Modulus())
	fmt.Println("= X large and Y large (satisfying curve equation) =")
	px.FillBytes(buf[:])
	fmt.Printf("X = 0x%x\n", buf[:])
	py.FillBytes(buf[:])
	fmt.Printf("Y = 0x%x\n", buf[:])
}

func TestDataSmallMalformedNotCurve(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	px, _ := rand.Int(rand.Reader, fp.Modulus())
	py, _ := rand.Int(rand.Reader, fp.Modulus())
	var p bn254.G1Affine
	p.X.SetBigInt(px)
	p.Y.SetBigInt(py)
	if p.IsOnCurve() {
		t.Fatal("point is on curve")
	}
	fmt.Println("= X and Y in range but not satisfying curve equation =")
	var buf [32]byte
	px.FillBytes(buf[:])
	fmt.Printf("X = 0x%x\n", buf[:])
	py.FillBytes(buf[:])
	fmt.Printf("Y = 0x%x\n", buf[:])
}

func TestDataSmallWell(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	for i := 0; i < 5; i++ {
		var p bn254.G1Affine
		s, _ := rand.Int(rand.Reader, fr.Modulus())
		p.ScalarMultiplicationBase(s)
		fmt.Println("= random point in G1 =")
		var buf [32]byte
		p.X.BigInt(new(big.Int)).FillBytes(buf[:])
		fmt.Printf("X = 0x%x\n", buf[:])
		p.Y.BigInt(new(big.Int)).FillBytes(buf[:])
		fmt.Printf("Y = 0x%x\n", buf[:])
	}
}

func TestDataLargeMalformedLarge(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	// bound 2^256-modulus
	bound := new(big.Int).Lsh(big.NewInt(1), 256)
	bound.Sub(bound, fp.Modulus())

	var buf [32]byte

	// test 1, XA large, XB in range, YA in range, YB in range
	xa, _ := rand.Int(rand.Reader, bound)
	xa.Add(xa, fp.Modulus())
	xb, _ := rand.Int(rand.Reader, fp.Modulus())
	ya, _ := rand.Int(rand.Reader, fp.Modulus())
	yb, _ := rand.Int(rand.Reader, fp.Modulus())
	var q bn254.G2Affine
	q.X.A0.SetBigInt(xa)
	q.X.A1.SetBigInt(xb)
	q.Y.A0.SetBigInt(ya)
	q.Y.A1.SetBigInt(yb)
	if q.IsOnCurve() {
		t.Fatal("point is on curve")
	}
	fmt.Println("= XA large, XB in range, YA in range, YB in range =")
	xa.FillBytes(buf[:])
	fmt.Printf("XA = 0x%x\n", buf[:])
	xb.FillBytes(buf[:])
	fmt.Printf("XB = 0x%x\n", buf[:])
	ya.FillBytes(buf[:])
	fmt.Printf("YA = 0x%x\n", buf[:])
	yb.FillBytes(buf[:])
	fmt.Printf("YB = 0x%x\n", buf[:])

	// test 2, XA in range, XB large, YA in range, YB in range
	xa, _ = rand.Int(rand.Reader, fp.Modulus())
	xb, _ = rand.Int(rand.Reader, bound)
	xb.Add(xb, fp.Modulus())
	ya, _ = rand.Int(rand.Reader, fp.Modulus())
	yb, _ = rand.Int(rand.Reader, fp.Modulus())
	q.X.A0.SetBigInt(xa)
	q.X.A1.SetBigInt(xb)
	q.Y.A0.SetBigInt(ya)
	q.Y.A1.SetBigInt(yb)
	if q.IsOnCurve() {
		t.Fatal("point is on curve")
	}
	fmt.Println("= XA in range, XB large, YA in range, YB in range =")
	xa.FillBytes(buf[:])
	fmt.Printf("XA = 0x%x\n", buf[:])
	xb.FillBytes(buf[:])
	fmt.Printf("XB = 0x%x\n", buf[:])
	ya.FillBytes(buf[:])
	fmt.Printf("YA = 0x%x\n", buf[:])
	yb.FillBytes(buf[:])
	fmt.Printf("YB = 0x%x\n", buf[:])

	// test 3, XA in range, XB in range, YA large, YB in range
	xa, _ = rand.Int(rand.Reader, fp.Modulus())
	xb, _ = rand.Int(rand.Reader, fp.Modulus())
	ya, _ = rand.Int(rand.Reader, bound)
	ya.Add(ya, fp.Modulus())
	yb, _ = rand.Int(rand.Reader, fp.Modulus())
	q.X.A0.SetBigInt(xa)
	q.X.A1.SetBigInt(xb)
	q.Y.A0.SetBigInt(ya)
	q.Y.A1.SetBigInt(yb)
	if q.IsOnCurve() {
		t.Fatal("point is on curve")
	}
	fmt.Println("= XA in range, XB in range, YA large, YB in range =")
	xa.FillBytes(buf[:])
	fmt.Printf("XA = 0x%x\n", buf[:])
	xb.FillBytes(buf[:])
	fmt.Printf("XB = 0x%x\n", buf[:])
	ya.FillBytes(buf[:])
	fmt.Printf("YA = 0x%x\n", buf[:])
	yb.FillBytes(buf[:])
	fmt.Printf("YB = 0x%x\n", buf[:])

	// test 4, XA in range, XB in range, YA in range, YB large
	xa, _ = rand.Int(rand.Reader, fp.Modulus())
	xb, _ = rand.Int(rand.Reader, fp.Modulus())
	ya, _ = rand.Int(rand.Reader, fp.Modulus())
	yb, _ = rand.Int(rand.Reader, bound)
	yb.Add(yb, fp.Modulus())
	q.X.A0.SetBigInt(xa)
	q.X.A1.SetBigInt(xb)
	q.Y.A0.SetBigInt(ya)
	q.Y.A1.SetBigInt(yb)
	if q.IsOnCurve() {
		t.Fatal("point is on curve")
	}
	fmt.Println("= XA in range, XB in range, YA in range, YB large =")
	xa.FillBytes(buf[:])
	fmt.Printf("XA = 0x%x\n", buf[:])
	xb.FillBytes(buf[:])
	fmt.Printf("XB = 0x%x\n", buf[:])
	ya.FillBytes(buf[:])
	fmt.Printf("YA = 0x%x\n", buf[:])
	yb.FillBytes(buf[:])
	fmt.Printf("YB = 0x%x\n", buf[:])

	// test 5, point in subgroup, but overflowing
	s, _ := rand.Int(rand.Reader, fr.Modulus())
	q.ScalarMultiplicationBase(s)
	xa = q.X.A0.BigInt(new(big.Int))
	xa.Add(xa, fp.Modulus())
	xb = q.X.A1.BigInt(new(big.Int))
	xb.Add(xb, fp.Modulus())
	ya = q.Y.A0.BigInt(new(big.Int))
	ya.Add(ya, fp.Modulus())
	yb = q.Y.A1.BigInt(new(big.Int))
	yb.Add(yb, fp.Modulus())
	q.X.A0.SetBigInt(xa)
	q.X.A1.SetBigInt(xb)
	q.Y.A0.SetBigInt(ya)
	q.Y.A1.SetBigInt(yb)
	if !q.IsOnCurve() {
		t.Fatal("point is on curve")
	}
	fmt.Println("= point in subgroup, but overflowing =")
	xa.FillBytes(buf[:])
	fmt.Printf("XA = 0x%x\n", buf[:])
	xb.FillBytes(buf[:])
	fmt.Printf("XB = 0x%x\n", buf[:])
	ya.FillBytes(buf[:])
	fmt.Printf("YA = 0x%x\n", buf[:])
	yb.FillBytes(buf[:])
	fmt.Printf("YB = 0x%x\n", buf[:])
}

func TestDataLargeMalformedNotCurve(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	var q bn254.G2Affine
	var buf [32]byte
	xa, _ := rand.Int(rand.Reader, fp.Modulus())
	xb, _ := rand.Int(rand.Reader, fp.Modulus())
	ya, _ := rand.Int(rand.Reader, fp.Modulus())
	yb, _ := rand.Int(rand.Reader, fp.Modulus())
	yb.Add(yb, fp.Modulus())
	q.X.A0.SetBigInt(xa)
	q.X.A1.SetBigInt(xb)
	q.Y.A0.SetBigInt(ya)
	q.Y.A1.SetBigInt(yb)
	if q.IsOnCurve() {
		t.Fatal("point is on curve")
	}
	fmt.Println("= point not on curve =")
	xa.FillBytes(buf[:])
	fmt.Printf("XA = 0x%x\n", buf[:])
	xb.FillBytes(buf[:])
	fmt.Printf("XB = 0x%x\n", buf[:])
	ya.FillBytes(buf[:])
	fmt.Printf("YA = 0x%x\n", buf[:])
	yb.FillBytes(buf[:])
	fmt.Printf("YB = 0x%x\n", buf[:])
}

func TestDataLargeMalformedNotSubgroup(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	var x, right, left, tmp, z, ZZ bn254.E2
	for {
		xa, _ := rand.Int(rand.Reader, fp.Modulus())
		xb, _ := rand.Int(rand.Reader, fp.Modulus())
		x.A0.SetBigInt(xa)
		x.A1.SetBigInt(xb)
		za, _ := rand.Int(rand.Reader, fp.Modulus())
		zb, _ := rand.Int(rand.Reader, fp.Modulus())
		z.A0.SetBigInt(za)
		z.A1.SetBigInt(zb)
		right.Square(&x).Mul(&right, &x)
		ZZ.Square(&z)
		tmp.Square(&ZZ).Mul(&tmp, &ZZ)
		tmp.MulBybTwistCurveCoeff(&tmp)
		right.Add(&right, &tmp)
		if right.Legendre() == 1 {
			break
		}
	}
	left.Sqrt(&right)
	QJac := bn254.G2Jac{
		X: x,
		Y: left,
		Z: z,
	}
	if !QJac.IsOnCurve() {
		t.Fatal("point is not on curve Jac")
	}
	var q bn254.G2Affine
	q.FromJacobian(&QJac)
	if !q.IsOnCurve() {
		t.Fatal("point is not on curve")
	}
	if q.IsInSubGroup() {
		t.Fatal("point is in subgroup")
	}

	var buf [32]byte
	fmt.Println("= point not in subgroup but on curve =")
	q.X.A0.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("XA = 0x%x\n", buf[:])
	q.X.A1.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("XB = 0x%x\n", buf[:])
	q.Y.A0.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("YA = 0x%x\n", buf[:])
	q.Y.A1.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("YB = 0x%x\n", buf[:])
}

func TestDataLargeWell(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	for i := 0; i < 5; i++ {
		var q bn254.G2Affine
		s, _ := rand.Int(rand.Reader, fr.Modulus())
		q.ScalarMultiplicationBase(s)
		fmt.Println("= random point in G2 =")
		var buf [32]byte
		q.X.A0.BigInt(new(big.Int)).FillBytes(buf[:])
		fmt.Printf("XA = 0x%x\n", buf[:])
		q.X.A1.BigInt(new(big.Int)).FillBytes(buf[:])
		fmt.Printf("XB = 0x%x\n", buf[:])
		q.Y.A0.BigInt(new(big.Int)).FillBytes(buf[:])
		fmt.Printf("YA = 0x%x\n", buf[:])
		q.Y.A1.BigInt(new(big.Int)).FillBytes(buf[:])
		fmt.Printf("YB = 0x%x\n", buf[:])
	}
}

func TestDataValidPairing(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	assert := test.NewAssert(t)
	// pairing where result is 1. Two pairs of inputs

	// sp1 * sq1 + sp2 * sq2 == 0
	// sq2 = (-sp1 * sq1) / sp2
	var p1, p2 bn254.G1Affine
	var q1, q2 bn254.G2Affine
	sp1, _ := rand.Int(rand.Reader, fr.Modulus())
	sp2, _ := rand.Int(rand.Reader, fr.Modulus())
	sq1, _ := rand.Int(rand.Reader, fr.Modulus())
	tmp := new(big.Int)
	tmp.ModInverse(sp2, fr.Modulus())
	tmp.Mul(tmp, sp1)
	tmp.Mul(tmp, sq1)
	tmp.Neg(tmp)
	sq2 := new(big.Int).Mod(tmp, fr.Modulus())

	p1.ScalarMultiplicationBase(sp1)
	p2.ScalarMultiplicationBase(sp2)
	q1.ScalarMultiplicationBase(sq1)
	q2.ScalarMultiplicationBase(sq2)
	ok, err := bn254.PairingCheck([]bn254.G1Affine{p1, p2}, []bn254.G2Affine{q1, q2})
	assert.NoError(err)
	assert.True(ok)
	fmt.Println("= double successful pairing =")
	printP(p1)
	printQ(q1)
	fmt.Println()
	printP(p2)
	printQ(q2)

	// pairing where result is 1. Three pairs of inputs
	// e(a, 2b) * e(2a, 2b) * e(-2a, 3b) == 1
	// 2 + 4 - 6 == 0
	var p3 bn254.G1Affine
	var q3 bn254.G2Affine
	sq2, _ = rand.Int(rand.Reader, fr.Modulus())
	sp3, _ := rand.Int(rand.Reader, fr.Modulus())
	n := new(big.Int)
	n.Add(n, new(big.Int).Mul(sp1, sq1))
	n.Add(n, new(big.Int).Mul(sp2, sq2))
	tmp.ModInverse(sp3, fr.Modulus())
	tmp.Mul(tmp, n)
	tmp.Neg(tmp)
	sq3 := new(big.Int).Mod(tmp, fr.Modulus())

	p1.ScalarMultiplicationBase(sp1)
	p2.ScalarMultiplicationBase(sp2)
	p3.ScalarMultiplicationBase(sp3)
	q1.ScalarMultiplicationBase(sq1)
	q2.ScalarMultiplicationBase(sq2)
	q3.ScalarMultiplicationBase(sq3)
	ok, err = bn254.PairingCheck([]bn254.G1Affine{p1, p2, p3}, []bn254.G2Affine{q1, q2, q3})
	assert.NoError(err)
	assert.True(ok)
	fmt.Print("\n\n")
	fmt.Println("= triple successful pairing =")
	printP(p1)
	printQ(q1)
	fmt.Println()
	printP(p2)
	printQ(q2)
	fmt.Println()
	printP(p3)
	printQ(q3)

	printPLimbs(p1)
	printQLimbs(q1)
	printPLimbs(p2)
	printQLimbs(q2)
	printPLimbs(p3)
	printQLimbs(q3)

	// pairing where result is 1. Four pairs of inputs
	// e(a, 2b) * e(3a, b) * e(a, 5b) * e(-2a, 5b) == 1
	// 2 + 3 + 5 - 10 == 0
	var p4 bn254.G1Affine
	var q4 bn254.G2Affine
	sq3, _ = rand.Int(rand.Reader, fr.Modulus())
	sp4, _ := rand.Int(rand.Reader, fr.Modulus())
	n = new(big.Int)
	n.Add(n, new(big.Int).Mul(sp1, sq1))
	n.Add(n, new(big.Int).Mul(sp2, sq2))
	n.Add(n, new(big.Int).Mul(sp3, sq3))
	tmp.ModInverse(sp4, fr.Modulus())
	tmp.Mul(tmp, n)
	tmp.Neg(tmp)
	sq4 := new(big.Int).Mod(tmp, fr.Modulus())

	p1.ScalarMultiplicationBase(sp1)
	p2.ScalarMultiplicationBase(sp2)
	p3.ScalarMultiplicationBase(sp3)
	p4.ScalarMultiplicationBase(sp4)
	q1.ScalarMultiplicationBase(sq1)
	q2.ScalarMultiplicationBase(sq2)
	q3.ScalarMultiplicationBase(sq3)
	q4.ScalarMultiplicationBase(sq4)
	ok, err = bn254.PairingCheck([]bn254.G1Affine{p1, p2, p3, p4}, []bn254.G2Affine{q1, q2, q3, q4})
	assert.NoError(err)
	assert.True(ok)
	fmt.Print("\n\n")
	fmt.Println("= quadruple successful pairing =")
	printP(p1)
	printQ(q1)
	fmt.Println()
	printP(p2)
	printQ(q2)
	fmt.Println()
	printP(p3)
	printQ(q3)
	fmt.Println()
	printP(p4)
	printQ(q4)
}

func TestG2TestData(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	assert := test.NewAssert(t)
	// test for a point not on the curve
	var Q bn254.G2Affine
	_, err := Q.X.A0.SetString("0x119606e6d3ea97cea4eff54433f5c7dbc026b8d0670ddfbe6441e31225028d31")
	assert.NoError(err)
	_, err = Q.X.A1.SetString("0x1d3df5be6084324da6333a6ad1367091ca9fbceb70179ec484543a58b8cb5d63")
	assert.NoError(err)
	_, err = Q.Y.A0.SetString("0x1b9a36ea373fe2c5b713557042ce6deb2907d34e12be595f9bbe84c144de86ef")
	assert.NoError(err)
	_, err = Q.Y.A1.SetString("0x49fe60975e8c78b7b31a6ed16a338ac8b28cf6a065cfd2ca47e9402882518ba0")
	assert.NoError(err)
	assert.False(Q.IsOnCurve())
	printQLimbs(Q)

	// test for a point on curve not in G2
	_, err = Q.X.A0.SetString("0x07192b9fd0e2a32e3e1caa8e59462b757326d48f641924e6a1d00d66478913eb")
	assert.NoError(err)
	_, err = Q.X.A1.SetString("0x15ce93f1b1c4946dd6cfbb3d287d9c9a1cdedb264bda7aada0844416d8a47a63")
	assert.NoError(err)
	_, err = Q.Y.A0.SetString("0x0fa65a9b48ba018361ed081e3b9e958451de5d9e8ae0bd251833ebb4b2fafc96")
	assert.NoError(err)
	_, err = Q.Y.A1.SetString("0x06e1f5e20f68f6dfa8a91a3bea048df66d9eaf56cc7f11215401f7e05027e0c6")
	assert.NoError(err)
	assert.True(Q.IsOnCurve())
	assert.False(Q.IsInSubGroup())
	printQLimbs(Q)

	// test for a point in G2
	Q.ScalarMultiplicationBase(big.NewInt(5678))
	printQLimbs(Q)
}

func TestDummyMillerLoopData(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	assert := test.NewAssert(t)
	_, _, p, q := bn254.Generators()
	lines := bn254.PrecomputeLines(q)
	mlres, err := bn254.MillerLoopFixedQ(
		[]bn254.G1Affine{p},
		[][2][len(bn254.LoopCounter)]bn254.LineEvaluationAff{lines},
	)
	assert.NoError(err)
	var one bn254.GT
	one.SetOne()
	printTLimbs(one)
	fmt.Println()
	printPLimbs(p)
	fmt.Println()
	printQLimbs(q)
	fmt.Println()
	printTLimbs(mlres)
}

func TestDummyMillerLoopFinalExpData(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	assert := test.NewAssert(t)
	_, _, p, q := bn254.Generators()
	var negP bn254.G1Affine
	negP.ScalarMultiplicationBase(big.NewInt(-1))
	lines := bn254.PrecomputeLines(q)
	mlres, err := bn254.MillerLoopFixedQ(
		[]bn254.G1Affine{negP},
		[][2][len(bn254.LoopCounter)]bn254.LineEvaluationAff{lines},
	)
	printTLimbs(mlres)
	assert.NoError(err)

	mlres2, err := bn254.MillerLoopFixedQ(
		[]bn254.G1Affine{p},
		[][2][len(bn254.LoopCounter)]bn254.LineEvaluationAff{lines},
	)
	assert.NoError(err)
	mlres.Mul(&mlres, &mlres2)
	res := bn254.FinalExponentiation(&mlres)
	assert.True(res.IsOne())
	assert.NoError(err)
	// var one bn254.GT
	fmt.Println()
	printPLimbs(p)
	fmt.Println()
	printQLimbs(q)
	// fmt.Println()
	// printTLimbs(mlres)
}

func TestDummyG2CheckData(t *testing.T) {
	t.Skip("skipping test, called manually when needed")
	assert := test.NewAssert(t)
	_, _, _, q := bn254.Generators()
	ok := q.IsInSubGroup()
	assert.True(ok)
	printQLimbs(q)
}

func printP(P bn254.G1Affine) {
	var buf [32]byte
	P.X.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("Ax = 0x%x\n", buf[:])
	P.Y.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("Ay = 0x%x\n", buf[:])
}

func printQ(Q bn254.G2Affine) {
	var buf [32]byte
	Q.X.A0.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("BxRe = 0x%x\n", buf[:])
	Q.X.A1.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("BxIm = 0x%x\n", buf[:])
	Q.Y.A0.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("ByRe = 0x%x\n", buf[:])
	Q.Y.A1.BigInt(new(big.Int)).FillBytes(buf[:])
	fmt.Printf("ByIm = 0x%x\n", buf[:])
}

func printPLimbs(P bn254.G1Affine) {
	px := P.X.Bytes()
	py := P.Y.Bytes()
	fmt.Printf("0x%x\n0x%x\n0x%x\n0x%x\n", px[0:16], px[16:32], py[0:16], py[16:32])
}

func printQLimbs(Q bn254.G2Affine) {
	qxre := Q.X.A0.Bytes()
	qxim := Q.X.A1.Bytes()
	qyre := Q.Y.A0.Bytes()
	qyim := Q.Y.A1.Bytes()
	fmt.Printf("0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n0x%x\n",
		qxre[0:16], qxre[16:32], qxim[0:16], qxim[16:32], qyre[0:16], qyre[16:32], qyim[0:16], qyim[16:32])
}

func printTLimbs(T bn254.GT) {
	for i, bb := range [][32]byte{
		T.C0.B0.A0.Bytes(),
		T.C0.B0.A1.Bytes(),
		T.C0.B1.A0.Bytes(),
		T.C0.B1.A1.Bytes(),
		T.C0.B2.A0.Bytes(),
		T.C0.B2.A1.Bytes(),
		T.C1.B0.A0.Bytes(),
		T.C1.B0.A1.Bytes(),
		T.C1.B1.A0.Bytes(),
		T.C1.B1.A1.Bytes(),
		T.C1.B2.A0.Bytes(),
		T.C1.B2.A1.Bytes(),
	} {
		_ = i
		fmt.Printf("0x%x\n0x%x\n", bb[0:16], bb[16:32])
	}
}
