/**
 * End-to-end in-place upgrade tests from the previously deployed bytecode.
 *
 * Unlike the unit reinitialisation tests (which simulate a legacy proxy by overwriting the initialized-version slot),
 * these deploy the actual legacy implementations (Validium "2.0" and LinethRollup "8.0", from
 * `local-deployments-artifacts/dynamic-artifacts`) behind a real TransparentUpgradeableProxy + ProxyAdmin, initialize
 * them exactly as a partner/production network would have, and then upgrade via `ProxyAdmin.upgradeAndCall` with the
 * new reinitializer. This exercises:
 *   - storage layout compatibility (pre-upgrade state is read back through the new implementation),
 *   - the real initialized version a legacy proxy is at (and that the new `initialize` rejects it),
 *   - the legacy-shnarf -> dataRollingHash migration,
 *   - a full post-upgrade submission + finalization chaining from the migrated value, using the
 *     "no transition" defaults for the final L1 rolling hash fields.
 */
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { BaseContract, ContractFactory, InterfaceAbi } from "ethers";
import { ethers } from "hardhat";

import { getAccountsFixture } from "./helpers";
import LinethRollupV8Artifact from "../../../local-deployments-artifacts/dynamic-artifacts/LinethRollupV8.json";
import ValidiumV2Artifact from "../../../local-deployments-artifacts/dynamic-artifacts/ValidiumV2.json";
import {
  ADDRESS_ZERO,
  GENESIS_L2_TIMESTAMP,
  HASH_ZERO,
  INITIALIZED_ALREADY_MESSAGE,
  INITIAL_MIGRATION_BLOCK,
  INITIAL_WITHDRAW_LIMIT,
  ONE_DAY_IN_SECONDS,
  OPERATOR_ROLE,
} from "../common/constants";
import {
  computeDataRollingHash,
  expectEvent,
  expectEvents,
  expectRevertWithCustomError,
  expectRevertWithReason,
  generateRandomBytes,
} from "../common/helpers";

const PROXY_ADMIN = "src/_testing/integration/ProxyAdmin.sol:ProxyAdmin";
const TRANSPARENT_PROXY = "src/_testing/integration/ProxyAdmin.sol:TransparentUpgradeableProxy";

type Legacy = {
  proxy: BaseContract & Record<string, any>; // eslint-disable-line @typescript-eslint/no-explicit-any
  proxyAdmin: BaseContract & Record<string, any>; // eslint-disable-line @typescript-eslint/no-explicit-any
  proxyAddress: string;
  verifier: string;
};

const toBytes8 = (version: string) => ethers.zeroPadBytes(ethers.toUtf8Bytes(version), 8);

async function initializedVersion(proxyAddress: string): Promise<bigint> {
  // OpenZeppelin 4.9 Initializable: `uint8 _initialized` in the low byte of slot 0.
  return BigInt(await ethers.provider.getStorage(proxyAddress, 0)) & 0xffn;
}

function legacyInitData(verifier: string, operator: string, securityCouncil: string, addressFilter: string) {
  return {
    initialStateRootHash: generateRandomBytes(32),
    initialL2BlockNumber: INITIAL_MIGRATION_BLOCK,
    genesisTimestamp: GENESIS_L2_TIMESTAMP,
    defaultVerifier: verifier,
    rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
    rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
    roleAddresses: [{ addressWithRole: operator, role: OPERATOR_ROLE }],
    pauseTypeRoles: [],
    unpauseTypeRoles: [],
    defaultAdmin: securityCouncil,
    shnarfProvider: ADDRESS_ZERO,
    addressFilter,
  };
}

async function deployLegacyProxy(
  artifact: { abi: InterfaceAbi; bytecode: string },
  initializerArgs: (verifier: string) => unknown[],
  admin: SignerWithAddress,
): Promise<Legacy> {
  const verifier = await (await ethers.getContractFactory("IntegrationTestTrueVerifier")).deploy();
  const verifierAddress = await verifier.getAddress();

  const legacyImplementation = await new ContractFactory(artifact.abi, artifact.bytecode, admin).deploy();
  const proxyAdmin = await (await ethers.getContractFactory(PROXY_ADMIN, admin)).deploy();

  const initCalldata = new ethers.Interface(artifact.abi).encodeFunctionData(
    "initialize",
    initializerArgs(verifierAddress),
  );
  const proxyContract = await (
    await ethers.getContractFactory(TRANSPARENT_PROXY, admin)
  ).deploy(await legacyImplementation.getAddress(), await proxyAdmin.getAddress(), initCalldata);
  const proxyAddress = await proxyContract.getAddress();

  return {
    proxy: new ethers.Contract(proxyAddress, artifact.abi, admin) as unknown as Legacy["proxy"],
    proxyAdmin: proxyAdmin as unknown as Legacy["proxyAdmin"],
    proxyAddress,
    verifier: verifierAddress,
  };
}

/** Finalization data for the first post-upgrade finalization, with "no transition" L1 rolling hash defaults. */
function postUpgradeFinalizationData(
  parentDataRollingHash: string,
  endDataRollingHash: string,
  finalTimestamp: bigint,
) {
  return {
    parentBlockHash: HASH_ZERO, // migration path: no block hash was ever stored by the legacy version
    endBlockNumber: 10n,
    lastFinalizedTimestamp: BigInt(GENESIS_L2_TIMESTAMP),
    finalTimestamp,
    lastFinalizedL1RollingHash: HASH_ZERO,
    l1RollingHash: HASH_ZERO, // default: no L1 rolling hash transition
    lastFinalizedL1RollingHashMessageNumber: 0n,
    l1RollingHashMessageNumber: 0n, // default
    l2MerkleTreesDepth: 0n,
    lastFinalizedForcedTransactionNumber: 0n,
    finalForcedTransactionNumber: 0n, // actual value: no forced transaction has been submitted yet
    lastFinalizedForcedTransactionRollingHash: HASH_ZERO,
    finalBlockHash: generateRandomBytes(32),
    parentDataRollingHash,
    endDataRollingHash,
    parentDataTailTakeBytes: 0n,
    finalDataTailDiscardBytes: 0n,
    l2MerkleRoots: [],
    filteredAddresses: [],
    verifierKeys: [],
    l2MessagingBlocksOffsets: "0x",
  };
}

describe("Legacy in-place upgrades (real previous bytecode)", () => {
  let admin: SignerWithAddress;
  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;

  before(async () => {
    ({ admin, securityCouncil, operator } = await loadFixture(getAccountsFixture));
  });

  describe("Validium 2.0 -> 3.0 (reinitializeValidiumV3)", () => {
    async function deployValidiumV2Fixture() {
      const legacy = await deployLegacyProxy(
        ValidiumV2Artifact as { abi: InterfaceAbi; bytecode: string },
        (verifier) => [legacyInitData(verifier, operator.address, securityCouncil.address, securityCouncil.address)],
        admin,
      );
      const newImplementation = await (await ethers.getContractFactory("Validium")).deploy();
      const validium = await ethers.getContractAt("Validium", legacy.proxyAddress);
      return { ...legacy, newImplementation: await newImplementation.getAddress(), validium };
    }

    async function upgradeValidium(fixture: Awaited<ReturnType<typeof deployValidiumV2Fixture>>) {
      const reinitCalldata = fixture.validium.interface.encodeFunctionData("reinitializeValidiumV3", []);
      return fixture.proxyAdmin.upgradeAndCall(fixture.proxyAddress, fixture.newImplementation, reinitCalldata);
    }

    it("Starts from the real legacy state: Validium 2.0 at initialized version 1 with a non-empty shnarf", async () => {
      const fixture = await loadFixture(deployValidiumV2Fixture);
      expect(await fixture.proxy.CONTRACT_VERSION()).to.equal("2.0");
      expect(await initializedVersion(fixture.proxyAddress)).to.equal(1n);
      expect(await fixture.proxy.currentFinalizedShnarf()).to.not.equal(HASH_ZERO);
    });

    it("Migrates the legacy shnarf, emits the events and lands on version 3", async () => {
      const fixture = await loadFixture(deployValidiumV2Fixture);
      const legacyShnarf = await fixture.proxy.currentFinalizedShnarf();

      await expectEvents(fixture.validium, upgradeValidium(fixture), [
        { name: "LegacyShnarfMigrated", args: [legacyShnarf] },
        { name: "LineaRollupVersionChanged", args: [toBytes8("2.0"), toBytes8("3.0")] },
      ]);

      expect(await initializedVersion(fixture.proxyAddress)).to.equal(3n);
      expect(await fixture.validium.CONTRACT_VERSION()).to.equal("3.0");
      expect(await fixture.validium.currentDataRollingHash()).to.equal(legacyShnarf);
      expect(await fixture.validium.currentDataTailBytes()).to.equal(0n);
      expect(await fixture.validium.dataRollingHashExists(legacyShnarf)).to.equal(1n);
      expect(await fixture.validium.currentFinalizedShnarf_DEPRECATED()).to.equal(HASH_ZERO);
    });

    it("Preserves the pre-upgrade state through the new storage layout", async () => {
      const fixture = await loadFixture(deployValidiumV2Fixture);
      const before = {
        currentL2BlockNumber: await fixture.proxy.currentL2BlockNumber(),
        currentFinalizedState: await fixture.proxy.currentFinalizedState(),
        verifier: await fixture.proxy.verifiers(0),
        nextForcedTransactionNumber: await fixture.proxy.nextForcedTransactionNumber(),
        shnarfProvider: await fixture.proxy.shnarfProvider(),
        addressFilter: await fixture.proxy.addressFilter(),
        limitInWei: await fixture.proxy.limitInWei(),
        periodInSeconds: await fixture.proxy.periodInSeconds(),
      };

      await upgradeValidium(fixture);
      const v = fixture.validium;

      expect(await v.currentL2BlockNumber()).to.equal(before.currentL2BlockNumber);
      expect(await v.currentFinalizedState()).to.equal(before.currentFinalizedState);
      expect(await v.verifiers(0)).to.equal(before.verifier);
      expect(await v.nextForcedTransactionNumber()).to.equal(before.nextForcedTransactionNumber);
      // The renamed provider slot keeps the legacy value (address(this) when deployed with the zero address).
      expect(await v.dataRollingHashProvider()).to.equal(before.shnarfProvider);
      expect(await v.dataRollingHashProvider()).to.equal(fixture.proxyAddress);
      expect(await v.addressFilter()).to.equal(before.addressFilter);
      expect(await v.limitInWei()).to.equal(before.limitInWei);
      expect(await v.periodInSeconds()).to.equal(before.periodInSeconds);
      expect(await v.hasRole(OPERATOR_ROLE, operator.address)).to.be.true;
      expect(await v.hasRole(await v.DEFAULT_ADMIN_ROLE(), securityCouncil.address)).to.be.true;
    });

    it("Rejects initialize and a second reinitialization after the upgrade", async () => {
      const fixture = await loadFixture(deployValidiumV2Fixture);
      await upgradeValidium(fixture);

      const initData = {
        initialBlockHash: generateRandomBytes(32),
        initialL2BlockNumber: 0n,
        genesisTimestamp: 0n,
        defaultVerifier: fixture.verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [],
        pauseTypeRoles: [],
        unpauseTypeRoles: [],
        verifierKeys: [],
        defaultAdmin: securityCouncil.address,
        dataRollingHashProvider: ADDRESS_ZERO,
        addressFilter: securityCouncil.address,
      };
      await expectRevertWithCustomError(
        fixture.validium,
        fixture.validium.initialize(initData),
        "InitializedVersionWrong",
        [0, 3],
      );
      await expectRevertWithReason(upgradeValidium(fixture), INITIALIZED_ALREADY_MESSAGE);
      await expectRevertWithReason(fixture.validium.reinitializeValidiumV3(), INITIALIZED_ALREADY_MESSAGE);
    });

    it("Rejects initialize on the legacy proxy before the upgrade call (version 1)", async () => {
      const fixture = await loadFixture(deployValidiumV2Fixture);
      // Upgrade without calling the reinitializer: the proxy is still at version 1.
      await fixture.proxyAdmin.upgrade(fixture.proxyAddress, fixture.newImplementation);
      expect(await initializedVersion(fixture.proxyAddress)).to.equal(1n);

      const initData = {
        initialBlockHash: generateRandomBytes(32),
        initialL2BlockNumber: 0n,
        genesisTimestamp: 0n,
        defaultVerifier: fixture.verifier,
        rateLimitPeriodInSeconds: ONE_DAY_IN_SECONDS,
        rateLimitAmountInWei: INITIAL_WITHDRAW_LIMIT,
        roleAddresses: [],
        pauseTypeRoles: [],
        unpauseTypeRoles: [],
        verifierKeys: [],
        defaultAdmin: securityCouncil.address,
        dataRollingHashProvider: ADDRESS_ZERO,
        addressFilter: securityCouncil.address,
      };
      await expectRevertWithCustomError(
        fixture.validium,
        fixture.validium.initialize(initData),
        "InitializedVersionWrong",
        [0, 1],
      );
    });

    it("Submits and finalizes from the migrated dataRollingHash using the no-transition defaults", async () => {
      const fixture = await loadFixture(deployValidiumV2Fixture);
      const legacyShnarf = await fixture.proxy.currentFinalizedShnarf();
      const legacyFinalizedState = await fixture.proxy.currentFinalizedState();
      await upgradeValidium(fixture);
      const v = fixture.validium;

      const endDataRollingHash = generateRandomBytes(32);
      await v.connect(operator).acceptDataRollingHash(legacyShnarf, endDataRollingHash);

      const finalTimestamp = BigInt((await ethers.provider.getBlock("latest"))!.timestamp) - 1n;
      const finalizationData = postUpgradeFinalizationData(legacyShnarf, endDataRollingHash, finalTimestamp);

      // L1 defaults keep the previous (genesis) L1 rolling hash state; forced transaction state is still the genesis
      // (0, EMPTY_HASH) passed in, so only the timestamp moves.
      // The event carries the kept values, so it always matches the stored finalized state.
      await expectEvent(v, v.connect(operator).finalizeBlocks("0x01", 0, finalizationData), "FinalizedStateUpdated", [
        finalizationData.endBlockNumber,
        finalTimestamp,
        0n,
        0n,
      ]);

      expect(legacyFinalizedState).to.not.equal(await v.currentFinalizedState());
      expect(await v.currentDataRollingHash()).to.equal(endDataRollingHash);
      expect(await v.currentL2BlockNumber()).to.equal(finalizationData.endBlockNumber);
      expect(await v.blockHashes(finalizationData.endBlockNumber)).to.equal(finalizationData.finalBlockHash);
    });
  });

  describe("LinethRollup 8.0 -> 9.0 (reinitializeLineaRollupV10)", () => {
    async function deployLinethRollupV8Fixture() {
      const yieldManager = ethers.Wallet.createRandom().address;
      const legacy = await deployLegacyProxy(
        LinethRollupV8Artifact as { abi: InterfaceAbi; bytecode: string },
        (verifier) => [
          legacyInitData(verifier, operator.address, securityCouncil.address, securityCouncil.address),
          operator.address,
          yieldManager,
        ],
        admin,
      );
      // TestLinethRollup is the production LinethRollup plus calldata DA, so the post-upgrade round-trip can run
      // without blob-carrying transactions. The reinitializer and storage layout are identical.
      const newImplementation = await (await ethers.getContractFactory("TestLinethRollup")).deploy();
      const linethRollup = await ethers.getContractAt("TestLinethRollup", legacy.proxyAddress);
      return { ...legacy, newImplementation: await newImplementation.getAddress(), linethRollup };
    }

    async function upgradeLinethRollup(fixture: Awaited<ReturnType<typeof deployLinethRollupV8Fixture>>) {
      const reinitCalldata = fixture.linethRollup.interface.encodeFunctionData("reinitializeLineaRollupV10", []);
      return fixture.proxyAdmin.upgradeAndCall(fixture.proxyAddress, fixture.newImplementation, reinitCalldata);
    }

    it("Starts from the real legacy state: LinethRollup 8.0 below initializer version 10 with a non-empty shnarf", async () => {
      const fixture = await loadFixture(deployLinethRollupV8Fixture);
      expect(await fixture.proxy.CONTRACT_VERSION()).to.equal("8.0");
      expect(await initializedVersion(fixture.proxyAddress)).to.be.lessThan(10n);
      expect(await fixture.proxy.currentFinalizedShnarf()).to.not.equal(HASH_ZERO);
    });

    it("Migrates the legacy shnarf, preserves state and lands on contract version 9.0 (initializer version 10)", async () => {
      const fixture = await loadFixture(deployLinethRollupV8Fixture);
      const legacyShnarf = await fixture.proxy.currentFinalizedShnarf();
      const before = {
        currentL2BlockNumber: await fixture.proxy.currentL2BlockNumber(),
        currentFinalizedState: await fixture.proxy.currentFinalizedState(),
        verifier: await fixture.proxy.verifiers(0),
        livenessRecoveryOperator: await fixture.proxy.livenessRecoveryOperator(),
        shnarfProvider: await fixture.proxy.shnarfProvider(),
      };

      await expectEvents(fixture.linethRollup, upgradeLinethRollup(fixture), [
        { name: "LegacyShnarfMigrated", args: [legacyShnarf] },
        { name: "LineaRollupVersionChanged", args: [toBytes8("8.0"), toBytes8("9.0")] },
      ]);

      const r = fixture.linethRollup;
      expect(await r.CONTRACT_VERSION()).to.equal("9.0");
      expect(await initializedVersion(fixture.proxyAddress)).to.equal(10n);
      expect(await r.currentDataRollingHash()).to.equal(legacyShnarf);
      expect(await r.dataRollingHashExists(legacyShnarf)).to.equal(1n);
      expect(await r.currentFinalizedShnarf_DEPRECATED()).to.equal(HASH_ZERO);

      expect(await r.currentL2BlockNumber()).to.equal(before.currentL2BlockNumber);
      expect(await r.currentFinalizedState()).to.equal(before.currentFinalizedState);
      expect(await r.verifiers(0)).to.equal(before.verifier);
      expect(await r.livenessRecoveryOperator()).to.equal(before.livenessRecoveryOperator);
      expect(await r.dataRollingHashProvider()).to.equal(before.shnarfProvider);
      expect(await r.hasRole(OPERATOR_ROLE, operator.address)).to.be.true;
    });

    it("Rejects a second reinitialization after the upgrade", async () => {
      const fixture = await loadFixture(deployLinethRollupV8Fixture);
      await upgradeLinethRollup(fixture);
      await expectRevertWithReason(fixture.linethRollup.reinitializeLineaRollupV10(), INITIALIZED_ALREADY_MESSAGE);
    });

    it("Submits and finalizes from the migrated dataRollingHash using the no-transition defaults", async () => {
      const fixture = await loadFixture(deployLinethRollupV8Fixture);
      const legacyShnarf = await fixture.proxy.currentFinalizedShnarf();
      await upgradeLinethRollup(fixture);
      const r = fixture.linethRollup;

      const compressedData = generateRandomBytes(64);
      const endDataRollingHash = computeDataRollingHash(legacyShnarf, ethers.keccak256(compressedData));
      await r.connect(operator).submitDataAsCalldata(compressedData, legacyShnarf, endDataRollingHash);

      const finalTimestamp = BigInt((await ethers.provider.getBlock("latest"))!.timestamp) - 1n;
      const finalizationData = postUpgradeFinalizationData(legacyShnarf, endDataRollingHash, finalTimestamp);

      await expectEvent(r, r.connect(operator).finalizeBlocks("0x01", 0, finalizationData), "FinalizedStateUpdated", [
        finalizationData.endBlockNumber,
        finalTimestamp,
        0n,
        0n,
      ]);

      expect(await r.currentDataRollingHash()).to.equal(endDataRollingHash);
      expect(await r.currentL2BlockNumber()).to.equal(finalizationData.endBlockNumber);
      expect(await r.blockHashes(finalizationData.endBlockNumber)).to.equal(finalizationData.finalBlockHash);
    });
  });
});
