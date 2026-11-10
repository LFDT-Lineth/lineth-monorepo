import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, beforeEach } from "node:test";
import { size, slice } from "viem";

import accountKeyTestVectors from "../_testData/account-key-test-vectors.json" with { type: "json" };
import merkleProofTestDataJson from "../_testData/merkle-proof-data-poseidon2.json" with { type: "json" };
import { deployFromFactory } from "../common/deployment.js";
import { address, hex, expectRevertWithCustomError } from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

type MerkleProofJson = { key: string; leafIndex: number; proof: { value: string; proofRelatedNodes: string[] } };

const toMerkleProof = ({ key, leafIndex, proof }: MerkleProofJson) => ({
  key: hex(key),
  leafIndex: BigInt(leafIndex),
  proof: { value: hex(proof.value), proofRelatedNodes: proof.proofRelatedNodes.map(hex) },
});

const merkleProofTestData = {
  accountProof: toMerkleProof(merkleProofTestDataJson.accountProof),
  storageProofs: merkleProofTestDataJson.storageProofs.map(toMerkleProof),
};

describe("SparseMerkleProof", () => {
  let sparseMerkleProof: ContractReturnType<"SparseMerkleProof">;

  const STATE_ROOT = "0x1aae3e0a143c0ac31b469af05096c1000a64836f05c250ad0857d0a1496f0c71";
  const STORAGE_ROOT = "0x07bd72a3216f334e18eb7cb3388a4ab4758d0b10486f08ae22e9834c7a0210d3";
  const ACCOUNT_KEY_HASH = "0x1b8d67463ebc5079420e3cd81eb80d3634377dea65f2372117425fb411526f7c";
  const ACCOUNT_VALUE_HASH = "0x5f5e38d86509c3451e2ffcc402d8e0d577ca6c60084c17923ed803f30d6ab1c6";
  const STORAGE_KEY_HASH = "0x3e59e46d51db286e504cbdf15ee428501786c73a574a96d06cc0f84b527c0954";
  const STORAGE_VALUE_HASH = "0x454eebb36976d82c78601ad67bd131797dd516b36b6b62b2457fcdd443662bc6";

  async function deploySparseMerkleProofFixture() {
    const poseidon2 = await deployFromFactory("Poseidon2");
    return deployFromFactory("SparseMerkleProof", [], { libraries: { Poseidon2: poseidon2.address } });
  }

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    sparseMerkleProof = await networkHelpers.loadFixture(deploySparseMerkleProofFixture);
  });

  describe("verifyProof", () => {
    describe("account proof", () => {
      it("Should revert when proof length is < 42", async () => {
        const {
          accountProof: {
            proof: { proofRelatedNodes },
          },
        } = merkleProofTestData;

        const leafIndex = 2n;

        const proofRelatedNodesPopped = proofRelatedNodes.slice(0, proofRelatedNodes.length - 1);

        await expectRevertWithCustomError(
          sparseMerkleProof,
          sparseMerkleProof.read.verifyProof([proofRelatedNodesPopped, leafIndex, STATE_ROOT]),
          "WrongProofLength",
          [42n, 41n],
        );
      });

      it("Should revert when proof length is > 42", async () => {
        const {
          accountProof: {
            proof: { proofRelatedNodes },
          },
        } = merkleProofTestData;

        const leafIndex = 200n;

        const clonedProof = proofRelatedNodes.slice(0, proofRelatedNodes.length);
        clonedProof.push(STATE_ROOT);

        await expectRevertWithCustomError(
          sparseMerkleProof,
          sparseMerkleProof.read.verifyProof([clonedProof, leafIndex, STATE_ROOT]),
          "WrongProofLength",
          [42n, 43n],
        );
      });

      it("Should revert when a value is not mod 32", async () => {
        const {
          accountProof: {
            proof: { proofRelatedNodes },
          },
        } = merkleProofTestData;

        const leafIndex = 200n;

        const clonedProof = proofRelatedNodes.slice(0, proofRelatedNodes.length);
        clonedProof[40] = "0x1234"; // set the second last item in the array

        await expectRevertWithCustomError(
          sparseMerkleProof,
          sparseMerkleProof.read.verifyProof([clonedProof, leafIndex, STATE_ROOT]),
          "LengthNotMod32",
        );
      });

      it("Should revert when index 0 is not 96 bytes", async () => {
        const {
          accountProof: {
            proof: { proofRelatedNodes },
          },
        } = merkleProofTestData;

        const leafIndex = 200n;

        const clonedProof = proofRelatedNodes.slice(0, proofRelatedNodes.length);
        clonedProof[0] = "0x1234";

        await expectRevertWithCustomError(
          sparseMerkleProof,
          sparseMerkleProof.read.verifyProof([clonedProof, leafIndex, STATE_ROOT]),
          "WrongBytesLength",
          [96n, 2n],
        );
      });

      it("Should revert when the leaf length is not 192 bytes", async () => {
        const {
          accountProof: {
            proof: { proofRelatedNodes },
          },
        } = merkleProofTestData;

        const leafIndex = 200n;

        const clonedProof = proofRelatedNodes.slice(0, proofRelatedNodes.length);
        clonedProof[clonedProof.length - 1] = "0x1234";

        await expectRevertWithCustomError(
          sparseMerkleProof,
          sparseMerkleProof.read.verifyProof([clonedProof, leafIndex, STATE_ROOT]),
          "WrongBytesLength",
          [192n, 2n],
        );
      });

      it("Should revert when the computedHash != subSmtRoot", async () => {
        const {
          accountProof: {
            proof: { proofRelatedNodes },
          },
        } = merkleProofTestData;

        const leafIndex = 200n;
        await expectRevertWithCustomError(
          sparseMerkleProof,
          sparseMerkleProof.read.verifyProof([proofRelatedNodes, leafIndex, STATE_ROOT]),
          "SubSmtRootMismatch",
          [slice(proofRelatedNodes[0], 64), "0x64e2a3801735b62727c6dd645f2167bd2f2876aa77f90cf71c45321019ab1440"],
        );
      });

      it("Should return false when the account proof is not correct", async () => {
        const {
          accountProof: {
            proof: { proofRelatedNodes },
            leafIndex,
          },
        } = merkleProofTestData;

        const result = await sparseMerkleProof.read.verifyProof([
          proofRelatedNodes,
          leafIndex,
          hex(`0x00${STATE_ROOT.slice(4)}`),
        ]);

        expect(result).to.be.false;
      });

      it("Should revert when leaf index is higher than max leaf index", async () => {
        const {
          accountProof: {
            proof: { proofRelatedNodes },
            leafIndex,
          },
        } = merkleProofTestData;

        const higherLeafIndex = leafIndex + 2n ** 40n;

        await expectRevertWithCustomError(
          sparseMerkleProof,
          sparseMerkleProof.read.verifyProof([proofRelatedNodes, higherLeafIndex, STATE_ROOT]),
          "MaxTreeLeafIndexExceed",
        );
      });

      it("Should return true when the account proof is correct", async () => {
        const {
          accountProof: {
            proof: { proofRelatedNodes },
            leafIndex,
          },
        } = merkleProofTestData;

        const result = await sparseMerkleProof.read.verifyProof([proofRelatedNodes, leafIndex, STATE_ROOT]);

        expect(result).to.be.true;
      });
    });

    describe("storage proof", () => {
      it("Should revert when the computedHash != subSmtRoot", async () => {
        const {
          storageProofs: [
            {
              proof: { proofRelatedNodes },
            },
          ],
        } = merkleProofTestData;

        const leafIndex = 200n;
        await expectRevertWithCustomError(
          sparseMerkleProof,
          sparseMerkleProof.read.verifyProof([proofRelatedNodes, leafIndex, STORAGE_ROOT]),
          "SubSmtRootMismatch",
          [slice(proofRelatedNodes[0], 64), "0x2fd8ac0b6f2a69964661ea2c27f2b1ad320d5aea49a3451735e55037672b1dd7"],
        );
      });

      it("Should return false when the storage proof is not correct", async () => {
        const {
          storageProofs: [
            {
              proof: { proofRelatedNodes },
              leafIndex,
            },
          ],
        } = merkleProofTestData;

        const result = await sparseMerkleProof.read.verifyProof([
          proofRelatedNodes,
          leafIndex,
          hex(`0x00${STORAGE_ROOT.slice(4)}`),
        ]);

        expect(result).to.be.false;
      });

      it("Should return true when the storage proof is correct", { timeout: 30_000 }, async () => {
        const { storageProofs } = merkleProofTestData;

        for (const {
          proof: { proofRelatedNodes },
          leafIndex,
        } of storageProofs) {
          const result = await sparseMerkleProof.read.verifyProof([proofRelatedNodes, leafIndex, STORAGE_ROOT]);
          expect(result).to.be.true;
        }
      });
    });
  });

  describe("hashAccountValue", () => {
    it("Should return value hash", async () => {
      const {
        accountProof: {
          proof: { value },
        },
      } = merkleProofTestData;

      const hVal = await sparseMerkleProof.read.hashAccountValue([value]);
      expect(hVal).to.be.equal(ACCOUNT_VALUE_HASH);
    });

    it("Should error if less than 192 length", async () => {
      const shortValue = "0x0012";

      await expectRevertWithCustomError(
        sparseMerkleProof,
        sparseMerkleProof.read.hashAccountValue([shortValue]),
        "WrongBytesLength",
        [192n, 2n],
      );
    });

    it("Should error if more than 192 length", async () => {
      const longValue =
        "0x000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000d2a66d5598b4fc5482c311f22d2dc657579b5452ab4b3e60fb1a9e9dbbfc99e00c24dd0468f02fbece668291f3c3eb20e06d1baec856f28430555967f2bf280d798c662debc23e8199fbf0b0a3a95649f2defe90af458d7f62c03881f916b3f0000000000000000000000000000000000000000000000000000000000003030454748";

      await expectRevertWithCustomError(
        sparseMerkleProof,
        sparseMerkleProof.read.hashAccountValue([longValue]),
        "WrongBytesLength",
        [192n, 195n],
      );
    });
  });

  describe("hashStorageValue", () => {
    it("Should return value hash", async () => {
      const {
        storageProofs: [
          {
            proof: { value },
          },
        ],
      } = merkleProofTestData;
      const hVal = await sparseMerkleProof.read.hashStorageValue([value]);
      expect(hVal).to.be.equal(STORAGE_VALUE_HASH);
    });
  });

  describe("hashAccountKey", () => {
    it("Should return account key hash", async () => {
      const {
        accountProof: { key },
      } = merkleProofTestData;

      const hKey = await sparseMerkleProof.read.hashAccountKey([key]);
      expect(hKey).to.be.equal(ACCOUNT_KEY_HASH);
    });

    it("Should return account key hash from test vectors", async () => {
      for (const { input, output } of accountKeyTestVectors) {
        const hKey = await sparseMerkleProof.read.hashAccountKey([address(input)]);
        expect(hKey).to.be.equal(output);
      }
    });
  });

  describe("getLeaf", () => {
    describe("account leaf", () => {
      it("Should revert when leaf bytes length < 192", async () => {
        const {
          accountProof: {
            proof: { proofRelatedNodes },
          },
        } = merkleProofTestData;

        const wrongLeaftValue = hex(`0x${proofRelatedNodes[proofRelatedNodes.length - 1].slice(4)}`);

        await expectRevertWithCustomError(
          sparseMerkleProof,
          sparseMerkleProof.read.getLeaf([wrongLeaftValue]),
          "WrongBytesLength",
          [192n, BigInt(size(wrongLeaftValue))],
        );
      });

      it("Should revert when leaf bytes length > 192", async () => {
        const {
          accountProof: {
            proof: { proofRelatedNodes },
          },
        } = merkleProofTestData;

        const wrongLeaftValue = hex(`${proofRelatedNodes[proofRelatedNodes.length - 1]}1234`);

        await expectRevertWithCustomError(
          sparseMerkleProof,
          sparseMerkleProof.read.getLeaf([wrongLeaftValue]),
          "WrongBytesLength",
          [192n, BigInt(size(wrongLeaftValue))],
        );
      });

      it("Should return parsed leaf", async () => {
        const {
          accountProof: {
            key,
            proof: { value, proofRelatedNodes },
          },
        } = merkleProofTestData;

        const leaf = await sparseMerkleProof.read.getLeaf([proofRelatedNodes[proofRelatedNodes.length - 1]]);
        const expectedHKey = await sparseMerkleProof.read.hashAccountKey([key]);
        const expectedHValue = await sparseMerkleProof.read.hashAccountValue([value]);

        expect(leaf.prev[0]).to.be.equal(0n);
        expect(leaf.prev[1]).to.be.equal(0n);
        expect(leaf.next[0]).to.be.equal(0n);
        expect(leaf.next[1]).to.be.equal(2n);
        expect(leaf.hKey).to.be.equal(expectedHKey);
        expect(leaf.hValue).to.be.equal(expectedHValue);
        expect(leaf.hKey).to.be.equal(ACCOUNT_KEY_HASH);
        expect(leaf.hValue).to.be.equal(ACCOUNT_VALUE_HASH);
      });
    });

    describe("storage leaf", () => {
      it("Should revert when leaf bytes length < 192", async () => {
        const {
          storageProofs: [
            {
              proof: { proofRelatedNodes },
            },
          ],
        } = merkleProofTestData;

        const wrongLeaftValue = hex(`0x${proofRelatedNodes[proofRelatedNodes.length - 1].slice(4)}`);

        await expectRevertWithCustomError(
          sparseMerkleProof,
          sparseMerkleProof.read.getLeaf([wrongLeaftValue]),
          "WrongBytesLength",
          [192n, BigInt(size(wrongLeaftValue))],
        );
      });

      it("Should return parsed leaf", async () => {
        const {
          storageProofs: [
            {
              key,
              proof: { value, proofRelatedNodes },
            },
          ],
        } = merkleProofTestData;

        const leaf = await sparseMerkleProof.read.getLeaf([proofRelatedNodes[proofRelatedNodes.length - 1]]);
        const expectedHKey = await sparseMerkleProof.read.hashStorageValue([key]);
        const expectedHValue = await sparseMerkleProof.read.hashStorageValue([value]);

        expect(leaf.prev[0]).to.be.equal(0n);
        expect(leaf.prev[1]).to.be.equal(0n);
        expect(leaf.next[0]).to.be.equal(0n);
        expect(leaf.next[1]).to.be.equal(1n);
        expect(leaf.hKey).to.be.equal(expectedHKey);
        expect(leaf.hValue).to.be.equal(expectedHValue);
        expect(leaf.hKey).to.be.equal(STORAGE_KEY_HASH);
        expect(leaf.hValue).to.be.equal(STORAGE_VALUE_HASH);
      });
    });
  });

  describe("getAccount", () => {
    it("Should revert when account bytes length < 192", async () => {
      const {
        accountProof: {
          proof: { value },
        },
      } = merkleProofTestData;

      const wrongAccountValue = hex(`0x${value.slice(4)}`);

      await expectRevertWithCustomError(
        sparseMerkleProof,
        sparseMerkleProof.read.getAccount([wrongAccountValue]),
        "WrongBytesLength",
        [192n, BigInt(size(wrongAccountValue))],
      );
    });

    it("Should revert when account bytes length > 192", async () => {
      const {
        accountProof: {
          proof: { value },
        },
      } = merkleProofTestData;

      const wrongAccountValue = hex(`${value}123456`);

      await expectRevertWithCustomError(
        sparseMerkleProof,
        sparseMerkleProof.read.getAccount([wrongAccountValue]),
        "WrongBytesLength",
        [192n, BigInt(size(wrongAccountValue))],
      );
    });

    it("Should return parsed account", async () => {
      const {
        accountProof: {
          proof: { value },
        },
      } = merkleProofTestData;

      const account = await sparseMerkleProof.read.getAccount([value]);

      expect(account.nonce).to.be.equal(41n);
      expect(account.balance).to.be.equal(15353n);
      expect(account.storageRoot).to.be.equal(STORAGE_ROOT);
      expect(account.snarkCodeHash).to.be.equal("0x000000000000000000000000000000000000000000000000000000000000004b");
      expect(account.keccakCodeHash).to.be.equal("0x0f00000000000000000000000000000000000000000000000000000000000000");
      expect(account.codeSize).to.be.equal(7n);
    });
  });

  describe("poseidon2Hash", () => {
    it("Should return poseidon2 hash", async () => {
      const {
        accountProof: {
          proof: { value },
        },
      } = merkleProofTestData;

      const hashedValue = await sparseMerkleProof.read.poseidon2Hash([value]);

      expect(hashedValue).to.be.equal("0x12275b306674a6b26e40a01c7c337a825058bd87160698ef3f0270bf2abbda20");
    });
  });
});
