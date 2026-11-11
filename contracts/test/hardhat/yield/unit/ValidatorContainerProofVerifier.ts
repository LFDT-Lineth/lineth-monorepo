import { expect } from "chai";
import hre, { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { type Hex, zeroAddress } from "viem";

import { DEFAULT_ADMIN_ROLE } from "../../../../common/constants/index.js";
import { randomBytes32 } from "../../../../common/helpers/encoding.js";
import {
  GI_FIRST_VALIDATOR,
  GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  HASH_ZERO,
  ONE_GWEI,
  SHARD_COMMITTEE_PERIOD,
  SLOTS_PER_EPOCH,
} from "../../common/constants/index.js";
import { deployFromFactory } from "../../common/deployment.js";
import {
  type TestAccount,
  buildAccessErrorMessage,
  getAccountsFixture,
  expectZeroAddressRevert,
  expectZeroHashRevert,
  expectEvent,
  expectRevertWithReason,
  expectRevertWithCustomError,
} from "../../common/helpers/index.js";
import {
  deployTestValidatorContainerProofVerifier,
  randomAddress as generateRandomAddress,
  type ValidatorContainerWitness,
} from "../helpers/index.js";
import {
  ACTIVE_0X01_VALIDATOR_PROOF,
  generateBeaconHeader,
  generateEIP4478Witness,
  generateValidatorContainer,
  prepareLocalMerkleTree,
  randomInt,
  setBeaconBlockRoot,
} from "../helpers/proof.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("ValidatorContainerProofVerifier", () => {
  let verifier: ContractReturnType<"TestValidatorContainerProofVerifier">;
  let sszMerkleTree: ContractReturnType<"SSZMerkleTree">;
  let firstValidatorLeafIndex: bigint;
  let lastValidatorIndex: bigint;
  let localTreeGiFirstValidator: Hex;
  let admin: TestAccount;
  let nonAdmin: TestAccount;

  before(async () => {
    const localTree = await prepareLocalMerkleTree();
    localTreeGiFirstValidator = localTree.gIFirstValidator;
    sszMerkleTree = localTree.sszMerkleTree;
    firstValidatorLeafIndex = localTree.firstValidatorLeafIndex;
    // populate merkle tree with validators
    for (let i = 1; i < 10; i++) {
      await sszMerkleTree.write.addValidatorLeaf([generateValidatorContainer()]);
    }
    // after adding validators, all newly added validator indexes will +n from this
    lastValidatorIndex = (await sszMerkleTree.read.leafCount()) - 1n - firstValidatorLeafIndex;
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    verifier = await networkHelpers.loadFixture(deployTestValidatorContainerProofVerifier);
    const accounts = await getAccountsFixture();
    admin = accounts.admin;
    nonAdmin = accounts.nonAuthorizedAccount;
    // test mocker
    const mockRoot = randomBytes32();
    const timestamp = await setBeaconBlockRoot(mockRoot);
    expect(await verifier.read.getParentBlockRoot([BigInt(timestamp)])).to.equal(mockRoot);
  });

  describe("constructor", () => {
    it("It should have the correct GI_FIRST_VALIDATOR", async () => {
      expect(await verifier.read.GI_FIRST_VALIDATOR()).eq(GI_FIRST_VALIDATOR);
    });
    it("It should have the correct GI_PENDING_PARTIAL_WITHDRAWALS_ROOT", async () => {
      expect(await verifier.read.GI_PENDING_PARTIAL_WITHDRAWALS_ROOT()).eq(GI_PENDING_PARTIAL_WITHDRAWALS_ROOT);
    });
    it("It should grant DEFAULT_ADMIN_ROLE to admin", async () => {
      expect(await verifier.read.hasRole([DEFAULT_ADMIN_ROLE, admin.address])).to.be.true;
    });
    it("should revert when admin address is zero", async () => {
      const { abi } = await hre.artifacts.readArtifact("TestValidatorContainerProofVerifier");
      await expectZeroAddressRevert({
        contract: { abi },
        deployOrInitCall: deployFromFactory("TestValidatorContainerProofVerifier", [
          zeroAddress,
          GI_FIRST_VALIDATOR,
          GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
        ]),
      });
    });
    it("should revert when GI_FIRST_VALIDATOR is zero hash", async () => {
      const { abi } = await hre.artifacts.readArtifact("TestValidatorContainerProofVerifier");
      const { admin: deployer } = await getAccountsFixture();
      await expectZeroHashRevert({
        contract: { abi },
        deployOrInitCall: deployFromFactory("TestValidatorContainerProofVerifier", [
          deployer.address,
          HASH_ZERO,
          GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
        ]),
      });
    });
    it("should revert when GI_PENDING_PARTIAL_WITHDRAWALS_ROOT is zero hash", async () => {
      const { abi } = await hre.artifacts.readArtifact("TestValidatorContainerProofVerifier");
      const { admin: deployer } = await getAccountsFixture();
      await expectZeroHashRevert({
        contract: { abi },
        deployOrInitCall: deployFromFactory("TestValidatorContainerProofVerifier", [
          deployer.address,
          GI_FIRST_VALIDATOR,
          HASH_ZERO,
        ]),
      });
    });
  });

  describe("setters", () => {
    it("should allow admin to set GI_FIRST_VALIDATOR", async () => {
      const newGIndex = randomBytes32();
      await verifier.write.setGIFirstValidator([newGIndex], { account: admin.account });
      expect(await verifier.read.GI_FIRST_VALIDATOR()).to.equal(newGIndex);
    });

    it("should emit GIFirstValidatorUpdated event when setting GI_FIRST_VALIDATOR", async () => {
      const oldGIndex = await verifier.read.GI_FIRST_VALIDATOR();
      const newGIndex = randomBytes32();
      await expectEvent(
        verifier,
        verifier.write.setGIFirstValidator([newGIndex], { account: admin.account }),
        "GIFirstValidatorUpdated",
        [oldGIndex, newGIndex],
      );
    });

    it("should allow admin to set GI_PENDING_PARTIAL_WITHDRAWALS_ROOT", async () => {
      const newGIndex = randomBytes32();
      await verifier.write.setGIPendingPartialWithdrawalsRoot([newGIndex], { account: admin.account });
      expect(await verifier.read.GI_PENDING_PARTIAL_WITHDRAWALS_ROOT()).to.equal(newGIndex);
    });

    it("should emit GIPendingPartialWithdrawalsRootUpdated event when setting GI_PENDING_PARTIAL_WITHDRAWALS_ROOT", async () => {
      const oldGIndex = await verifier.read.GI_PENDING_PARTIAL_WITHDRAWALS_ROOT();
      const newGIndex = randomBytes32();
      await expectEvent(
        verifier,
        verifier.write.setGIPendingPartialWithdrawalsRoot([newGIndex], { account: admin.account }),
        "GIPendingPartialWithdrawalsRootUpdated",
        [oldGIndex, newGIndex],
      );
    });

    it("should revert when non-admin tries to set GI_FIRST_VALIDATOR", async () => {
      const newGIndex = randomBytes32();
      await expectRevertWithReason(
        verifier.write.setGIFirstValidator([newGIndex], { account: nonAdmin.account }),
        buildAccessErrorMessage(nonAdmin, DEFAULT_ADMIN_ROLE),
      );
    });

    it("should revert when non-admin tries to set GI_PENDING_PARTIAL_WITHDRAWALS_ROOT", async () => {
      const newGIndex = randomBytes32();
      await expectRevertWithReason(
        verifier.write.setGIPendingPartialWithdrawalsRoot([newGIndex], { account: nonAdmin.account }),
        buildAccessErrorMessage(nonAdmin, DEFAULT_ADMIN_ROLE),
      );
    });

    it("should revert when admin tries to set GI_FIRST_VALIDATOR to zero hash", async () => {
      await expectRevertWithCustomError(
        verifier,
        verifier.write.setGIFirstValidator([HASH_ZERO], { account: admin.account }),
        "ZeroHashNotAllowed",
      );
    });

    it("should revert when admin tries to set GI_PENDING_PARTIAL_WITHDRAWALS_ROOT to zero hash", async () => {
      await expectRevertWithCustomError(
        verifier,
        verifier.write.setGIPendingPartialWithdrawalsRoot([HASH_ZERO], { account: admin.account }),
        "ZeroHashNotAllowed",
      );
    });
  });

  it("should verify precalculated 0x01 validator object in merkle tree", async () => {
    const [, validatorMerkleRoot] = await sszMerkleTree.read.getValidatorPubkeyWCParentProof([
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator,
    ]);
    const [beaconHeaderMerkleProof, beaconHeaderMerkleRoot, , beaconHeaderMerkleIndex] =
      await sszMerkleTree.read.getBeaconBlockHeaderProof([ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader]);
    const validatorGIndex = await verifier.read.getValidatorGI([ACTIVE_0X01_VALIDATOR_PROOF.witness.validatorIndex]);

    // Verify (ValidatorContainer) leaf against (StateRoot) Merkle root
    await sszMerkleTree.read.verifyProof([
      [...ACTIVE_0X01_VALIDATOR_PROOF.witness.proof],
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.stateRoot,
      validatorMerkleRoot,
      validatorGIndex,
    ]);

    // Verify (StateRoot) leaf against (BeaconBlockRoot) Merkle root
    await sszMerkleTree.read.verifyProof([
      [...beaconHeaderMerkleProof],
      beaconHeaderMerkleRoot,
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.stateRoot,
      beaconHeaderMerkleIndex,
    ]);

    // concatentate all proofs to match PG style
    const concatenatedProof = [...ACTIVE_0X01_VALIDATOR_PROOF.witness.proof, ...beaconHeaderMerkleProof];

    const timestamp = await setBeaconBlockRoot(ACTIVE_0X01_VALIDATOR_PROOF.blockRoot);

    const ValidatorContainerWitness: ValidatorContainerWitness = {
      proof: concatenatedProof,
      effectiveBalance: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.effectiveBalance),
      activationEpoch: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.activationEpoch),
      activationEligibilityEpoch: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.activationEligibilityEpoch),
    };

    // PG style proof verification from PK+WC to BeaconBlockRoot
    await verifier.read.verifyActiveValidatorContainer([
      ValidatorContainerWitness,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.pubkey,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.withdrawalCredentials,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validatorIndex,
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.slot,
      BigInt(timestamp),
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.proposerIndex,
    ]);
  });

  it("should revert if no beacon chain root found for the timestamp", async () => {
    const [, validatorMerkleRoot] = await sszMerkleTree.read.getValidatorPubkeyWCParentProof([
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator,
    ]);
    const [beaconHeaderMerkleProof, beaconHeaderMerkleRoot, , beaconHeaderMerkleIndex] =
      await sszMerkleTree.read.getBeaconBlockHeaderProof([ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader]);
    const validatorGIndex = await verifier.read.getValidatorGI([ACTIVE_0X01_VALIDATOR_PROOF.witness.validatorIndex]);

    // Verify (ValidatorContainer) leaf against (StateRoot) Merkle root
    await sszMerkleTree.read.verifyProof([
      [...ACTIVE_0X01_VALIDATOR_PROOF.witness.proof],
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.stateRoot,
      validatorMerkleRoot,
      validatorGIndex,
    ]);

    // Verify (StateRoot) leaf against (BeaconBlockRoot) Merkle root
    await sszMerkleTree.read.verifyProof([
      [...beaconHeaderMerkleProof],
      beaconHeaderMerkleRoot,
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.stateRoot,
      beaconHeaderMerkleIndex,
    ]);

    // concatentate all proofs to match PG style
    const concatenatedProof = [...ACTIVE_0X01_VALIDATOR_PROOF.witness.proof, ...beaconHeaderMerkleProof];

    const ValidatorContainerWitness: ValidatorContainerWitness = {
      proof: concatenatedProof,
      effectiveBalance: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.effectiveBalance),
      activationEpoch: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.activationEpoch),
      activationEligibilityEpoch: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.activationEligibilityEpoch),
    };

    // PG style proof verification from PK+WC to BeaconBlockRoot
    const call = verifier.read.verifyActiveValidatorContainer([
      ValidatorContainerWitness,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.pubkey,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.withdrawalCredentials,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validatorIndex,
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.slot,
      0n,
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.proposerIndex,
    ]);
    await expectRevertWithCustomError(verifier, call, "RootNotFound");
  });

  it("can verify against dynamic merkle tree", async () => {
    const validator = generateValidatorContainer();

    const [validatorMerkleProof, validatorMerkleRoot, validatorMerkleParentNode, validatorMerkleParentIndex] =
      await sszMerkleTree.read.getValidatorPubkeyWCParentProof([validator]);

    // Verify (PK+WC) leaf against (ValidatorRoot) Merkle root
    await sszMerkleTree.read.verifyProof([
      [...validatorMerkleProof],
      validatorMerkleRoot,
      validatorMerkleParentNode,
      validatorMerkleParentIndex,
    ]);

    // deploy new verifier with new gIFirstValidator
    const { admin: deployer } = await getAccountsFixture();
    const newVerifier = await deployFromFactory("TestValidatorContainerProofVerifier", [
      deployer.address,
      localTreeGiFirstValidator,
      GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
    ]);

    // add validator to CL state merkle tree
    await sszMerkleTree.write.addValidatorLeaf([validator]);
    const validatorIndex = lastValidatorIndex + 1n;
    const stateRoot = await sszMerkleTree.read.getMerkleRoot();

    const validatorLeafIndex = firstValidatorLeafIndex + validatorIndex;
    const stateProof = await sszMerkleTree.read.getMerkleProof([validatorLeafIndex]);
    const validatorGIndex = await sszMerkleTree.read.getGeneralizedIndex([validatorLeafIndex]);

    expect(await newVerifier.read.getValidatorGI([validatorIndex])).to.equal(validatorGIndex);

    // Verify (ValidatorContainer) leaf against (StateRoot) Merkle root
    await sszMerkleTree.read.verifyProof([[...stateProof], stateRoot, validatorMerkleRoot, validatorGIndex]);

    // Pass ValidatorNotActiveForLongEnough() error
    const activationEpoch = validator.activationEpoch;
    const minimumSlot = 32n * (activationEpoch + 256n) + 1n;

    const beaconHeader = generateBeaconHeader(stateRoot, Number(minimumSlot));
    const [beaconMerkleProof, beaconMerkleRoot, , beaconMerkleIndex] =
      await sszMerkleTree.read.getBeaconBlockHeaderProof([beaconHeader]);

    // Verify (StateRoot) leaf against (BeaconBlockRoot) Merkle root
    await sszMerkleTree.read.verifyProof([[...beaconMerkleProof], beaconMerkleRoot, stateRoot, beaconMerkleIndex]);

    const timestamp = await setBeaconBlockRoot(beaconMerkleRoot);
    const proof = [...stateProof, ...beaconMerkleProof];

    await newVerifier.read.verifyActiveValidatorContainer([
      {
        proof: [...proof],
        effectiveBalance: validator.effectiveBalance,
        activationEpoch: validator.activationEpoch,
        activationEligibilityEpoch: validator.activationEligibilityEpoch,
      },
      validator.pubkey,
      validator.withdrawalCredentials,
      validatorIndex,
      beaconHeader.slot,
      BigInt(timestamp),
      beaconHeader.proposerIndex,
    ]);
  });

  it("should validate proof with different gIndex after update", async () => {
    const provenValidator = generateValidatorContainer();
    const slot = 100000;
    provenValidator.activationEpoch = BigInt(Math.floor(slot / 32) - 257);

    const prepareCLState = async (gIndex: Hex, slotNum: number) => {
      const {
        sszMerkleTree: localTree,
        gIFirstValidator,
        firstValidatorLeafIndex: localFirstValidatorLeafIndex,
      } = await prepareLocalMerkleTree(gIndex);
      await localTree.write.addValidatorLeaf([provenValidator]);

      const gIndexProven = await localTree.read.getGeneralizedIndex([localFirstValidatorLeafIndex + 1n]);
      const stateProof = await localTree.read.getMerkleProof([localFirstValidatorLeafIndex + 1n]);
      const beaconHeader = generateBeaconHeader(await localTree.read.getMerkleRoot(), slotNum);
      const [beaconMerkleProof, beaconMerkleRoot] = await localTree.read.getBeaconBlockHeaderProof([beaconHeader]);
      const proof = [...stateProof, ...beaconMerkleProof];

      return {
        localTree,
        gIFirstValidator,
        gIndexProven,
        proof: [...proof],
        beaconHeader,
        beaconRoot: beaconMerkleRoot,
      };
    };

    const curr = await prepareCLState("0x0000000000000000000000000000000000000000000000000096000000000028", slot);

    // deploy verifier with initial GIndex
    const { admin: deployer } = await getAccountsFixture();
    const newVerifier = await deployFromFactory("TestValidatorContainerProofVerifier", [
      deployer.address,
      curr.gIFirstValidator,
      GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
    ]);

    expect(await newVerifier.read.getValidatorGI([1n])).to.equal(curr.gIndexProven);

    // verify proof works with initial GIndex
    const timestampCurr = await setBeaconBlockRoot(curr.beaconRoot);
    await newVerifier.read.verifyActiveValidatorContainer([
      {
        proof: curr.proof,
        effectiveBalance: provenValidator.effectiveBalance,
        activationEpoch: provenValidator.activationEpoch,
        activationEligibilityEpoch: provenValidator.activationEligibilityEpoch,
      },
      provenValidator.pubkey,
      provenValidator.withdrawalCredentials,
      1n,
      curr.beaconHeader.slot,
      BigInt(timestampCurr),
      curr.beaconHeader.proposerIndex,
    ]);
  });

  it("should verify fabricated 0x02 validator object in merkle tree", async () => {
    const randomAddress = generateRandomAddress();
    const eip4788Witness = await generateEIP4478Witness(sszMerkleTree, verifier, randomAddress);
    const timestamp = await setBeaconBlockRoot(eip4788Witness.blockRoot);
    // PG style proof verification from PK+WC to BeaconBlockRoot
    await verifier.read.verifyActiveValidatorContainer([
      eip4788Witness.beaconProofWitness.validatorContainerWitness,
      eip4788Witness.pubkey,
      eip4788Witness.withdrawalCredentials,
      eip4788Witness.validatorIndex,
      eip4788Witness.beaconBlockHeader.slot,
      BigInt(timestamp),
      eip4788Witness.beaconProofWitness.proposerIndex,
    ]);
  });

  it("should revert for Validator that has not been active for long enough", async () => {
    const slot = randomInt(1743359);
    const epoch = BigInt(slot) / SLOTS_PER_EPOCH;
    // Ensure activationEpoch is non-negative (uint64 cannot be negative)
    // If epoch is too small, use 0 as activationEpoch which will still fail the check
    const activationEpoch = epoch >= SHARD_COMMITTEE_PERIOD - 1n ? epoch - SHARD_COMMITTEE_PERIOD + 1n : 0n;

    await expectRevertWithCustomError(
      verifier,
      verifier.read.validateActivationEpoch([BigInt(slot), activationEpoch]),
      "ValidatorNotActiveForLongEnough",
    );
  });

  it("should verify random pending partial withdrawals with synthetic Merkle proofs", async () => {
    const randomAddress = generateRandomAddress();
    const eip4788Witness = await generateEIP4478Witness(
      sszMerkleTree,
      verifier,
      randomAddress,
      ONE_GWEI * 32n,
      [],
      200, // increase the number of random pending partial withdrawals to higher manually to avoid flakiness on the CI
    );
    const timestamp = await setBeaconBlockRoot(eip4788Witness.blockRoot);
    // PG style proof verification from PK+WC to BeaconBlockRoot
    await verifier.read.verifyPendingPartialWithdrawals([
      eip4788Witness.beaconProofWitness.pendingPartialWithdrawalsWitness,
      eip4788Witness.beaconBlockHeader.slot,
      BigInt(timestamp),
      eip4788Witness.beaconProofWitness.proposerIndex,
    ]);
  });
});
