package polynomials

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/gnark-crypto/utils"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// fftExtInplace applies the FFT to each of the 4 coordinates of an []Ext slice
// individually, using the base-field FFT provided by *fft.Domain.
// After the call, poly holds the Lagrange (evaluation) form of the input.
func fftExtInplace(poly []field.Ext, d *fft.Domain) {
	size := len(poly)
	buf := make([]field.Element, size)

	copyCoord := func(get func(int) *field.Element) {
		for i := 0; i < size; i++ {
			buf[i].Set(get(i))
		}
		d.FFT(buf, fft.DIF)
		utils.BitReverse(buf)
		for i := 0; i < size; i++ {
			get(i).Set(&buf[i])
		}
	}

	copyCoord(func(i int) *field.Element { return &poly[i].B0.A0 })
	copyCoord(func(i int) *field.Element { return &poly[i].B0.A1 })
	copyCoord(func(i int) *field.Element { return &poly[i].B1.A0 })
	copyCoord(func(i int) *field.Element { return &poly[i].B1.A1 })
}

// fftBaseInplace applies the FFT to a []Element slice, converting from
// canonical form to Lagrange (evaluation) form.
func fftBaseInplace(poly []field.Element, d *fft.Domain) {
	d.FFT(poly, fft.DIF)
	utils.BitReverse(poly)
}

// ifftExtInplace applies the inverse FFT coordinate-by-coordinate.
func ifftExtInplace(poly []field.Ext, d *fft.Domain) {
	size := len(poly)
	buf := make([]field.Element, size)

	copyCoord := func(get func(int) *field.Element) {
		for i := 0; i < size; i++ {
			buf[i].Set(get(i))
		}
		d.FFTInverse(buf, fft.DIF)
		utils.BitReverse(buf)
		for i := 0; i < size; i++ {
			get(i).Set(&buf[i])
		}
	}

	copyCoord(func(i int) *field.Element { return &poly[i].B0.A0 })
	copyCoord(func(i int) *field.Element { return &poly[i].B0.A1 })
	copyCoord(func(i int) *field.Element { return &poly[i].B1.A0 })
	copyCoord(func(i int) *field.Element { return &poly[i].B1.A1 })
}

func TestEvalLagrange(t *testing.T) {
	const size = 64
	d := fft.NewDomain(uint64(size))
	rng := newRng()

	t.Run("ext_poly", func(t *testing.T) {
		// Generate a random polynomial in canonical (Ext) form
		canonical := make([]field.Ext, size)
		for i := range canonical {
			canonical[i] = randExt(rng)
		}
		z := field.ElemFromExt(randExt(rng))

		// Reference: evaluate in canonical form using Horner
		want := hornerExt(canonical, z.AsExt())

		// Convert to Lagrange form via FFT
		lagrange := make([]field.Ext, size)
		copy(lagrange, canonical)
		fftExtInplace(lagrange, d)

		got := EvalLagrange(field.VecFromExt(lagrange), z)

		if !extEq(got.AsExt(), want) {
			t.Fatalf("ext_poly: got %v, want %v", got.AsExt(), want)
		}
	})

	t.Run("base_poly", func(t *testing.T) {
		// Generate a random polynomial in canonical (base) form
		canonical := make([]field.Element, size)
		for i := range canonical {
			canonical[i] = randBase(rng)
		}
		z := field.ElemFromExt(randExt(rng))

		// Reference: lift to Ext and use hornerExt
		canonicalExt := make([]field.Ext, size)
		for i, b := range canonical {
			canonicalExt[i] = field.Lift(b)
		}
		want := hornerExt(canonicalExt, z.AsExt())

		// Convert to Lagrange form via FFT
		lagrange := make([]field.Element, size)
		copy(lagrange, canonical)
		fftBaseInplace(lagrange, d)

		got := EvalLagrange(field.VecFromBase(lagrange), z)

		if !extEq(got.AsExt(), want) {
			t.Fatalf("base_poly: got %v, want %v", got.AsExt(), want)
		}
	})
}

func TestEvalLagrangeBatch(t *testing.T) {
	const size = 64
	d := fft.NewDomain(uint64(size))
	rng := newRng()

	canonical := make([]field.Ext, size)
	for i := range canonical {
		canonical[i] = randExt(rng)
	}

	lagrange := make([]field.Ext, size)
	copy(lagrange, canonical)
	fftExtInplace(lagrange, d)

	poly := field.VecFromExt(lagrange)
	zs := []field.Gen{
		field.ElemFromExt(randExt(rng)),
		field.ElemFromExt(randExt(rng)),
		field.ElemFromExt(randExt(rng)),
	}

	batch := EvalLagrangeBatch(poly, zs)
	if len(batch) != len(zs) {
		t.Fatalf("batch length: got %d, want %d", len(batch), len(zs))
	}
	for i, z := range zs {
		want := EvalLagrange(poly, z)
		if !extEq(batch[i].AsExt(), want.AsExt()) {
			t.Fatalf("z[%d]: batch=%v single=%v", i, batch[i].AsExt(), want.AsExt())
		}
	}
}

func TestComputeLagrangeAtZ(t *testing.T) {
	const size = 8
	d := fft.NewDomain(uint64(size))
	rng := newRng()

	z := field.ElemFromExt(randExt(rng))

	// Reference: for each i, build the i-th Lagrange basis polynomial
	// as an indicator in evaluation form, IFFT to canonical, then evaluate at z.
	reference := make([]field.Gen, size)
	for i := 0; i < size; i++ {
		indicator := make([]field.Ext, size) // all zeros
		indicator[i].B0.A0.SetOne()
		ifftExtInplace(indicator, d)
		reference[i] = field.ElemFromExt(hornerExt(indicator, z.AsExt()))
	}

	got := ComputeLagrangeAtZ(z, d.Cardinality)
	if len(got) != size {
		t.Fatalf("len: got %d, want %d", len(got), size)
	}
	for i := range got {
		if !extEq(got[i].AsExt(), reference[i].AsExt()) {
			t.Fatalf("L[%d]: got %v, want %v", i, got[i].AsExt(), reference[i].AsExt())
		}
	}
}
