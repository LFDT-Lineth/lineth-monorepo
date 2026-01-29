import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { SSZMerkleTree, TestValidatorContainerProofVerifier } from "contracts/typechain-types";
import { deployTestValidatorContainerProofVerifier, ValidatorContainerWitness } from "../helpers";
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import {
  ACTIVE_0X01_VALIDATOR_PROOF,
  generateBeaconHeader,
  generateEIP4478Witness,
  generateValidatorContainer,
  prepareLocalMerkleTree,
  randomInt,
  setBeaconBlockRoot,
} from "../helpers/proof";
import { ethers } from "hardhat";
import {
  GI_FIRST_VALIDATOR,
  GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
  ONE_GWEI,
  SHARD_COMMITTEE_PERIOD,
  SLOTS_PER_EPOCH,
} from "../../common/constants";
import { buildAccessErrorMessage, expectRevertWithCustomError, getAccountsFixture } from "../../common/helpers";
import { randomBytes32 } from "../../../../common/helpers/encoding";
import { expectEvent } from "../../common/helpers/expectations";

describe("ValidatorContainerProofVerifier", () => {
  let verifier: TestValidatorContainerProofVerifier;
  let sszMerkleTree: SSZMerkleTree;
  let firstValidatorLeafIndex: bigint;
  let lastValidatorIndex: bigint;
  let localTreeGiFirstValidator: string;
  let admin: SignerWithAddress;
  let nonAdmin: SignerWithAddress;

  before(async () => {
    const localTree = await prepareLocalMerkleTree();
    localTreeGiFirstValidator = localTree.gIFirstValidator;
    sszMerkleTree = localTree.sszMerkleTree;
    firstValidatorLeafIndex = localTree.firstValidatorLeafIndex;
    // populate merkle tree with validators
    for (let i = 1; i < 100; i++) {
      await sszMerkleTree.addValidatorLeaf(generateValidatorContainer());
    }
    // after adding validators, all newly added validator indexes will +n from this
    lastValidatorIndex = (await sszMerkleTree.leafCount()) - 1n - firstValidatorLeafIndex;
  });

  beforeEach(async () => {
    verifier = await loadFixture(deployTestValidatorContainerProofVerifier);
    const accounts = await getAccountsFixture();
    admin = accounts.admin;
    nonAdmin = accounts.nonAuthorizedAccount;
    // test mocker
    const mockRoot = randomBytes32();
    const timestamp = await setBeaconBlockRoot(mockRoot);
    expect(await verifier.getParentBlockRoot(timestamp)).to.equal(mockRoot);
  });

  describe("constructor", () => {
    it("It should have the correct GI_FIRST_VALIDATOR", async () => {
      expect(await verifier.GI_FIRST_VALIDATOR()).eq(GI_FIRST_VALIDATOR);
    });
    it("It should have the correct GI_PENDING_PARTIAL_WITHDRAWALS_ROOT", async () => {
      expect(await verifier.GI_PENDING_PARTIAL_WITHDRAWALS_ROOT()).eq(GI_PENDING_PARTIAL_WITHDRAWALS_ROOT);
    });
    it("It should grant DEFAULT_ADMIN_ROLE to admin", async () => {
      const DEFAULT_ADMIN_ROLE = "0x0000000000000000000000000000000000000000000000000000000000000000";
      expect(await verifier.hasRole(DEFAULT_ADMIN_ROLE, await admin.getAddress())).to.be.true;
    });
    it("should revert when admin address is zero", async () => {
      const factory = await ethers.getContractFactory("TestValidatorContainerProofVerifier");
      const deployCall = factory.deploy(ethers.ZeroAddress, GI_FIRST_VALIDATOR, GI_PENDING_PARTIAL_WITHDRAWALS_ROOT);
      await expectRevertWithCustomError(factory, deployCall, "ZeroAddressNotAllowed");
    });
    it("should revert when GI_FIRST_VALIDATOR is zero hash", async () => {
      const factory = await ethers.getContractFactory("TestValidatorContainerProofVerifier");
      const [deployer] = await ethers.getSigners();
      const deployCall = factory.deploy(
        await deployer.getAddress(),
        ethers.ZeroHash,
        GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
      );
      await expectRevertWithCustomError(factory, deployCall, "ZeroHashNotAllowed");
    });
    it("should revert when GI_PENDING_PARTIAL_WITHDRAWALS_ROOT is zero hash", async () => {
      const factory = await ethers.getContractFactory("TestValidatorContainerProofVerifier");
      const [deployer] = await ethers.getSigners();
      const deployCall = factory.deploy(await deployer.getAddress(), GI_FIRST_VALIDATOR, ethers.ZeroHash);
      await expectRevertWithCustomError(factory, deployCall, "ZeroHashNotAllowed");
    });
  });

  describe("setters", () => {
    it("should allow admin to set GI_FIRST_VALIDATOR", async () => {
      const newGIndex = randomBytes32();
      await verifier.connect(admin).setGIFirstValidator(newGIndex);
      expect(await verifier.GI_FIRST_VALIDATOR()).to.equal(newGIndex);
    });

    it("should emit GIFirstValidatorUpdated event when setting GI_FIRST_VALIDATOR", async () => {
      const oldGIndex = await verifier.GI_FIRST_VALIDATOR();
      const newGIndex = randomBytes32();
      await expectEvent(verifier, verifier.connect(admin).setGIFirstValidator(newGIndex), "GIFirstValidatorUpdated", [
        oldGIndex,
        newGIndex,
      ]);
    });

    it("should allow admin to set GI_PENDING_PARTIAL_WITHDRAWALS_ROOT", async () => {
      const newGIndex = randomBytes32();
      await verifier.connect(admin).setGIPendingPartialWithdrawalsRoot(newGIndex);
      expect(await verifier.GI_PENDING_PARTIAL_WITHDRAWALS_ROOT()).to.equal(newGIndex);
    });

    it("should emit GIPendingPartialWithdrawalsRootUpdated event when setting GI_PENDING_PARTIAL_WITHDRAWALS_ROOT", async () => {
      const oldGIndex = await verifier.GI_PENDING_PARTIAL_WITHDRAWALS_ROOT();
      const newGIndex = randomBytes32();
      await expectEvent(
        verifier,
        verifier.connect(admin).setGIPendingPartialWithdrawalsRoot(newGIndex),
        "GIPendingPartialWithdrawalsRootUpdated",
        [oldGIndex, newGIndex],
      );
    });

    it("should revert when non-admin tries to set GI_FIRST_VALIDATOR", async () => {
      const newGIndex = randomBytes32();
      const DEFAULT_ADMIN_ROLE = "0x0000000000000000000000000000000000000000000000000000000000000000";
      await expect(verifier.connect(nonAdmin).setGIFirstValidator(newGIndex)).to.be.revertedWith(
        buildAccessErrorMessage(nonAdmin, DEFAULT_ADMIN_ROLE),
      );
    });

    it("should revert when non-admin tries to set GI_PENDING_PARTIAL_WITHDRAWALS_ROOT", async () => {
      const newGIndex = randomBytes32();
      const DEFAULT_ADMIN_ROLE = "0x0000000000000000000000000000000000000000000000000000000000000000";
      await expect(verifier.connect(nonAdmin).setGIPendingPartialWithdrawalsRoot(newGIndex)).to.be.revertedWith(
        buildAccessErrorMessage(nonAdmin, DEFAULT_ADMIN_ROLE),
      );
    });

    it("should revert when admin tries to set GI_FIRST_VALIDATOR to zero hash", async () => {
      await expectRevertWithCustomError(
        verifier,
        verifier.connect(admin).setGIFirstValidator(ethers.ZeroHash),
        "ZeroHashNotAllowed",
      );
    });

    it("should revert when admin tries to set GI_PENDING_PARTIAL_WITHDRAWALS_ROOT to zero hash", async () => {
      await expectRevertWithCustomError(
        verifier,
        verifier.connect(admin).setGIPendingPartialWithdrawalsRoot(ethers.ZeroHash),
        "ZeroHashNotAllowed",
      );
    });
  });

  it("should verify precalculated 0x01 validator object in merkle tree", async () => {
    const validatorMerkle = await sszMerkleTree.getValidatorPubkeyWCParentProof(
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator,
    );
    const beaconHeaderMerkle = await sszMerkleTree.getBeaconBlockHeaderProof(
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader,
    );
    const validatorGIndex = await verifier.getValidatorGI(ACTIVE_0X01_VALIDATOR_PROOF.witness.validatorIndex);

    // Verify (ValidatorContainer) leaf against (StateRoot) Merkle root
    await sszMerkleTree.verifyProof(
      [...ACTIVE_0X01_VALIDATOR_PROOF.witness.proof],
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.stateRoot,
      validatorMerkle.root,
      validatorGIndex,
    );

    // Verify (StateRoot) leaf against (BeaconBlockRoot) Merkle root
    await sszMerkleTree.verifyProof(
      [...beaconHeaderMerkle.proof],
      beaconHeaderMerkle.root,
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.stateRoot,
      beaconHeaderMerkle.index,
    );

    // concatentate all proofs to match PG style
    const concatenatedProof = [...ACTIVE_0X01_VALIDATOR_PROOF.witness.proof, ...beaconHeaderMerkle.proof];

    const timestamp = await setBeaconBlockRoot(ACTIVE_0X01_VALIDATOR_PROOF.blockRoot);

    const ValidatorContainerWitness: ValidatorContainerWitness = {
      proof: concatenatedProof,
      effectiveBalance: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.effectiveBalance),
      activationEpoch: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.activationEpoch),
      activationEligibilityEpoch: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.activationEligibilityEpoch),
    };

    // PG style proof verification from PK+WC to BeaconBlockRoot
    await verifier.verifyActiveValidatorContainer(
      ValidatorContainerWitness,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.pubkey,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.withdrawalCredentials,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validatorIndex,
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.slot,
      timestamp,
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.proposerIndex,
    );
  });

  it("should revert if no beacon chain root found for the timestamp", async () => {
    const validatorMerkle = await sszMerkleTree.getValidatorPubkeyWCParentProof(
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator,
    );
    const beaconHeaderMerkle = await sszMerkleTree.getBeaconBlockHeaderProof(
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader,
    );
    const validatorGIndex = await verifier.getValidatorGI(ACTIVE_0X01_VALIDATOR_PROOF.witness.validatorIndex);

    // Verify (ValidatorContainer) leaf against (StateRoot) Merkle root
    await sszMerkleTree.verifyProof(
      [...ACTIVE_0X01_VALIDATOR_PROOF.witness.proof],
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.stateRoot,
      validatorMerkle.root,
      validatorGIndex,
    );

    // Verify (StateRoot) leaf against (BeaconBlockRoot) Merkle root
    await sszMerkleTree.verifyProof(
      [...beaconHeaderMerkle.proof],
      beaconHeaderMerkle.root,
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.stateRoot,
      beaconHeaderMerkle.index,
    );

    // concatentate all proofs to match PG style
    const concatenatedProof = [...ACTIVE_0X01_VALIDATOR_PROOF.witness.proof, ...beaconHeaderMerkle.proof];

    const ValidatorContainerWitness: ValidatorContainerWitness = {
      proof: concatenatedProof,
      effectiveBalance: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.effectiveBalance),
      activationEpoch: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.activationEpoch),
      activationEligibilityEpoch: BigInt(ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.activationEligibilityEpoch),
    };

    // PG style proof verification from PK+WC to BeaconBlockRoot
    const call = verifier.verifyActiveValidatorContainer(
      ValidatorContainerWitness,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.pubkey,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validator.withdrawalCredentials,
      ACTIVE_0X01_VALIDATOR_PROOF.witness.validatorIndex,
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.slot,
      0,
      ACTIVE_0X01_VALIDATOR_PROOF.beaconBlockHeader.proposerIndex,
    );
    await expectRevertWithCustomError(verifier, call, "RootNotFound");
  });

  it("can verify against dynamic merkle tree", async () => {
    const validator = generateValidatorContainer();

    const validatorMerkle = await sszMerkleTree.getValidatorPubkeyWCParentProof(validator);

    // Verify (PK+WC) leaf against (ValidatorRoot) Merkle root
    await sszMerkleTree.verifyProof(
      [...validatorMerkle.proof],
      validatorMerkle.root,
      validatorMerkle.parentNode,
      validatorMerkle.parentIndex,
    );

    // deploy new verifier with new gIFirstValidator
    const [deployer] = await ethers.getSigners();
    const factory = await ethers.getContractFactory("TestValidatorContainerProofVerifier");
    const newVerifier = await factory.deploy(
      await deployer.getAddress(),
      localTreeGiFirstValidator,
      GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
    );
    await newVerifier.waitForDeployment();

    // add validator to CL state merkle tree
    await sszMerkleTree.addValidatorLeaf(validator);
    const validatorIndex = lastValidatorIndex + 1n;
    const stateRoot = await sszMerkleTree.getMerkleRoot();

    const validatorLeafIndex = firstValidatorLeafIndex + validatorIndex;
    const stateProof = await sszMerkleTree.getMerkleProof(validatorLeafIndex);
    const validatorGIndex = await sszMerkleTree.getGeneralizedIndex(validatorLeafIndex);

    expect(await newVerifier.getValidatorGI(validatorIndex)).to.equal(validatorGIndex);

    // Verify (ValidatorContainer) leaf against (StateRoot) Merkle root
    await sszMerkleTree.verifyProof([...stateProof], stateRoot, validatorMerkle.root, validatorGIndex);

    // Pass ValidatorNotActiveForLongEnough() error
    const activationEpoch = validator.activationEpoch;
    const minimumSlot = 32n * (activationEpoch + 256n) + 1n;

    const beaconHeader = generateBeaconHeader(stateRoot, Number(minimumSlot));
    const beaconMerkle = await sszMerkleTree.getBeaconBlockHeaderProof(beaconHeader);

    // Verify (StateRoot) leaf against (BeaconBlockRoot) Merkle root
    await sszMerkleTree.verifyProof([...beaconMerkle.proof], beaconMerkle.root, stateRoot, beaconMerkle.index);

    const timestamp = await setBeaconBlockRoot(beaconMerkle.root);
    const proof = [...stateProof, ...beaconMerkle.proof];

    await newVerifier.verifyActiveValidatorContainer(
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
      timestamp,
      beaconHeader.proposerIndex,
    );
  });

  it("should validate proof with different gIndex after update", async () => {
    const provenValidator = generateValidatorContainer();
    const slot = 100000;
    provenValidator.activationEpoch = BigInt(Math.floor(slot / 32) - 257);

    const prepareCLState = async (gIndex: string, slotNum: number) => {
      const {
        sszMerkleTree: localTree,
        gIFirstValidator,
        firstValidatorLeafIndex: localFirstValidatorLeafIndex,
      } = await prepareLocalMerkleTree(gIndex);
      await localTree.addValidatorLeaf(provenValidator);

      const gIndexProven = await localTree.getGeneralizedIndex(localFirstValidatorLeafIndex + 1n);
      const stateProof = await localTree.getMerkleProof(localFirstValidatorLeafIndex + 1n);
      const beaconHeader = generateBeaconHeader(await localTree.getMerkleRoot(), slotNum);
      const beaconMerkle = await localTree.getBeaconBlockHeaderProof(beaconHeader);
      const proof = [...stateProof, ...beaconMerkle.proof];

      return {
        localTree,
        gIFirstValidator,
        gIndexProven,
        proof: [...proof],
        beaconHeader,
        beaconRoot: beaconMerkle.root,
      };
    };

    const curr = await prepareCLState("0x0000000000000000000000000000000000000000000000000096000000000028", slot);

    // deploy verifier with initial GIndex
    const [deployer] = await ethers.getSigners();
    const factory = await ethers.getContractFactory("TestValidatorContainerProofVerifier");
    const newVerifier = await factory.deploy(
      await deployer.getAddress(),
      curr.gIFirstValidator,
      GI_PENDING_PARTIAL_WITHDRAWALS_ROOT,
    );
    await newVerifier.waitForDeployment();

    expect(await newVerifier.getValidatorGI(1n)).to.equal(curr.gIndexProven);

    // verify proof works with initial GIndex
    const timestampCurr = await setBeaconBlockRoot(curr.beaconRoot);
    await newVerifier.verifyActiveValidatorContainer(
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
      timestampCurr,
      curr.beaconHeader.proposerIndex,
    );
  });

  it("should verify fabricated 0x02 validator object in merkle tree", async () => {
    const randomAddress = ethers.Wallet.createRandom().address;
    const eip4788Witness = await generateEIP4478Witness(sszMerkleTree, verifier, randomAddress);
    const timestamp = await setBeaconBlockRoot(eip4788Witness.blockRoot);
    // PG style proof verification from PK+WC to BeaconBlockRoot
    await verifier.verifyActiveValidatorContainer(
      eip4788Witness.beaconProofWitness.validatorContainerWitness,
      eip4788Witness.pubkey,
      eip4788Witness.withdrawalCredentials,
      eip4788Witness.validatorIndex,
      eip4788Witness.beaconBlockHeader.slot,
      timestamp,
      eip4788Witness.beaconProofWitness.proposerIndex,
    );
  });

  it("should revert for Validator that has not been active for long enough", async () => {
    const slot = randomInt(1743359);
    const epoch = BigInt(slot) / SLOTS_PER_EPOCH;
    // Ensure activationEpoch is non-negative (uint64 cannot be negative)
    // If epoch is too small, use 0 as activationEpoch which will still fail the check
    const activationEpoch = epoch >= SHARD_COMMITTEE_PERIOD - 1n ? epoch - SHARD_COMMITTEE_PERIOD + 1n : 0n;

    await expectRevertWithCustomError(
      verifier,
      verifier.validateActivationEpoch(slot, activationEpoch),
      "ValidatorNotActiveForLongEnough",
    );
  });

  it("should verify random pending partial withdrawals with synthetic Merkle proofs", async () => {
    const randomAddress = ethers.Wallet.createRandom().address;
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
    await verifier.verifyPendingPartialWithdrawals(
      eip4788Witness.beaconProofWitness.pendingPartialWithdrawalsWitness,
      eip4788Witness.beaconBlockHeader.slot,
      timestamp,
      eip4788Witness.beaconProofWitness.proposerIndex,
    );
  });
});
