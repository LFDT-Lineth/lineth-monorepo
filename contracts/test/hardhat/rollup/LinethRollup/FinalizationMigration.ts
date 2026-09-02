import { loadFixture, time } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { TestLinethRollup } from "contracts/typechain-types";
import { ethers } from "hardhat";

import { OPERATOR_ROLE } from "../../common/constants";
import { deployUpgradableFromFactory } from "../../common/deployment";
import { generateRandomBytes, computePositionCommitment, computeDataRollingHash } from "../../common/helpers";

const EMPTY_HASH = ethers.ZeroHash;
const INITIAL_BLOCK_NUMBER = 100n;
const LEGACY_PARENT_STATE_ROOT = generateRandomBytes(32);
const INITIAL_BLOCK_HASH = generateRandomBytes(32);
const PROOF = "0x01";

describe("LinethRollup finalization migration", () => {
  async function deployFixture() {
    const [defaultAdmin, operator, fallbackOperator, yieldManager] = await ethers.getSigners();

    const verifierFactory = await ethers.getContractFactory("IntegrationTestTrueVerifier");
    const verifier = await verifierFactory.deploy();
    await verifier.waitForDeployment();

    const initializationData = {
      initialBlockHash: INITIAL_BLOCK_HASH,
      initialL2BlockNumber: INITIAL_BLOCK_NUMBER,
      genesisTimestamp: 1n,
      defaultVerifier: await verifier.getAddress(),
      rateLimitPeriodInSeconds: 86_400n,
      rateLimitAmountInWei: ethers.parseEther("100"),
      roleAddresses: [
        {
          addressWithRole: operator.address,
          role: OPERATOR_ROLE,
        },
      ],
      pauseTypeRoles: [],
      unpauseTypeRoles: [],
      verifierKeys: [],
      defaultAdmin: defaultAdmin.address,
      shnarfProvider: ethers.ZeroAddress,
      addressFilter: defaultAdmin.address,
    };

    const linethRollup = (await deployUpgradableFromFactory(
      "TestLinethRollup",
      [initializationData, fallbackOperator.address, yieldManager.address],
      {
        initializer: "initialize",
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      },
    )) as unknown as TestLinethRollup;

    // Seed the legacy parent: block-hash slot empty (old state-root model) + the legacy state root.
    await linethRollup.setBlockHash(INITIAL_BLOCK_NUMBER, EMPTY_HASH);
    await linethRollup.setStateRootHash(INITIAL_BLOCK_NUMBER, LEGACY_PARENT_STATE_ROOT);

    // Seed the finalized-state hash so the V6 FinalizationStateIncorrect check passes.
    // Matches the 5-field tuple supplied by createFinalizationData: messageNumber 0, empty rolling hash,
    // forced-tx number 0, empty forced-tx rolling hash, lastFinalizedTimestamp 1.
    await linethRollup.setLastFinalizedState(0, EMPTY_HASH, 0, EMPTY_HASH, 1n);

    // Genesis position commitment: the stored commitment that migration opens with prev=(0,0).
    await linethRollup.setLastFinalizedShnarf(computePositionCommitment(EMPTY_HASH, 0n));

    return { linethRollup, operator, initializationData };
  }

  async function createFinalizationData(linethRollup: TestLinethRollup, overrides: Record<string, unknown> = {}) {
    const endDataRollingHash = computeDataRollingHash(EMPTY_HASH, generateRandomBytes(32));

    const finalizationData = {
      parentStateRootHash: LEGACY_PARENT_STATE_ROOT,
      parentBlockHash: EMPTY_HASH,
      endBlockNumber: 200n,
      lastFinalizedTimestamp: 1n,
      finalTimestamp: BigInt((await time.latest()) - 10),
      lastFinalizedL1RollingHash: EMPTY_HASH,
      l1RollingHash: EMPTY_HASH,
      lastFinalizedL1RollingHashMessageNumber: 0n,
      l1RollingHashMessageNumber: 0n,
      l2MerkleTreesDepth: 0n,
      lastFinalizedForcedTransactionNumber: 0n,
      finalForcedTransactionNumber: 0n,
      lastFinalizedForcedTransactionRollingHash: EMPTY_HASH,
      finalBlockHash: generateRandomBytes(32),
      prevDataRollingHash: EMPTY_HASH,
      prevOffset: 0n,
      parentDataRollingHash: EMPTY_HASH,
      endDataRollingHash,
      startOffset: 0n,
      endOffset: 0n,
      l2MerkleRoots: [],
      filteredAddresses: [],
      verifierKeys: [],
      l2MessagingBlocksOffsets: "0x",
      ...overrides,
    };

    // Anchor the end dataRollingHash so the FinalDataRollingHashNotAnchored check passes.
    await linethRollup.setupParentShnarf(finalizationData.endDataRollingHash);

    return finalizationData;
  }

  it("migrates once using the legacy parent root and anchors the new final block hash", async () => {
    const { linethRollup, operator } = await loadFixture(deployFixture);
    const finalizationData = await createFinalizationData(linethRollup);

    await linethRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData);

    expect(await linethRollup.blockHashes(finalizationData.endBlockNumber)).to.equal(finalizationData.finalBlockHash);
    expect(await linethRollup.stateRootHashes(finalizationData.endBlockNumber)).to.equal(EMPTY_HASH);
    // The position commitment is sealed for the finalized end stream position.
    expect(await linethRollup.currentFinalizedShnarf()).to.equal(
      computePositionCommitment(finalizationData.endDataRollingHash, 0n),
    );
  });

  it("rejects a nonzero declared parent block hash during migration", async () => {
    const { linethRollup, operator } = await loadFixture(deployFixture);
    const finalizationData = await createFinalizationData(linethRollup, {
      parentBlockHash: generateRandomBytes(32),
    });

    await expect(
      linethRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData),
    ).to.be.revertedWithCustomError(linethRollup, "StartingBlockHashDoesNotMatch");
  });

  it("rejects an incorrect legacy parent state root during migration", async () => {
    const { linethRollup, operator } = await loadFixture(deployFixture);
    const finalizationData = await createFinalizationData(linethRollup, {
      parentStateRootHash: generateRandomBytes(32),
    });

    await expect(
      linethRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData),
    ).to.be.revertedWithCustomError(linethRollup, "StartingRootHashDoesNotMatch");
  });

  it("rejects migration when the legacy parent state root is missing", async () => {
    const { linethRollup, operator } = await loadFixture(deployFixture);
    await linethRollup.setStateRootHash(INITIAL_BLOCK_NUMBER, EMPTY_HASH);
    const finalizationData = await createFinalizationData(linethRollup, {
      parentStateRootHash: EMPTY_HASH,
    });

    await expect(
      linethRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData),
    ).to.be.revertedWithCustomError(linethRollup, "StartingRootHashDoesNotMatch");
  });

  it("rejects a zero final block hash", async () => {
    const { linethRollup, operator } = await loadFixture(deployFixture);
    const finalizationData = await createFinalizationData(linethRollup, {
      finalBlockHash: EMPTY_HASH,
    });

    await expect(
      linethRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData),
    ).to.be.revertedWithCustomError(linethRollup, "FinalizationBlockHashIsZeroHash");
  });

  it("rejects an unanchored end dataRollingHash", async () => {
    const { linethRollup, operator } = await loadFixture(deployFixture);
    // Build the data without anchoring, then point the end stream position at a hash never submitted.
    const finalizationData = await createFinalizationData(linethRollup);
    const unanchoredEnd = generateRandomBytes(32);
    finalizationData.endDataRollingHash = unanchoredEnd;

    await expect(linethRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData))
      .to.be.revertedWithCustomError(linethRollup, "FinalDataRollingHashNotAnchored")
      .withArgs(unanchoredEnd);
  });

  it("rejects a position commitment that does not match the stored commitment", async () => {
    const { linethRollup, operator } = await loadFixture(deployFixture);
    const finalizationData = await createFinalizationData(linethRollup, {
      // Wrong prev position: the stored commitment is commit(0,0), this opens a different one.
      prevDataRollingHash: generateRandomBytes(32),
    });
    // Keep parent == prev so the DA-continuity check isn't what fires.
    finalizationData.parentDataRollingHash = finalizationData.prevDataRollingHash;

    await expect(
      linethRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData),
    ).to.be.revertedWithCustomError(linethRollup, "PositionCommitmentMismatch");
  });

  it("uses the migrated final block hash as the next round parent", async () => {
    const { linethRollup, operator } = await loadFixture(deployFixture);
    const migrationData = await createFinalizationData(linethRollup);
    await linethRollup.connect(operator).finalizeBlocks(PROOF, 0, migrationData);

    const migrationEnd = migrationData.endDataRollingHash;
    const nextEndDataRollingHash = computeDataRollingHash(migrationEnd, generateRandomBytes(32));
    const nextFinalizationData = await createFinalizationData(linethRollup, {
      parentStateRootHash: EMPTY_HASH,
      parentBlockHash: migrationData.finalBlockHash,
      endBlockNumber: 300n,
      prevDataRollingHash: migrationEnd,
      parentDataRollingHash: migrationEnd,
      endDataRollingHash: nextEndDataRollingHash,
      lastFinalizedTimestamp: migrationData.finalTimestamp,
      finalTimestamp: migrationData.finalTimestamp + 1n,
      finalBlockHash: generateRandomBytes(32),
    });

    await linethRollup.connect(operator).finalizeBlocks(PROOF, 0, nextFinalizationData);

    expect(await linethRollup.blockHashes(nextFinalizationData.endBlockNumber)).to.equal(
      nextFinalizationData.finalBlockHash,
    );
    expect(await linethRollup.currentFinalizedShnarf()).to.equal(computePositionCommitment(nextEndDataRollingHash, 0n));
  });

  it("rejects an incorrect parent block hash after migration", async () => {
    const { linethRollup, operator } = await loadFixture(deployFixture);
    const migrationData = await createFinalizationData(linethRollup);
    await linethRollup.connect(operator).finalizeBlocks(PROOF, 0, migrationData);

    const migrationEnd = migrationData.endDataRollingHash;
    const nextFinalizationData = await createFinalizationData(linethRollup, {
      parentStateRootHash: EMPTY_HASH,
      parentBlockHash: generateRandomBytes(32),
      endBlockNumber: 300n,
      prevDataRollingHash: migrationEnd,
      parentDataRollingHash: migrationEnd,
      endDataRollingHash: computeDataRollingHash(migrationEnd, generateRandomBytes(32)),
      lastFinalizedTimestamp: migrationData.finalTimestamp,
      finalTimestamp: migrationData.finalTimestamp + 1n,
      finalBlockHash: generateRandomBytes(32),
    });

    await expect(
      linethRollup.connect(operator).finalizeBlocks(PROOF, 0, nextFinalizationData),
    ).to.be.revertedWithCustomError(linethRollup, "StartingBlockHashDoesNotMatch");
  });
});
