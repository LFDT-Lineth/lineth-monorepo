import { loadFixture, time } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { TestLineaRollup } from "contracts/typechain-types";
import { ethers } from "hardhat";

import { OPERATOR_ROLE } from "../../common/constants";
import { deployUpgradableFromFactory } from "../../common/deployment";
import { generateRandomBytes } from "../../common/helpers";

const EMPTY_HASH = ethers.ZeroHash;
const INITIAL_BLOCK_NUMBER = 100n;
const LEGACY_PARENT_STATE_ROOT = generateRandomBytes(32);
const INITIAL_BLOCK_HASH = generateRandomBytes(32);
const PROOF = "0x01";

describe("LineaRollup finalization migration", () => {
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

    const lineaRollup = (await deployUpgradableFromFactory(
      "TestLineaRollup",
      [initializationData, fallbackOperator.address, yieldManager.address],
      {
        initializer: "initialize",
        unsafeAllow: ["constructor", "incorrect-initializer-order"],
      },
    )) as unknown as TestLineaRollup;

    await lineaRollup.setBlockHash(INITIAL_BLOCK_NUMBER, EMPTY_HASH);
    await lineaRollup.setStateRootHash(INITIAL_BLOCK_NUMBER, LEGACY_PARENT_STATE_ROOT);

    return { lineaRollup, operator, initializationData };
  }

  async function createFinalizationData(lineaRollup: TestLineaRollup, overrides: Record<string, unknown> = {}) {
    const parentShnarf = await lineaRollup.currentFinalizedShnarf();
    const finalizationData = {
      parentStateRootHash: LEGACY_PARENT_STATE_ROOT,
      parentBlockHash: EMPTY_HASH,
      endBlockNumber: 200n,
      shnarfData: {
        parentShnarf,
        snarkHash: EMPTY_HASH,
        finalStateRootHash: EMPTY_HASH,
        dataEvaluationPoint: EMPTY_HASH,
        dataEvaluationClaim: EMPTY_HASH,
      },
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
      finalBlobHash: generateRandomBytes(32),
      l2MerkleRoots: [],
      filteredAddresses: [],
      verifierKeys: [],
      l2MessagingBlocksOffsets: "0x",
      ...overrides,
    };

    const finalShnarf = ethers.solidityPackedKeccak256(
      ["bytes32", "bytes32", "bytes32"],
      [
        finalizationData.shnarfData.parentShnarf,
        finalizationData.finalBlockHash as string,
        finalizationData.finalBlobHash as string,
      ],
    );
    await lineaRollup.setupParentShnarf(finalShnarf);

    return finalizationData;
  }

  it("migrates once using the legacy parent root and anchors the new final block hash", async () => {
    const { lineaRollup, operator } = await loadFixture(deployFixture);
    const finalizationData = await createFinalizationData(lineaRollup);

    await lineaRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData);

    expect(await lineaRollup.blockHashes(finalizationData.endBlockNumber)).to.equal(finalizationData.finalBlockHash);
    expect(await lineaRollup.stateRootHashes(finalizationData.endBlockNumber)).to.equal(EMPTY_HASH);
  });

  it("rejects a nonzero declared parent block hash during migration", async () => {
    const { lineaRollup, operator } = await loadFixture(deployFixture);
    const finalizationData = await createFinalizationData(lineaRollup, {
      parentBlockHash: generateRandomBytes(32),
    });

    await expect(
      lineaRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData),
    ).to.be.revertedWithCustomError(lineaRollup, "StartingBlockHashDoesNotMatch");
  });

  it("rejects an incorrect legacy parent state root during migration", async () => {
    const { lineaRollup, operator } = await loadFixture(deployFixture);
    const finalizationData = await createFinalizationData(lineaRollup, {
      parentStateRootHash: generateRandomBytes(32),
    });

    await expect(
      lineaRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData),
    ).to.be.revertedWithCustomError(lineaRollup, "StartingRootHashDoesNotMatch");
  });

  it("rejects migration when the legacy parent state root is missing", async () => {
    const { lineaRollup, operator } = await loadFixture(deployFixture);
    await lineaRollup.setStateRootHash(INITIAL_BLOCK_NUMBER, EMPTY_HASH);
    const finalizationData = await createFinalizationData(lineaRollup, {
      parentStateRootHash: EMPTY_HASH,
    });

    await expect(
      lineaRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData),
    ).to.be.revertedWithCustomError(lineaRollup, "StartingRootHashDoesNotMatch");
  });

  it("rejects a zero final block hash", async () => {
    const { lineaRollup, operator } = await loadFixture(deployFixture);
    const finalizationData = await createFinalizationData(lineaRollup, {
      finalBlockHash: EMPTY_HASH,
    });

    await expect(
      lineaRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData),
    ).to.be.revertedWithCustomError(lineaRollup, "FinalizationBlockHashIsZeroHash");
  });

  it("rejects a legacy-format shnarf during migration", async () => {
    const { lineaRollup, operator } = await loadFixture(deployFixture);
    const finalizationData = await createFinalizationData(lineaRollup);
    const newShnarf = ethers.solidityPackedKeccak256(
      ["bytes32", "bytes32", "bytes32"],
      [finalizationData.shnarfData.parentShnarf, finalizationData.finalBlockHash, finalizationData.finalBlobHash],
    );
    const legacyShnarf = ethers.solidityPackedKeccak256(
      ["bytes32", "bytes32", "bytes32", "bytes32", "bytes32"],
      [
        finalizationData.shnarfData.parentShnarf,
        finalizationData.shnarfData.snarkHash,
        finalizationData.shnarfData.finalStateRootHash,
        finalizationData.shnarfData.dataEvaluationPoint,
        finalizationData.shnarfData.dataEvaluationClaim,
      ],
    );
    await lineaRollup.setShnarfFinalBlockNumber(newShnarf, 0);
    await lineaRollup.setupParentShnarf(legacyShnarf);

    await expect(lineaRollup.connect(operator).finalizeBlocks(PROOF, 0, finalizationData))
      .to.be.revertedWithCustomError(lineaRollup, "FinalShnarfNotSubmitted")
      .withArgs(newShnarf);
  });

  it("uses the migrated final block hash as the next round parent", async () => {
    const { lineaRollup, operator } = await loadFixture(deployFixture);
    const migrationData = await createFinalizationData(lineaRollup);
    await lineaRollup.connect(operator).finalizeBlocks(PROOF, 0, migrationData);

    const nextFinalBlockHash = generateRandomBytes(32);
    const nextFinalBlobHash = generateRandomBytes(32);
    const nextFinalizationData = await createFinalizationData(lineaRollup, {
      parentStateRootHash: EMPTY_HASH,
      parentBlockHash: migrationData.finalBlockHash,
      endBlockNumber: 300n,
      shnarfData: {
        ...migrationData.shnarfData,
        parentShnarf: await lineaRollup.currentFinalizedShnarf(),
      },
      lastFinalizedTimestamp: migrationData.finalTimestamp,
      finalTimestamp: migrationData.finalTimestamp + 1n,
      finalBlockHash: nextFinalBlockHash,
      finalBlobHash: nextFinalBlobHash,
    });

    await lineaRollup.connect(operator).finalizeBlocks(PROOF, 0, nextFinalizationData);

    expect(await lineaRollup.blockHashes(nextFinalizationData.endBlockNumber)).to.equal(nextFinalBlockHash);
  });

  it("rejects an incorrect parent block hash after migration", async () => {
    const { lineaRollup, operator } = await loadFixture(deployFixture);
    const migrationData = await createFinalizationData(lineaRollup);
    await lineaRollup.connect(operator).finalizeBlocks(PROOF, 0, migrationData);

    const nextFinalizationData = await createFinalizationData(lineaRollup, {
      parentStateRootHash: EMPTY_HASH,
      parentBlockHash: generateRandomBytes(32),
      endBlockNumber: 300n,
      shnarfData: {
        ...migrationData.shnarfData,
        parentShnarf: await lineaRollup.currentFinalizedShnarf(),
      },
      lastFinalizedTimestamp: migrationData.finalTimestamp,
      finalTimestamp: migrationData.finalTimestamp + 1n,
      finalBlockHash: generateRandomBytes(32),
      finalBlobHash: generateRandomBytes(32),
    });

    await expect(
      lineaRollup.connect(operator).finalizeBlocks(PROOF, 0, nextFinalizationData),
    ).to.be.revertedWithCustomError(lineaRollup, "StartingBlockHashDoesNotMatch");
  });
});
