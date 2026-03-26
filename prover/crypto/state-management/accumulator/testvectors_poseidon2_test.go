//go:build !fuzzlight

package accumulator_test

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/stretchr/testify/require"
)

// Test the root hash of an empty accumulator
func TestEmptyAccumulatorPoseidon2(t *testing.T) {

	acc := newTestAccumulatorPoseidon2DummyVal()

	// next free node
	require.Equal(t, int64(2), acc.NextFreeNode)

	// Check the value of the first leaf, it should be "head" with an updated next value
	{
		leafOpening := acc.Data.MustGet(0).LeafOpening
		require.Equal(
			t,
			"LeafOpening{Prev: 0, Next: 1, HKey: 0x0000000000000000000000000000000000000000000000000000000000000000, HVal: 0x0000000000000000000000000000000000000000000000000000000000000000}",
			leafOpening.String(),
		)

		// also checks its hash value (i.e, the corresponding leaf in the tree)
		require.Equal(t, "0x2f5de7ad279a134761de0d89702e52743b2f04f41b86cc5400dfae1d4b981340", leafOpening.Hash().Hex())
	}

	// check the value of the second leaf opening. It should be tail with an updated prev value
	{
		leafOpening := acc.Data.MustGet(1).LeafOpening
		require.Equal(
			t,
			"LeafOpening{Prev: 0, Next: 1, HKey: 0x7f0000007f0000007f0000007f0000007f0000007f0000007f0000007f000000, HVal: 0x0000000000000000000000000000000000000000000000000000000000000000}",
			leafOpening.String(),
		)

		// also check its hash value
		require.Equal(t, "0x5c73480b5e85fc2c3ab61fc155cf475b74590d005d8916b85fdd5c32773b1255", leafOpening.Hash().Hex())
	}

	// root of the subtree (e.g. exluding the next free node)
	require.Equal(
		t,
		"0x3a00a8e34a16f8a1225fee734816edb326f783bd6678d793345a28f046586ba6",
		acc.SubTreeRoot().Hex(),
	)

	// root of the complete accumulator (e.g including the last node)
	require.Equal(
		t,
		"0x2fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df7",
		acc.TopRoot().Hex(),
	)
}

// Test the root after inserting a new entry
func TestInsertionRootHashPoseidon2(t *testing.T) {

	acc := newTestAccumulatorPoseidon2DummyVal()

	key := dumkey(58) // "KEY_58"
	val := dumval(42) // "VAL_42"

	_ = acc.InsertAndProve(key, val)

	// We inserted, so the next free node should have been increased
	require.Equal(t, int64(3), acc.NextFreeNode)

	// Check the value of the first leaf, it should be "head" with an updated next value
	{
		leafOpening := acc.Data.MustGet(0).LeafOpening
		require.Equal(t, int64(0), leafOpening.Prev)
		require.Equal(t, int64(2), leafOpening.Next)
		require.Equal(
			t,
			"LeafOpening{Prev: 0, Next: 2, HKey: 0x0000000000000000000000000000000000000000000000000000000000000000, HVal: 0x0000000000000000000000000000000000000000000000000000000000000000}",
			leafOpening.String(),
		)

		// also checks its hash value (i.e, the corresponding leaf in the tree)
		leaf := leafOpening.Hash()
		require.Equal(t, "0x22e7a2ea33304d915fd93eac62b418ba7ca915131bea42827670d7f31a3b3445", leaf.Hex())
	}

	// check the value of the second leaf opening. It should be tail with an updated prev value
	{
		leafOpening := acc.Data.MustGet(1).LeafOpening
		require.Equal(
			t,
			"LeafOpening{Prev: 2, Next: 1, HKey: 0x7f0000007f0000007f0000007f0000007f0000007f0000007f0000007f000000, HVal: 0x0000000000000000000000000000000000000000000000000000000000000000}",
			leafOpening.String(),
		)

		// also check its hash value
		leaf := leafOpening.Hash()
		require.Equal(t, "0x7172ab730766d9b6367938c031afbd13607d95796a60930945ab6b88434339b2", leaf.Hex())
	}

	// check the value of the third leaf opening, corresponds to the inserted entry
	{
		leafOpening := acc.Data.MustGet(2).LeafOpening
		require.Equal(
			t,
			"LeafOpening{Prev: 0, Next: 1, HKey: 0x2a5104052407a7e1473dc2b03101215d22e5e4d5411072b33909be0129f31faa, HVal: 0x40549e906a7e4b076930ae253c51fb5f0239607b45f47349247c8e5a70a649b8}",
			leafOpening.String(),
		)

		// also check its hash value
		leaf := leafOpening.Hash()
		require.Equal(t, "0x05f2b1de703588c454c171b1380e4ca0096466bc0b9b18b7741976ff291c699e", leaf.Hex())
	}

	// root of the subtree (e.g. exluding the next free node)
	require.Equal(
		t,
		"0x2f20cbc26937b220551a344d548fccf216eec47b13b844c53ff76ade2f4c174d",
		acc.SubTreeRoot().Hex(),
	)

	// root of the complete accumulator (e.g including the last node)
	require.Equal(
		t,
		"0x52dd53ad66b0791d64d5edfb491715690998877d448e1fe11dcdc8d226bfe411",
		acc.TopRoot().Hex(),
	)
}

// Test the root after inserting a new entry, and then updateting it
func TestInsertAndUpdateRootHashPoseidon2(t *testing.T) {

	acc := newTestAccumulatorPoseidon2DummyVal()

	key := dumkey(58)    // "KEY_58"
	val := dumval(41)    // "VAL_41"
	newval := dumval(42) // "VAL_42"

	_ = acc.InsertAndProve(key, val)
	_ = acc.UpdateAndProve(key, newval)

	// Note : the tree should be in exactly the same state as after directly
	// inserting 42,
	// The same roots as in TestInsertionRootHashPoseidon2

	// We inserted, so the next free node should have been increased
	require.Equal(t, int64(3), acc.NextFreeNode)

	// root of the subtree (e.g. exluding the next free node)
	require.Equal(
		t,
		"0x2f20cbc26937b220551a344d548fccf216eec47b13b844c53ff76ade2f4c174d",
		acc.SubTreeRoot().Hex(),
	)

	// root of the complete accumulator (e.g including the last node)
	require.Equal(
		t,
		"0x52dd53ad66b0791d64d5edfb491715690998877d448e1fe11dcdc8d226bfe411",
		acc.TopRoot().Hex(),
	)
}

// Test the root after inserting a new entry and deleting right away
func TestInsertAndDeleteRootHashPoseidon2(t *testing.T) {

	acc := newTestAccumulatorPoseidon2DummyVal()

	key := dumkey(58) // "KEY_58"
	val := dumval(41) // "VAL_41"

	_ = acc.InsertAndProve(key, val)
	_ = acc.DeleteAndProve(key)

	// We inserted, so the next free node should have been increased
	require.Equal(t, int64(3), acc.NextFreeNode)

	// root of the subtree (e.g. exluding the next free node). It equals the one
	// of the empty tree
	// The same SubTreeRoot as in TestEmptyAccumulatorPoseidon2, but the TopRoot are different as the NextFreeNode are distinct
	require.Equal(
		t,
		"0x3a00a8e34a16f8a1225fee734816edb326f783bd6678d793345a28f046586ba6",
		acc.SubTreeRoot().Hex(),
	)

	// root of the complete accumulator (e.g including the last node). It does not
	// equal the one of the empty subtree because we have bumped next free node when
	// inserting.
	require.Equal(
		t,
		"0x391010e55fad441a1e3a33b11fb8271f68e86b6c1812ebd402d678d343fded62",
		acc.TopRoot().Hex(),
	)
}

func newTestAccumulatorPoseidon2() *accumulator.ProverState[types.EthAddress, types.Account] {

	return accumulator.InitializeProverState[types.EthAddress, types.Account](locationTesting)
}

// Test the root after inserting a new entry, without dummy key and value types
func TestRealKeyAndVal(t *testing.T) {

	acc := newTestAccumulatorPoseidon2()

	key, _ := types.AddressFromHex("0x2400000000000000000000000000000000000000")

	account :=
		types.Account{
			Nonce:          65,
			Balance:        big.NewInt(835),
			StorageRoot:    types.MustHexToKoalabearOctuplet("0x2fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df7"),
			LineaCodeHash:  types.MustHexToKoalabearOctuplet("0x0656ab853b3f52840362a8177e217b630c3f876b11e848365145aa24220647fc"),
			KeccakCodeHash: types.FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
			CodeSize:       0,
		}

	_ = acc.InsertAndProve(key, account)

	// We inserted, so the next free node should have been increased
	require.Equal(t, int64(3), acc.NextFreeNode)

	// root of the subtree (e.g. exluding the next free node)
	require.Equal(
		t,
		"0x75a23718103d010a0357dee12411e1a036a989fa05adb8e05e5fdd5472ad37c9",
		acc.SubTreeRoot().Hex(),
	)

	// root of the complete accumulator (e.g including the last node)
	require.Equal(
		t,
		"0x6b5d2e111a55e55826396df03ac3d0055dc4a88671edbcc8274db3f0117e0f97",
		acc.TopRoot().Hex(),
	)
}

// Test Proof from ReadAndProve after inserting a new entry
func TestProofFromReadAndProve(t *testing.T) {

	acc := newTestAccumulatorPoseidon2DummyVal()

	key := dumkey(36) // "KEY_36"
	val := dumval(32) // "VAL_32"

	_ = acc.InsertAndProve(key, val)
	// Snapshot the verifier after the insertions because of the verifier
	ver := acc.VerifierState()
	trace := acc.ReadNonZeroAndProve(key)

	require.Equal(
		t,
		"0x0000002400000024000000240000002400000024000000240000002400000024",
		key.Hex(),
	)

	require.Equal(
		t,
		"0x0000002000000020000000200000002000000020000000200000002000000020",
		val.Hex(),
	)

	err := ver.ReadNonZeroVerify(trace)
	require.NoErrorf(t, err, "check #%v - trace %++v", key, trace)
	require.Equal(t, val, trace.Value)

}

// Test Proof from InsertAndProve after inserting a new entry
func TestProofFromInsertAndProve(t *testing.T) {

	acc := newTestAccumulatorPoseidon2()

	key, _ := types.AddressFromHex("0x2400000000000000000000000000000000000000")

	account :=
		types.Account{
			Nonce:          65,
			Balance:        big.NewInt(835),
			StorageRoot:    types.MustHexToKoalabearOctuplet("0x2fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df7"),
			LineaCodeHash:  types.MustHexToKoalabearOctuplet("0x0656ab853b3f52840362a8177e217b630c3f876b11e848365145aa24220647fc"),
			KeccakCodeHash: types.FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
			CodeSize:       0,
		}

	encodedAccount := "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000041000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003432fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df70656ab853b3f52840362a8177e217b630c3f876b11e848365145aa24220647fc0000c5d200004601000086f70000233c0000927e00007db20000dcc7000003c00000e5000000b6530000ca820000273b00007bfa0000d80400005d850000a47000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

	trace := acc.InsertAndProve(key, account)

	require.Equal(
		t,
		"0x2400000000000000000000000000000000000000",
		key.Hex(),
	)

	buf := &bytes.Buffer{}
	_, err := account.WriteTo(buf)
	require.NoError(t, err)
	val := utils.HexEncodeToString(buf.Bytes())
	require.Equal(
		t,
		encodedAccount,
		val,
	)

	require.Equal(
		t,
		"0x3a00a8e34a16f8a1225fee734816edb326f783bd6678d793345a28f046586ba6",
		trace.OldSubRoot.Hex(),
	)

	require.Equal(
		t,
		"0x75a23718103d010a0357dee12411e1a036a989fa05adb8e05e5fdd5472ad37c9",
		trace.NewSubRoot.Hex(),
	)

	fmt.Printf("Prev Proof: %v\n\n", trace.ProofMinus.String())
	fmt.Printf("New Proof: %v\n\n", trace.ProofNew.String())
	fmt.Printf("Next Proof: %v\n\n", trace.ProofPlus.String())

}

// Dummy hashable type that we can use for the accumulator
type DummyFullKey = types.FullBytes32
type DummyFullVal = types.FullBytes32

func newTestAccumulatorPoseidon2DummyFullVal() *accumulator.ProverState[DummyFullKey, DummyFullVal] {

	return accumulator.InitializeProverState[DummyFullKey, DummyFullVal](locationTesting)
}

// Test Proof from InsertAndProve after inserting two new entries
func TestProofFromInsertTwoAndProve(t *testing.T) {

	acc := newTestAccumulatorPoseidon2()

	fmt.Printf("------------------- Trace 1: insert account 1 ------------------\n")

	key, _ := types.AddressFromHex("0x2400000000000000000000000000000000000000")

	account :=
		types.Account{
			Nonce:          65,
			Balance:        big.NewInt(835),
			StorageRoot:    types.MustHexToKoalabearOctuplet("0x2fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df7"),
			LineaCodeHash:  types.MustHexToKoalabearOctuplet("0x0656ab853b3f52840362a8177e217b630c3f876b11e848365145aa24220647fc"),
			KeccakCodeHash: types.FullBytes32FromHex("0xc5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"),
			CodeSize:       0,
		}

	encodedAccount := "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000041000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003432fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df70656ab853b3f52840362a8177e217b630c3f876b11e848365145aa24220647fc0000c5d200004601000086f70000233c0000927e00007db20000dcc7000003c00000e5000000b6530000ca820000273b00007bfa0000d80400005d850000a47000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

	json, _ := account.MarshalJSON()
	fmt.Printf("Account JSON: %s\n", string(json))
	trace := acc.InsertAndProve(key, account)
	require.Equal(t, int64(3), acc.NextFreeNode)

	require.Equal(
		t,
		"0x2400000000000000000000000000000000000000",
		key.Hex(),
	)

	buf := &bytes.Buffer{}
	_, err := account.WriteTo(buf)
	require.NoError(t, err)
	val := utils.HexEncodeToString(buf.Bytes())
	require.Equal(
		t,
		encodedAccount,
		val,
	)

	require.Equal(
		t,
		"0x3a00a8e34a16f8a1225fee734816edb326f783bd6678d793345a28f046586ba6",
		trace.OldSubRoot.Hex(),
	)

	require.Equal(
		t,
		"0x75a23718103d010a0357dee12411e1a036a989fa05adb8e05e5fdd5472ad37c9",
		trace.NewSubRoot.Hex(),
	)

	fmt.Printf("Prev Proof: %v\n\n", trace.ProofMinus.String())
	fmt.Printf("New Proof: %v\n\n", trace.ProofNew.String())
	fmt.Printf("Next Proof: %v\n\n", trace.ProofPlus.String())

	fmt.Printf("------------------- Trace 2: insert account 2 ------------------\n")

	key2, _ := types.AddressFromHex("0x2f00000000000000000000000000000000000000")

	account2 :=
		types.Account{
			Nonce:          41,
			Balance:        big.NewInt(15353),
			StorageRoot:    types.MustHexToKoalabearOctuplet("0x2fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df7"),
			LineaCodeHash:  types.MustHexToKoalabearOctuplet("0x000000000000000000000000000000000000000000000000000000000000004b"),
			KeccakCodeHash: types.FullBytes32FromHex("0x0f00000000000000000000000000000000000000000000000000000000000000"),
			CodeSize:       7,
		}

	encodedAccount2 := "0x0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002900000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003bf92fa0344a2fab2b310d2af3155c330261263f887379aef18b4941e3ea1cc59df7000000000000000000000000000000000000000000000000000000000000004b00000f0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007"

	trace2 := acc.InsertAndProve(key2, account2)
	require.Equal(t, int64(4), acc.NextFreeNode)

	require.Equal(
		t,
		"0x2f00000000000000000000000000000000000000",
		key2.Hex(),
	)

	buf2 := &bytes.Buffer{}
	_, err2 := account2.WriteTo(buf2)
	require.NoError(t, err2)
	val2 := utils.HexEncodeToString(buf2.Bytes())
	require.Equal(
		t,
		encodedAccount2,
		val2,
	)

	require.Equal(
		t,
		"0x75a23718103d010a0357dee12411e1a036a989fa05adb8e05e5fdd5472ad37c9",
		trace2.OldSubRoot.Hex(),
	)

	require.Equal(
		t,
		"0x301d34711d7c20426f77b0b566e6c70e55a7d18177d7eec93200fd4475fc4e02",
		trace2.NewSubRoot.Hex(),
	)

	fmt.Printf("Prev Proof: %v\n\n", trace2.ProofMinus.String())
	fmt.Printf("New Proof: %v\n\n", trace2.ProofNew.String())
	fmt.Printf("Next Proof: %v\n\n", trace2.ProofPlus.String())

	fmt.Printf("------------------- Trace 3: add a dummy account in another accumulator, with key2 as the location  ------------------\n")

	accdum := newTestAccumulatorPoseidon2DummyFullVal()
	{
		leafOpening := accdum.Data.MustGet(0).LeafOpening
		require.Equal(
			t,
			"LeafOpening{Prev: 0, Next: 1, HKey: 0x0000000000000000000000000000000000000000000000000000000000000000, HVal: 0x0000000000000000000000000000000000000000000000000000000000000000}",
			leafOpening.String(),
		)

	}

	{
		leafOpening := accdum.Data.MustGet(1).LeafOpening
		require.Equal(
			t,
			"LeafOpening{Prev: 0, Next: 1, HKey: 0x7f0000007f0000007f0000007f0000007f0000007f0000007f0000007f000000, HVal: 0x0000000000000000000000000000000000000000000000000000000000000000}",
			leafOpening.String(),
		)

	}

	key3 := types.FullBytes32FromHex("0x0e00000000000000000000000000000000000000000000000000000000000000")
	val3 := types.FullBytes32FromHex("0x1200000000000000000000000000000000000000000000000000000000000000")

	accdum.Location = key2.Hex()
	trace3 := accdum.InsertAndProve(key3, val3)

	require.Equal(
		t,
		"0x0e00000000000000000000000000000000000000000000000000000000000000",
		key3.Hex(),
	)

	require.Equal(
		t,
		"0x1200000000000000000000000000000000000000000000000000000000000000",
		val3.Hex(),
	)

	{
		leafOpening := accdum.Data.MustGet(2).LeafOpening
		require.Equal(
			t,
			"LeafOpening{Prev: 0, Next: 1, HKey: 0x3e59e46d51db286e504cbdf15ee428501786c73a574a96d06cc0f84b527c0954, HVal: 0x454eebb36976d82c78601ad67bd131797dd516b36b6b62b2457fcdd443662bc6}",
			leafOpening.String(),
		)

	}

	require.Equal(t, int64(3), accdum.NextFreeNode)

	require.Equal(
		t,
		"0x3a00a8e34a16f8a1225fee734816edb326f783bd6678d793345a28f046586ba6",
		trace3.OldSubRoot.Hex(),
	)

	require.Equal(
		t,
		"0x7a47390f07bccddb52ea3dd3746e54c14c79361d08cf35f35f293e7d5a85db92",
		trace3.NewSubRoot.Hex(),
	)

	fmt.Printf("Prev Proof: %v\n\n", trace3.ProofMinus.String())
	fmt.Printf("New Proof: %v\n\n", trace3.ProofNew.String())
	fmt.Printf("Next Proof: %v\n\n", trace3.ProofPlus.String())
	fmt.Printf("Location: %v\n\n", trace3.Location)

	fmt.Printf("------------------- Trace 4: update account 2 ------------------\n")

	account2.StorageRoot = types.MustHexToKoalabearOctuplet("0x07bd72a3216f334e18eb7cb3388a4ab4758d0b10486f08ae22e9834c7a0210d3")
	encodedNewAccount2 := "0x0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002900000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003bf907bd72a3216f334e18eb7cb3388a4ab4758d0b10486f08ae22e9834c7a0210d3000000000000000000000000000000000000000000000000000000000000004b00000f0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000007"

	trace4 := acc.UpdateAndProve(key2, account2)
	require.Equal(t, int64(4), acc.NextFreeNode)

	buf4 := &bytes.Buffer{}
	_, err4 := account2.WriteTo(buf4)
	require.NoError(t, err4)
	val4 := utils.HexEncodeToString(buf4.Bytes())
	require.Equal(
		t,
		encodedNewAccount2,
		val4,
	)

	require.Equal(
		t,
		"0x301d34711d7c20426f77b0b566e6c70e55a7d18177d7eec93200fd4475fc4e02",
		trace4.OldSubRoot.Hex(),
	)

	require.Equal(
		t,
		"0x748bb45221fecf5705112d7964563f0937b8fe3973079dbe3c3056274e512136",
		trace4.NewSubRoot.Hex(),
	)
	// check the value of the new leaf opening
	{
		leafOpening := acc.Data.MustGet(3).LeafOpening
		require.Equal(
			t,
			"LeafOpening{Prev: 0, Next: 2, HKey: 0x1b8d67463ebc5079420e3cd81eb80d3634377dea65f2372117425fb411526f7c, HVal: 0x5f5e38d86509c3451e2ffcc402d8e0d577ca6c60084c17923ed803f30d6ab1c6}",
			leafOpening.String(),
		)

		// also check its hash value
		leaf := leafOpening.Hash()
		require.Equal(t, "0x138908925d5ba2be424ecce93083d6eb69cf099a24f0722460397ab1584cc36f", leaf.Hex())
	}
	fmt.Printf("Proof: %v\n\n", trace4.Proof.String())

}
