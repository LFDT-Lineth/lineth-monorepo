package fiatshamir

import (
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
)

// Outputs of [transcript] for the original buffering implementation.
var (
	goldenDigest = [8]koalabear.Element{koalabear.Element{0x7567a418}, koalabear.Element{0x4e90ed96}, koalabear.Element{0x15c734}, koalabear.Element{0x4ea83c63}, koalabear.Element{0x1e91082a}, koalabear.Element{0x5e46de19}, koalabear.Element{0x333d57ea}, koalabear.Element{0x6a29522e}}
	goldenExts   = []extensions.E6{extensions.E6{B0: extensions.E2{A0: koalabear.Element{0x30499f00}, A1: koalabear.Element{0x445e8831}}, B1: extensions.E2{A0: koalabear.Element{0x6efe7597}, A1: koalabear.Element{0x12abf8dc}}, B2: extensions.E2{A0: koalabear.Element{0x463186ea}, A1: koalabear.Element{0x1877bd0b}}}, extensions.E6{B0: extensions.E2{A0: koalabear.Element{0x4b3e2dfe}, A1: koalabear.Element{0x229ed8a7}}, B1: extensions.E2{A0: koalabear.Element{0x270e3798}, A1: koalabear.Element{0x2c97f697}}, B2: extensions.E2{A0: koalabear.Element{0x2cd0fc07}, A1: koalabear.Element{0x6414c681}}}, extensions.E6{B0: extensions.E2{A0: koalabear.Element{0x314e0dbe}, A1: koalabear.Element{0x7dc7eb3c}}, B1: extensions.E2{A0: koalabear.Element{0x199b6fc3}, A1: koalabear.Element{0x6927f4ef}}, B2: extensions.E2{A0: koalabear.Element{0x441219b2}, A1: koalabear.Element{0x187e28ed}}}, extensions.E6{B0: extensions.E2{A0: koalabear.Element{0x24a7810}, A1: koalabear.Element{0x23f0ee90}}, B1: extensions.E2{A0: koalabear.Element{0x45575a44}, A1: koalabear.Element{0xbe051e5}}, B2: extensions.E2{A0: koalabear.Element{0x31fb1faa}, A1: koalabear.Element{0x2a35e677}}}}
	goldenInts   = []int{793, 3, 446, 295, 898, 484, 683, 440, 866, 32, 81, 668, 953, 23, 438, 915, 496, 617, 985}
)
