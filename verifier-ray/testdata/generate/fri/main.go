// Package main generates verifier-ray's crypto.merkle Zig test vectors from
// prover-ray's exported Merkle-tree API.
//
// This is a separate Go module from testdata/generate (the wiop/vanishing
// fixture generator) so its prover-ray dependency can be pinned
// independently: bumping this module's pin never has to touch, or be
// blocked by, unrelated code in that other module.
//
// Only exported prover-ray symbols are used here (fri.NewTree,
// Tree.Root/OpenBranch, poseidon2.NewMDHasher): this generator has no
// white-box access to prover-ray internals.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

func main() {
	if err := writeMerkleFixtures(); err != nil {
		panic(err)
	}
}

// ─── Zig-literal helpers ─────────────────────────────────────────────────
//
// Mirrors testdata/generate/main.go's own helpers of the same name and
// behavior; duplicated rather than shared because the two are separate Go
// modules with independently-pinned dependencies.

func elem(v uint64) field.Element {
	var e field.Element
	e.SetUint64(v)
	return e
}

func u(e field.Element) uint64 {
	return e.Uint64()
}

func oct8(values field.Octuplet) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprintf("%d", u(value))
	}
	return ".{ " + strings.Join(parts, ", ") + " }"
}

func commitmentSlice(values []field.Octuplet) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = oct8(value)
	}
	return ".{ " + strings.Join(parts, ", ") + " }"
}

func runZigFmt(data []byte) ([]byte, error) {
	tmp, err := os.CreateTemp("", "verifier-ray-vectors-*.zig")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	cmd := os.Getenv("ZIG")
	if cmd == "" {
		cmd = "zig"
	}
	if err := exec.Command(cmd, "fmt", tmp.Name()).Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(tmp.Name())
}

// ─── Merkle fixtures: prover-ray's exported Tree/Branch surface ────────────
//
// fri.NewTree([][]Octuplet{nil, ..., leaves}) (leaves at index log2(n), nil
// above) is bit-identical to the package's unexported newCompleteBinaryTree:
// same node count, same hashNode(left, right, nil) internal nodes, since the
// nil upper levels leave every Aux slot nil.

// hashOne hashes a single small integer into a leaf octuplet.
func hashOne(v uint64) field.Octuplet {
	h := poseidon2.NewMDHasher()
	h.WriteElements(elem(v))
	return h.SumDigest()
}

// wrongOctuplet stands in for "some digest that must not match the honest
// one"; the exact value is immaterial as long as it differs.
func wrongOctuplet() field.Octuplet { return hashOne(999_999) }

type merkleCase struct {
	Name        string
	Leaf        field.Octuplet
	Siblings    []field.Octuplet
	Index       int
	Root        field.Octuplet
	ExpectMatch bool
}

func buildMerkleCases() []merkleCase {
	var cases []merkleCase

	// Two-leaf tree: both parities in a single level.
	{
		leaves := []field.Octuplet{hashOne(1), hashOne(2)}
		tree := fri.NewTree([][]field.Octuplet{nil, leaves})
		root := tree.Root()
		for _, idx := range []int{0, 1} {
			b := tree.OpenBranch(idx)
			cases = append(cases, merkleCase{
				Name: fmt.Sprintf("two_leaf_index_%d", idx),
				Leaf: b.Leaf, Siblings: b.Siblings,
				Index: idx, Root: root, ExpectMatch: true,
			})
		}

		b := tree.OpenBranch(0)
		siblings := append([]field.Octuplet(nil), b.Siblings...)
		siblings[len(siblings)-1] = wrongOctuplet()
		cases = append(cases, merkleCase{
			Name: "two_leaf_wrong_sibling",
			Leaf: b.Leaf, Siblings: siblings,
			Index: 0, Root: root, ExpectMatch: false,
		})
	}

	// Four-leaf tree: a deeper tree, proving the walk threads correctly
	// across more than one level.
	{
		leaves := []field.Octuplet{hashOne(10), hashOne(20), hashOne(30), hashOne(40)}
		tree := fri.NewTree([][]field.Octuplet{nil, nil, leaves})
		root := tree.Root()
		b := tree.OpenBranch(1)
		cases = append(cases, merkleCase{
			Name: "four_leaf_index_1",
			Leaf: b.Leaf, Siblings: b.Siblings,
			Index: 1, Root: root, ExpectMatch: true,
		})
	}

	return cases
}

func writeMerkleCase(out *bytes.Buffer, c merkleCase) {
	fmt.Fprintf(out, "    .{\n")
	fmt.Fprintf(out, "        .name = \"%s\",\n", c.Name)
	fmt.Fprintf(out, "        .leaf = %s,\n", oct8(c.Leaf))
	fmt.Fprintf(out, "        .siblings = &%s,\n", commitmentSlice(c.Siblings))
	fmt.Fprintf(out, "        .index = %d,\n", c.Index)
	fmt.Fprintf(out, "        .root = %s,\n", oct8(c.Root))
	fmt.Fprintf(out, "        .expect_match = %t,\n", c.ExpectMatch)
	fmt.Fprintf(out, "    },\n")
}

func writeMerkleFixtures() error {
	var out bytes.Buffer
	fmt.Fprintln(&out, "// Code generated by verifier-ray/testdata/generate/fri; DO NOT EDIT.")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "pub const MerkleCase = struct {")
	fmt.Fprintln(&out, "    name: []const u8,")
	fmt.Fprintln(&out, "    leaf: [8]u32,")
	fmt.Fprintln(&out, "    siblings: []const [8]u32,")
	fmt.Fprintln(&out, "    index: usize,")
	fmt.Fprintln(&out, "    root: [8]u32,")
	fmt.Fprintln(&out, "    expect_match: bool,")
	fmt.Fprintln(&out, "};")
	fmt.Fprintln(&out)

	cases := buildMerkleCases()
	fmt.Fprintln(&out, "pub const merkle_cases = [_]MerkleCase{")
	for _, c := range cases {
		writeMerkleCase(&out, c)
	}
	fmt.Fprintln(&out, "};")

	data := out.Bytes()
	zigfmt, err := runZigFmt(data)
	if err == nil {
		data = zigfmt
	}
	return os.WriteFile(filepath.Join("..", "..", "generated", "fri.zig"), data, 0o644)
}
