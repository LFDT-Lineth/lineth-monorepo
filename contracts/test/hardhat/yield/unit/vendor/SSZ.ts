import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { TestGIndex, TestSSZ } from "contracts/typechain-types";
import { deployFromFactory } from "../../../common/deployment";
import { hexlify, randomBytes, zeroPadBytes, ZeroHash, sha256, concat, getBytes } from "ethers";
import { BeaconBlockHeader, PendingPartialWithdrawal, ValidatorContainer } from "../../../yield/helpers/types";
import { expectRevertWithCustomError } from "../../../common/helpers";
import { UINT64_MAX } from "../../../common/constants/general";

describe("SSZ", () => {
  let ssz: TestSSZ;
  let gIndexLib: TestGIndex;

  beforeEach(async () => {
    async function deploy() {
      const [ssz_, gindex_] = await Promise.all([deployFromFactory("TestSSZ"), deployFromFactory("TestGIndex")]);
      return { ssz_: ssz_ as TestSSZ, gindex_: gindex_ as TestGIndex };
    }

    const deployed = await loadFixture(deploy);
    ssz = deployed.ssz_;
    gIndexLib = deployed.gindex_;
  });

  describe("toLittleEndian", () => {
    it("uint: example value", async () => {
      const v = 0x1234567890abcdefn;
      const expected = zeroPadBytes("0xEFCDAB9078563412", 32);
      const actual = await ssz.toLittleEndianUint(v);
      expect(actual).to.equal(expected);
    });

    it("uint: zero", async () => {
      const expected = ZeroHash;
      const actual = await ssz.toLittleEndianUint(0n);
      expect(actual).to.equal(expected);
    });

    it("bool: false", async () => {
      const expected = ZeroHash;
      const actual = await ssz.toLittleEndianBool(false);
      expect(actual).to.equal(expected);
    });

    it("bool: true", async () => {
      const expected = zeroPadBytes("0x01", 32);
      const actual = await ssz.toLittleEndianBool(true);
      expect(actual).to.equal(expected);
    });

    it("fuzz-ish: applying twice returns original", async () => {
      for (let i = 0; i < 10; i++) {
        const v = BigInt(hexlify(randomBytes(32)));
        const once = await ssz.toLittleEndianUint(v);
        const twice = await ssz.toLittleEndianUint(BigInt(once));
        expect(BigInt(twice)).to.equal(v);
      }
    });
  });

  describe("hashTreeRoot(Validator)", () => {
    it("Exited + Slashed", async () => {
      const validator: ValidatorContainer = {
        pubkey: "0x91760f8a17729cfcb68bfc621438e5d9dfa831cd648e7b2b7d33540a7cbfda1257e4405e67cd8d3260351ab3ff71b213",
        withdrawalCredentials: "0x01000000000000000000000006676e8584342cc8b6052cfdf381c3a281f00ac8",
        effectiveBalance: 30000000000n,
        slashed: true,
        activationEligibilityEpoch: 242529n,
        activationEpoch: 242551n,
        exitEpoch: 242556n,
        withdrawableEpoch: 250743n,
      };

      const expected = "0xe4674dc5c27e7d3049fcd298745c00d3e314f03d33c877f64bf071d3b77eb942";
      const actual = await ssz.hashTreeRoot_Validator(validator);
      expect(actual).to.equal(expected);
    });

    it("Active", async () => {
      const validator: ValidatorContainer = {
        pubkey: "0x8fb78536e82bcec34e98fff85c907f0a8e6f4b1ccdbf1e8ace26b59eb5a06d16f34e50837f6c490e2ad6a255db8d543b",
        withdrawalCredentials: "0x0023b9d00bf66e7f8071208a85afde59b3148dea046ee3db5d79244880734881",
        effectiveBalance: 32000000000n,
        slashed: false,
        activationEligibilityEpoch: 2593n,
        activationEpoch: 5890n,
        exitEpoch: BigInt("0xffffffffffffffff"),
        withdrawableEpoch: BigInt("0xffffffffffffffff"),
      };

      const actual = await ssz.hashTreeRoot_Validator(validator);
      const expected = "0x60fb91184416404ddfc62bef6df9e9a52c910751daddd47ea426aabaf19dfa09";
      expect(actual).to.equal(expected);
    });

    it("Extra bytes in pubkey are ignored", async () => {
      const validator: ValidatorContainer = {
        pubkey:
          "0x8fb78536e82bcec34e98fff85c907f0a8e6f4b1ccdbf1e8ace26b59eb5a06d16f34e50837f6c490e2ad6a255db8d543bDEADBEEFDEADBEEFDEADBEEFDEADBEEF",
        withdrawalCredentials: "0x0023b9d00bf66e7f8071208a85afde59b3148dea046ee3db5d79244880734881",
        effectiveBalance: 32000000000n,
        slashed: false,
        activationEligibilityEpoch: 2593n,
        activationEpoch: 5890n,
        exitEpoch: BigInt("0xffffffffffffffff"),
        withdrawableEpoch: BigInt("0xffffffffffffffff"),
      };

      const actual = await ssz.hashTreeRoot_Validator(validator);
      const expected = "0x60fb91184416404ddfc62bef6df9e9a52c910751daddd47ea426aabaf19dfa09";
      expect(actual).to.equal(expected);
    });

    it("All zeroes", async () => {
      const validator: ValidatorContainer = {
        pubkey: "0x" + "0".repeat(96),
        withdrawalCredentials: ZeroHash,
        effectiveBalance: 0n,
        slashed: false,
        activationEligibilityEpoch: 0n,
        activationEpoch: 0n,
        exitEpoch: 0n,
        withdrawableEpoch: 0n,
      };

      const actual = await ssz.hashTreeRoot_Validator(validator);
      const expected = "0xfa324a462bcb0f10c24c9e17c326a4e0ebad204feced523eccaf346c686f06ee";
      expect(actual).to.equal(expected);
    });

    it("All ones", async () => {
      const validator: ValidatorContainer = {
        pubkey: "0x" + "f".repeat(96),
        withdrawalCredentials: "0x" + "f".repeat(64),
        effectiveBalance: BigInt("0xffffffffffffffff"),
        slashed: true,
        activationEligibilityEpoch: BigInt("0xffffffffffffffff"),
        activationEpoch: BigInt("0xffffffffffffffff"),
        exitEpoch: BigInt("0xffffffffffffffff"),
        withdrawableEpoch: BigInt("0xffffffffffffffff"),
      };

      const actual = await ssz.hashTreeRoot_Validator(validator);
      const expected = "0x29c03a7cc9a8047ff05619a04bb6e60440a791e6ac3fe7d72e6fe9037dd3696f";
      expect(actual).to.equal(expected);
    });
  });

  describe("hashTreeRoot(BeaconBlockHeader)", () => {
    it("mainnet header example", async () => {
      const header: BeaconBlockHeader = {
        slot: 7472518,
        proposerIndex: 152834,
        parentRoot: "0x4916af1ff31b06f1b27125d2d20cd26e123c425a4b34ebd414e5f0120537e78d",
        stateRoot: "0x76ca64f3732754bc02c7966271fb6356a9464fe5fce85be8e7abc403c8c7b56b",
        bodyRoot: "0x6d858c959f1c95f411dba526c4ae9ab8b2690f8b1e59ed1b79ad963ab798b01a",
      };

      const expected = "0x26631ee28ab4dd44a39c3756e03714d6a35a256560de5e2885caef9c3efd5516";
      const actual = await ssz.hashTreeRoot_BeaconBlockHeader(header);
      expect(actual).to.equal(expected);
    });

    it("all zeroes", async () => {
      const header: BeaconBlockHeader = {
        slot: 0,
        proposerIndex: 0,
        parentRoot: ZeroHash,
        stateRoot: ZeroHash,
        bodyRoot: ZeroHash,
      };

      const expected = "0xc78009fdf07fc56a11f122370658a353aaa542ed63e44c4bc15ff4cd105ab33c";
      const actual = await ssz.hashTreeRoot_BeaconBlockHeader(header);
      expect(actual).to.equal(expected);
    });

    it("all ones", async () => {
      const header: BeaconBlockHeader = {
        slot: UINT64_MAX,
        proposerIndex: UINT64_MAX,
        parentRoot: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
        stateRoot: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
        bodyRoot: "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
      };

      const expected = "0x5ebe9f2b0267944bd80dd5cde20317a91d07225ff12e9cd5ba1e834c05cc2b05";
      const actual = await ssz.hashTreeRoot_BeaconBlockHeader(header);
      expect(actual).to.equal(expected);
    });
  });

  // Test cases - https://github.com/ethereum/consensus-spec-tests/tree/master/tests/mainnet/electra/ssz_static/PendingPartialWithdrawal/ssz_random
  describe("hashTreeRoot(PendingPartialWithdrawal)", () => {
    it("example", async () => {
      const pendingPartialWithdrawal: PendingPartialWithdrawal = {
        validatorIndex: 0,
        amount: 1,
        withdrawableEpoch: 2,
      };

      const expected = "0x4a07d56213d62b2d194a3cc1f19bec40364540bdf3d45eb0d6fe82094d21b4dc";
      const actual = await ssz.hashTreeRoot_PendingPartialWithdrawal(pendingPartialWithdrawal);
      expect(actual).to.equal(expected);
    });

    it("example 2", async () => {
      const pendingPartialWithdrawal: PendingPartialWithdrawal = {
        validatorIndex: 0,
        amount: 1,
        withdrawableEpoch: 0,
      };

      const expected = "0x4833912e1264aef8a18392d795f3f2eed17cf5c0e8471cb0c0db2ec5aca10231";
      const actual = await ssz.hashTreeRoot_PendingPartialWithdrawal(pendingPartialWithdrawal);
      expect(actual).to.equal(expected);
    });

    it("all zeroes", async () => {
      const pendingPartialWithdrawal: PendingPartialWithdrawal = {
        validatorIndex: 0,
        amount: 0,
        withdrawableEpoch: 0,
      };

      const expected = "0xdb56114e00fdd4c1f85c892bf35ac9a89289aaecb1ebd0a96cde606a748b5d71";
      const actual = await ssz.hashTreeRoot_PendingPartialWithdrawal(pendingPartialWithdrawal);
      expect(actual).to.equal(expected);
    });

    it("example 3", async () => {
      const pendingPartialWithdrawal: PendingPartialWithdrawal = {
        validatorIndex: 9556824998668043785n,
        amount: 18095667167504007302n,
        withdrawableEpoch: 12065041970590563750n,
      };

      const expected = "0xfee5527172fd2af098adcdfa5d4108ffc52d19b4cb03fcdb186685a11147fe7b";
      const actual = await ssz.hashTreeRoot_PendingPartialWithdrawal(pendingPartialWithdrawal);
      expect(actual).to.equal(expected);
    });
  });

  // Obtained expected hashes from https://github.com/kyzooghost/consensus-specs/blob/7a27a0ecb17255b04618f96220bcda3de88bed28/tests/core/pyspec/eth2spec/utils/test_merkle_minimal.py#L127-L186
  // Run `make test k=test_merkleize_chunks_with_mix_in_length` from the project root
  describe("hashTreeRoot(PendingPartialWithdrawal[])", () => {
    it("empty array", async () => {
      const pendingPartialWithdrawals: PendingPartialWithdrawal[] = [];

      const expected = "0xfcada1ce97f6629a9b31bd46dc9824a4ee18e91bb76243e16387616176e1d899";
      const actual = await ssz.hashTreeRoot_PendingPartialWithdrawalArray(pendingPartialWithdrawals);
      expect(actual).to.equal(expected);
    });

    it("single element", async () => {
      const pendingPartialWithdrawals: PendingPartialWithdrawal[] = [
        {
          validatorIndex: 0,
          amount: 1,
          withdrawableEpoch: 2,
        },
      ];

      const expected = "0x4cf2a5b8d91e782f8f2b060a7cbf904d2ae73e063cd032a433b396aeb69be647";
      const actual = await ssz.hashTreeRoot_PendingPartialWithdrawalArray(pendingPartialWithdrawals);
      expect(actual).to.equal(expected);
    });

    it("two elements", async () => {
      const pendingPartialWithdrawals: PendingPartialWithdrawal[] = [
        // 0x4a07d56213d62b2d194a3cc1f19bec40364540bdf3d45eb0d6fe82094d21b4dc
        {
          validatorIndex: 0,
          amount: 1,
          withdrawableEpoch: 2,
        },
        // 0x4833912e1264aef8a18392d795f3f2eed17cf5c0e8471cb0c0db2ec5aca10231
        {
          validatorIndex: 0,
          amount: 1,
          withdrawableEpoch: 0,
        },
      ];

      const expected = "0xef5ec7811106d9b2f4323d7e730d3c2dbd1ac3a94082e55cf407dad718b7cf18";
      const actual = await ssz.hashTreeRoot_PendingPartialWithdrawalArray(pendingPartialWithdrawals);
      expect(actual).to.equal(expected);
    });

    it("three elements", async () => {
      const pendingPartialWithdrawals: PendingPartialWithdrawal[] = [
        // 0xdb56114e00fdd4c1f85c892bf35ac9a89289aaecb1ebd0a96cde606a748b5d71
        {
          validatorIndex: 0,
          amount: 0,
          withdrawableEpoch: 0,
        },
        {
          validatorIndex: 0,
          amount: 0,
          withdrawableEpoch: 0,
        },
        {
          validatorIndex: 0,
          amount: 0,
          withdrawableEpoch: 0,
        },
      ];

      const expected = "0x46c3a97159201f02892f56dd655dea88e234bdb63b5e3ae33641b1fd767c806e";
      const actual = await ssz.hashTreeRoot_PendingPartialWithdrawalArray(pendingPartialWithdrawals);
      expect(actual).to.equal(expected);
    });

    it("four elements", async () => {
      const pendingPartialWithdrawals: PendingPartialWithdrawal[] = [
        // 0xfee5527172fd2af098adcdfa5d4108ffc52d19b4cb03fcdb186685a11147fe7b
        {
          validatorIndex: 9556824998668043785n,
          amount: 18095667167504007302n,
          withdrawableEpoch: 12065041970590563750n,
        },
        // 0xd4c8a4e38ed4d4d09d3b74df1d825d244243218fa2ce1878eeb3d0356ec7fcab
        {
          validatorIndex: 18198258603828382500n,
          amount: 4349232369502288358n,
          withdrawableEpoch: 3598560756448475534n,
        },
        // 0x3548a86db6940952e5ab87b50e46cfbdb2324603ccfac73836834a87f160181a
        {
          validatorIndex: 12778732824589014348n,
          amount: 4849311627484200036n,
          withdrawableEpoch: 457195784761064180n,
        },
        // 0x8ceb740b26a61041ea7dc2d6b1372686cf3381150bd9d9a19cfafeb9e0335c04
        {
          validatorIndex: 350840880130630803n,
          amount: 8902480238376794760n,
          withdrawableEpoch: 3145816884024322139n,
        },
      ];

      const expected = "0xd47528eb7a0c6e3edf7d69c824d90a7e583455667e05f7e9c905864b5a332c06";
      const actual = await ssz.hashTreeRoot_PendingPartialWithdrawalArray(pendingPartialWithdrawals);
      expect(actual).to.equal(expected);
    });

    it("five elements", async () => {
      const pendingPartialWithdrawals: PendingPartialWithdrawal[] = [
        // 0xfee5527172fd2af098adcdfa5d4108ffc52d19b4cb03fcdb186685a11147fe7b
        {
          validatorIndex: 9556824998668043785n,
          amount: 18095667167504007302n,
          withdrawableEpoch: 12065041970590563750n,
        },
        // 0xd4c8a4e38ed4d4d09d3b74df1d825d244243218fa2ce1878eeb3d0356ec7fcab
        {
          validatorIndex: 18198258603828382500n,
          amount: 4349232369502288358n,
          withdrawableEpoch: 3598560756448475534n,
        },
        // 0x3548a86db6940952e5ab87b50e46cfbdb2324603ccfac73836834a87f160181a
        {
          validatorIndex: 12778732824589014348n,
          amount: 4849311627484200036n,
          withdrawableEpoch: 457195784761064180n,
        },
        // 0x8ceb740b26a61041ea7dc2d6b1372686cf3381150bd9d9a19cfafeb9e0335c04
        {
          validatorIndex: 350840880130630803n,
          amount: 8902480238376794760n,
          withdrawableEpoch: 3145816884024322139n,
        },
        // 0xde460e5b596f11e6791d4b658544351a44e8bfc86b0952b8ac655354b399adad
        {
          validatorIndex: 17694784621958833581n,
          amount: 4314273187093803793n,
          withdrawableEpoch: 8404893953447176506n,
        },
      ];

      const expected = "0xb503de61a6faaee49ba30ce6fc2216c7306aa0e46611aeacb5f29f2eacd53d0f";
      const actual = await ssz.hashTreeRoot_PendingPartialWithdrawalArray(pendingPartialWithdrawals);
      expect(actual).to.equal(expected);
    });

    // Note: Testing with exactly 2^27 or 2^27 + 1 elements is impractical due to JavaScript
    // array size limitations (RangeError: Invalid array length). The bounds check
    // `if (count > (1 << MAX_PENDING_PARTIAL_WITHDRAWAL_DEPTH))` is verified through:
    // 1. Code review - the check is straightforward and correctly placed
    // 2. Existing tests verify the function works correctly for normal-sized arrays
    // 3. The check happens before any processing, so invalid inputs fail fast
    // In practice, arrays larger than 2^27 would be rejected by the bounds check,
    // preventing out-of-bounds writes in mergeSSZChunk.
  });

  describe("verifyProof", () => {
    // For the tests below, assume there's the following tree from the bottom up:
    // --
    // 0x0000000000000000000000000000000000000000000000000000000000000000
    // 0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5
    // 0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30
    // 0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85
    // --
    // 0x0a4b105f69a6f41c3b3efc9bb5ac525b5b557a524039a13c657a916d8eb04451
    // 0xf4551dd23f47858f0e66957db62a0bced8cfd5e9cbd63f2fd73672ed0db7c124
    // --
    // 0xda1c902c54a4386439ce622d7e527dc11decace28ebb902379cba91c4a116b1c

    it("happy path", async () => {
      const proof = [
        "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
        "0xf4551dd23f47858f0e66957db62a0bced8cfd5e9cbd63f2fd73672ed0db7c124",
      ];
      await ssz.verifyProof(
        proof,
        "0xda1c902c54a4386439ce622d7e527dc11decace28ebb902379cba91c4a116b1c",
        ZeroHash,
        await gIndexLib.pack(4, 0),
      );

      const proof2 = [ZeroHash, "0xf4551dd23f47858f0e66957db62a0bced8cfd5e9cbd63f2fd73672ed0db7c124"];
      await ssz.verifyProof(
        proof2,
        "0xda1c902c54a4386439ce622d7e527dc11decace28ebb902379cba91c4a116b1c",
        "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
        await gIndexLib.pack(5, 0),
      );
    });
    it("one item", async () => {
      const proof = ["0xf4551dd23f47858f0e66957db62a0bced8cfd5e9cbd63f2fd73672ed0db7c124"];
      await ssz.verifyProof(
        proof,
        "0xda1c902c54a4386439ce622d7e527dc11decace28ebb902379cba91c4a116b1c",
        "0x0a4b105f69a6f41c3b3efc9bb5ac525b5b557a524039a13c657a916d8eb04451",
        await gIndexLib.pack(2, 0),
      );
    });
    it("revert: no proof", async () => {
      const call = ssz.verifyProof(
        [],
        "0xf5a5fd42d16a20302798ef6ed309979b43003d2320d9f0e8ea9831a92759fb4b",
        ZeroHash,
        await gIndexLib.pack(2, 0),
      );

      await expectRevertWithCustomError(ssz, call, "InvalidProof");
    });
    it("revert: proving root", async () => {
      await expect(
        ssz.verifyProof(
          [],
          "0xf5a5fd42d16a20302798ef6ed309979b43003d2320d9f0e8ea9831a92759fb4b",
          "0xf5a5fd42d16a20302798ef6ed309979b43003d2320d9f0e8ea9831a92759fb4b",
          await gIndexLib.pack(1, 0),
        ),
      ).to.be.revertedWithCustomError(ssz, "InvalidProof");
    });
    it("revert: invalid proof", async () => {
      const proof = [
        "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
        "0xf4551dd23f47858f0e66957db62a0bced8cfd5e9cbd63f2fd73672ed0db7c124",
      ];
      await expect(
        ssz.verifyProof(
          proof,
          "0xda1c902c54a4386439ce622d7e527dc11decace28ebb902379cba91c4a116b1c",
          "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
          await gIndexLib.pack(4, 0),
        ),
      ).to.be.revertedWithCustomError(ssz, "InvalidProof");
    });
    it("revert: wrong gindex", async () => {
      const proof = [
        "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
        "0xf4551dd23f47858f0e66957db62a0bced8cfd5e9cbd63f2fd73672ed0db7c124",
      ];
      await expect(
        ssz.verifyProof(
          proof,
          "0xda1c902c54a4386439ce622d7e527dc11decace28ebb902379cba91c4a116b1c",
          ZeroHash,
          await gIndexLib.pack(5, 0),
        ),
      ).to.be.revertedWithCustomError(ssz, "InvalidProof");
    });
    it("revert: BranchHasExtraItem", async () => {
      const proof = [
        "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
        "0xf4551dd23f47858f0e66957db62a0bced8cfd5e9cbd63f2fd73672ed0db7c124",
      ];
      await expect(
        ssz.verifyProof(
          proof,
          "0xda1c902c54a4386439ce622d7e527dc11decace28ebb902379cba91c4a116b1c",
          "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
          await gIndexLib.pack(2, 0),
        ),
      ).to.be.revertedWithCustomError(ssz, "BranchHasExtraItem");
    });
    it("revert: BranchHasMissingItem", async () => {
      const proof = [
        "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
        "0xf4551dd23f47858f0e66957db62a0bced8cfd5e9cbd63f2fd73672ed0db7c124",
      ];
      await expect(
        ssz.verifyProof(
          proof,
          "0xda1c902c54a4386439ce622d7e527dc11decace28ebb902379cba91c4a116b1c",
          "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
          await gIndexLib.pack(8, 0),
        ),
      ).to.be.revertedWithCustomError(ssz, "BranchHasMissingItem");
    });
  });

  describe("sha256Pair", () => {
    it("zeros + zeros", async () => {
      const left = ZeroHash;
      const right = ZeroHash;

      const expected = sha256(concat([getBytes(left), getBytes(right)]));
      const actual = await ssz.sha256Pair(left, right);
      expect(actual).to.equal(expected);
    });

    it("distinct inputs", async () => {
      const left = "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5";
      const right = "0xf4551dd23f47858f0e66957db62a0bced8cfd5e9cbd63f2fd73672ed0db7c124";

      const expected = sha256(concat([getBytes(left), getBytes(right)]));
      const actual = await ssz.sha256Pair(left, right);
      expect(actual).to.equal(expected);
    });
  });
});
