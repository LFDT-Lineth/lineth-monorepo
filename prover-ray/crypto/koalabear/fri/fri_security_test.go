package fri

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/commitment"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/hash"
	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

func TestCheckQueryRejectsOpeningAtWrongLeafIndex(t *testing.T) {
	p, err := NewParams(8, 2, 1, commitment.DefaultLeafHasher, commitment.DefaultNodeHasher)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}

	opened := securityTestElement(7)
	layer := []koalabear.Element{
		opened, opened, securityTestElement(11), securityTestElement(13),
		opened, opened, securityTestElement(17), securityTestElement(19),
	}
	tree, err := p.BuildLevelTree(layer)
	if err != nil {
		t.Fatalf("BuildLevelTree: %v", err)
	}

	const challengeIndex = 2
	const openedIdx = 0
	path, err := tree.OpenProof(openedIdx)
	if err != nil {
		t.Fatalf("OpenProof: %v", err)
	}

	query := Query{Layers: []QueryLayer{{
		Field:     field.KindBase,
		LeafPBase: layer[openedIdx],
		LeafQBase: layer[openedIdx+len(layer)/2],
		Path:      path,
	}}}
	introductions, err := newLevelIntroductions(p, []int{p.D})
	if err != nil {
		t.Fatalf("newLevelIntroductions: %v", err)
	}

	err = checkQuery(
		challengeIndex,
		query,
		nil,
		nil,
		introductions,
		nil,
		[]hash.Digest{tree.Root()},
		[]koalabear.Element{opened},
		[]koalabear.Element{{}},
		p,
	)
	if err == nil {
		t.Fatalf("checkQuery accepted a FRI opening at leaf %d for challenge-derived leaf %d",
			openedIdx, challengeIndex)
	}
}

func securityTestElement(v uint64) koalabear.Element {
	var e koalabear.Element
	e.SetUint64(v)
	return e
}
