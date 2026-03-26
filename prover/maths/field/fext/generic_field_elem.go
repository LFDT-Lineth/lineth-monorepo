package fext

import (
	"fmt"
	"math/big"

	"github.com/consensys/linea-monorepo/prover/maths/field"
)

// GenericFieldElem is a generic field element that can either be a Base field element or an extension field element.
// It should only be used in places where performance is not critical, as it incurs overhead due to the
// storing both a Base and extension field version, as well as checks and conversions between Base and extension elements.
type GenericFieldElem struct {
	Base   field.Element
	Ext    Element
	IsBase bool
}

func NewGenFieldFromBase(base field.Element) GenericFieldElem {
	var res GenericFieldElem
	res.Base.Set(&base)
	SetFromBase(&res.Ext, &base)
	res.IsBase = true
	return res
}

func (e *GenericFieldElem) GetBase() (field.Element, error) {
	if e.IsBase {
		return e.Base, nil
	} else {
		return field.Zero(), fmt.Errorf("cannot get Base element from an extension element")
	}
}

func (e *GenericFieldElem) GetExt() Element {
	if e.IsBase {
		SetFromBase(&e.Ext, &e.Base)
		return e.Ext
	} else {
		return e.Ext
	}
}

func NewGenFieldFromExt(ext Element) GenericFieldElem {
	var res GenericFieldElem
	res.Ext.Set(&ext)
	res.IsBase = false
	return res
}

func NewMinimalESHashFromExt(ext *Element) *GenericFieldElem {
	if IsBase(ext) {
		baseElem, _ := GetBase(ext)
		return &GenericFieldElem{
			Base:   *new(field.Element).Set(&baseElem),
			Ext:    *new(Element).Set(ext),
			IsBase: true,
		}
	} else {
		return &GenericFieldElem{
			Base:   field.Zero(),
			Ext:    *new(Element).Set(ext),
			IsBase: false,
		}
	}

}

func (e *GenericFieldElem) GetIsBase() bool {
	return e.IsBase
}

func (e *GenericFieldElem) IsEqual(inp *GenericFieldElem) bool {
	// first check if both are actual Base elements
	if IsBase(&e.Ext) && IsBase(&inp.Ext) {
		return e.Ext.B0.A0.Equal(&inp.Ext.B0.A0)
	} else {
		return e.Ext.Equal(&inp.Ext)
	}
}

func (e *GenericFieldElem) IsEqualBase(inp *field.Element) bool {
	if e.IsBase {
		return e.Base.Equal(inp)
	}
	if IsBase(&e.Ext) {
		actualBase, _ := GetBase(&e.Ext)
		return actualBase.Equal(inp)
	}
	return false
}

func (e *GenericFieldElem) IsEqualExt(inp *Element) bool {
	return e.Ext.Equal(inp)
}

func (e *GenericFieldElem) Set(inp *GenericFieldElem) *GenericFieldElem {
	e.IsBase = inp.IsBase
	e.Ext.Set(&inp.Ext)
	e.Base.Set(&inp.Base)
	return e
}

func (e *GenericFieldElem) Mul(inp *GenericFieldElem) *GenericFieldElem {
	if e.IsBase && inp.IsBase {
		e.Base.Mul(&e.Base, &inp.Base)
		e.IsBase = true
		SetFromBase(&e.Ext, &e.Base)
		return e
	} else {
		// not both are Base elements
		if e.IsBase {
			e.Ext.MulByElement(&inp.Ext, &e.Base)
		}
		if inp.IsBase {
			e.Ext.MulByElement(&e.Ext, &inp.Base)
		}
		if !e.IsBase && !inp.IsBase {
			// both are extensions
			e.Ext.Mul(&e.Ext, &inp.Ext)
		}

		// Check if the final result is a Base element
		if IsBase(&e.Ext) {
			actualBase, _ := GetBase(&e.Ext)
			e.Base.Set(&actualBase)
			e.IsBase = true
		} else {
			e.Base.SetZero()
			e.IsBase = false
		}
		return e
	}
}

func (e *GenericFieldElem) Add(inp *GenericFieldElem) *GenericFieldElem {

	switch {
	case e.IsBase && inp.IsBase:
		e.Base.Add(&e.Base, &inp.Base)
		SetFromBase(&e.Ext, &e.Base)

	case e.IsBase && !inp.IsBase:
		AddByBase(&e.Ext, &inp.Ext, &e.Base)
		e.Base.SetZero()
		e.IsBase = false

	case !e.IsBase && inp.IsBase:
		AddByBase(&e.Ext, &e.Ext, &inp.Base)

	case !e.IsBase && !inp.IsBase:
		e.Ext.Add(&e.Ext, &inp.Ext)
	}

	return e
}

func (e *GenericFieldElem) Div(inp *GenericFieldElem) *GenericFieldElem {
	if e.IsBase && inp.IsBase {
		e.Base.Div(&e.Base, &inp.Base)
		SetFromBase(&e.Ext, &e.Base)
		e.IsBase = true
	} else {
		// not both are Base elements
		if e.IsBase {
			e.Ext.Div(&e.Ext, &inp.Ext)
		}
		if inp.IsBase {
			DivByBase(&e.Ext, &e.Ext, &inp.Base)
		}
		if !e.IsBase && !inp.IsBase {
			// both are extensions
			e.Ext.Div(&e.Ext, &inp.Ext)
		}

		// Check if the final result is a Base element
		if IsBase(&e.Ext) {
			actualBase, _ := GetBase(&e.Ext)
			e.Base.Set(&actualBase)
			e.IsBase = true
		} else {
			e.Base.SetZero()
			e.IsBase = false
		}
	}
	return e
}

func (e *GenericFieldElem) String() string {
	if e.IsBase {
		return e.Base.String()
	}
	return e.Ext.String()
}

func (e *GenericFieldElem) IsZero() bool {
	if e.IsBase {
		return e.Base.IsZero()
	} else {
		return e.Ext.IsZero()
	}
}

func (e *GenericFieldElem) IsOne() bool {
	if e.IsBase {
		return e.Base.IsOne()
	} else {
		return e.Ext.IsOne()
	}
}

func GenericFieldOne() GenericFieldElem {
	return NewGenFieldFromBase(field.One())
}

func GenericFieldZero() GenericFieldElem {
	return NewGenFieldFromBase(field.Zero())
}

func (e *GenericFieldElem) Square(inp *GenericFieldElem) *GenericFieldElem {
	if inp.IsBase {
		// inp is a Base element
		e.Base.Square(&inp.Base)
		SetFromBase(&e.Ext, &e.Base)
		e.IsBase = true
		return e
	} else {
		// inp is an extension
		e.Ext.Square(&inp.Ext)
		if IsBase(&e.Ext) {
			actualBase, _ := GetBase(&e.Ext)
			e.Base.Set(&actualBase)
			e.IsBase = true
		} else {
			e.Base.SetZero()
			e.IsBase = false
		}
		return e
	}
}

func (e *GenericFieldElem) Exp(inp *GenericFieldElem, exponent *big.Int) *GenericFieldElem {
	if inp.IsBase {
		e.Base.Exp(inp.Base, exponent)
		SetFromBase(&e.Ext, &e.Base)
		return e
	} else {
		e.Ext.Exp(inp.Ext, exponent)
		if IsBase(&e.Ext) {
			actualBase, _ := GetBase(&e.Ext)
			e.Base.Set(&actualBase)
			e.IsBase = true
		} else {
			e.Base.SetZero()
			e.IsBase = false
		}
		return e
	}
}

func (e *GenericFieldElem) SetInt64(v int64) *GenericFieldElem {
	e.Base.SetInt64(v)
	SetFromBase(&e.Ext, &e.Base)
	e.IsBase = true
	return e
}

func (e *GenericFieldElem) GenericText(base int) string {
	if e.GetIsBase() {
		return e.Base.Text(base)
	} else {
		return Text(&e.Ext, base)
	}
}

func SetGenericInt64(v int64) GenericFieldElem {
	var e GenericFieldElem
	e.Base.SetInt64(v)
	SetFromBase(&e.Ext, &e.Base)
	e.IsBase = true
	return e
}
func (z *GenericFieldElem) GenericBytes() []byte {
	if z.GetIsBase() {
		res := z.Base.Bytes()
		return res[:]
	} else {
		res := Bytes(&z.Ext)
		return res[:]
	}
}

func (z *GenericFieldElem) Inverse(x *GenericFieldElem) *GenericFieldElem {
	if x.GetIsBase() {
		z.Base.Inverse(&x.Base)
	} else {
		z.Ext.Inverse(&x.Ext)
	}
	return z
}

func PrettifyGeneric(a []GenericFieldElem) string {
	res := "["

	for i := range a {
		// Discards the case first element when adding a comma
		if i > 0 {
			res += ", "
		}

		res += fmt.Sprintf("%v", a[i].String())
	}
	res += "]"

	return res
}
