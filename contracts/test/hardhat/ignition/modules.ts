import { expect } from "chai";
import { describe, it } from "node:test";
import { getAddress, parseEther, zeroHash } from "viem";

import { buildParametersFromEnv, deployedAddress, expectCode, expectPinnedTransparentProxy } from "./helpers.js";
import {
  L1_L2_MESSAGE_SETTER_ROLE,
  OPERATOR_ROLE,
  PAUSE_ALL_ROLE,
  SECURITY_COUNCIL_ROLE,
} from "../../../common/constants/index.js";
import BridgedTokenModule from "../../../ignition/modules/bridging/BridgedToken.js";
import CustomBridgedTokenModule from "../../../ignition/modules/bridging/CustomBridgedToken.js";
import TokenBridgeModule from "../../../ignition/modules/bridging/TokenBridge.js";
import TimelockModule from "../../../ignition/modules/governance/Timelock.js";
import { buildL1ProtocolModule } from "../../../ignition/modules/lib/l1Protocol.js";
import {
  pinnedL2MessageServiceV1Artifact,
  pinnedUpgradeableBeaconArtifact,
} from "../../../ignition/modules/lib/pinned.js";
import { buildPlonkVerifierModule } from "../../../ignition/modules/lib/plonkVerifier.js";
import L2MessageServiceModule from "../../../ignition/modules/messaging/L2MessageService.js";
import L2MessageServiceV1Module from "../../../ignition/modules/messaging/L2MessageServiceV1.js";
import L1LineaTokenBurnerModule from "../../../ignition/modules/operational/L1LineaTokenBurner.js";
import LineaSequencerUptimeFeedModule from "../../../ignition/modules/operational/LineaSequencerUptimeFeed.js";
import PaymentSplitterWrapperModule from "../../../ignition/modules/operational/PaymentSplitterWrapper.js";
import RollupRevenueVaultModule from "../../../ignition/modules/operational/RollupRevenueVault.js";
import SafeExecutionConditionsModule from "../../../ignition/modules/operational/SafeExecutionConditions.js";
import V3DexSwapAdapterModule from "../../../ignition/modules/operational/V3DexSwapAdapter.js";
import UpgradeableBeaconChainDepositPredeployModule from "../../../ignition/modules/predeploy/UpgradeableBeaconChainDepositPredeploy.js";
import UpgradeableConsolidationQueuePredeployModule from "../../../ignition/modules/predeploy/UpgradeableConsolidationQueuePredeploy.js";
import UpgradeableWithdrawalQueuePredeployModule from "../../../ignition/modules/predeploy/UpgradeableWithdrawalQueuePredeploy.js";
import CallForwardingProxyModule from "../../../ignition/modules/proxies/CallForwardingProxy.js";
import RecoverFundsModule from "../../../ignition/modules/recovery/RecoverFunds.js";
import AddressFilterModule from "../../../ignition/modules/rollup/AddressFilter.js";
import ForcedTransactionGatewayModule from "../../../ignition/modules/rollup/ForcedTransactionGateway.js";
import LinethRollupModule, { LIVENESS_RECOVERY_OPERATOR } from "../../../ignition/modules/rollup/LinethRollup.js";
import ValidiumModule from "../../../ignition/modules/rollup/Validium.js";
import L2ProtocolModule from "../../../ignition/modules/stacks/L2Protocol.js";
import TokenBridgeWithBeaconModule from "../../../ignition/modules/stacks/TokenBridgeWithBeacon.js";
import TestERC20Module from "../../../ignition/modules/testing/TestERC20.js";
import YieldManagerUpgradeModule from "../../../ignition/modules/upgrades/YieldManagerUpgrade.js";
import YieldManagerModule from "../../../ignition/modules/yield/YieldManager.js";
import YieldManagerArtifactsModule from "../../../ignition/modules/yield/YieldManagerArtifacts.js";
import { getConnection, useFreshChain } from "../common/helpers/connection.js";

import type { Address } from "viem";

const VERIFIER_CONTRACT_NAME = "PlonkVerifierForDataAggregation";
const STATE_ROOT_HASH = "0x072ead6777750dc20232d1cee8dc9a395c2d350df4bbaa5096c6f59b214dcecd";

/** Distinct, deterministic addresses for roles and external dependencies (none of them needs code). */
function role(index: number): Address {
  return getAddress(`0x${(0x1000 + index).toString(16).padStart(40, "0")}`);
}

const securityCouncil = role(1);
const operatorA = role(2);
const operatorB = role(3);
const l2MessageService = role(4);
const yieldManagerAddress = role(5);
const addressFilterAddress = role(6);
const l1l2MessageSetter = role(7);
const remoteSender = role(8);
const verifierAddress = role(9);
const linethRollupAddress = role(10);

function rollupEnv(): Record<string, string> {
  return {
    INITIAL_L2_STATE_ROOT_HASH: STATE_ROOT_HASH,
    INITIAL_L2_BLOCK_NUMBER: "123456",
    L2_GENESIS_TIMESTAMP: "1683325137",
    L1_SECURITY_COUNCIL: securityCouncil,
    LINETH_ROLLUP_OPERATORS: `${operatorA},${operatorB}`,
    LINETH_ROLLUP_RATE_LIMIT_PERIOD: "86400",
    LINETH_ROLLUP_RATE_LIMIT_AMOUNT: "1000000000000000000000",
    YIELD_MANAGER_ADDRESS: yieldManagerAddress,
    LINETH_ROLLUP_ADDRESS_FILTER: addressFilterAddress,
  };
}

function verifierEnv(): Record<string, string> {
  return {
    VERIFIER_CONTRACT_NAME,
    VERIFIER_PROOF_TYPE: "0",
    VERIFIER_CHAIN_ID: "59144",
    VERIFIER_BASE_FEE: "7",
    VERIFIER_COINBASE: role(11),
    L2_MESSAGE_SERVICE_ADDRESS: l2MessageService,
    VERIFIER_IS_ALLOWED_CIRCUIT_ID: "255",
  };
}

function timelockEnv(): Record<string, string> {
  return {
    TIMELOCK_PROPOSERS: `${securityCouncil},${operatorA}`,
    TIMELOCK_EXECUTORS: securityCouncil,
    TIMELOCK_ADMIN_ADDRESS: securityCouncil,
    MIN_DELAY: "3600",
  };
}

function l2MessageServiceEnv(): Record<string, string> {
  return {
    L2_SECURITY_COUNCIL: securityCouncil,
    L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER: l1l2MessageSetter,
    L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD: "86400",
    L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT: "1000000000000000000000",
  };
}

function tokenBridgeEnv(): Record<string, string> {
  return {
    L2_MESSAGE_SERVICE_ADDRESS: l2MessageService,
    LINETH_ROLLUP_ADDRESS: linethRollupAddress,
    REMOTE_CHAIN_ID: "59144",
    REMOTE_SENDER_ADDRESS: remoteSender,
    DEPLOY_TOKEN_BRIDGE_ON_L1: "true",
    L1_SECURITY_COUNCIL: securityCouncil,
    L1_RESERVED_TOKEN_ADDRESSES: `${role(20)},${role(21)}`,
  };
}

function yieldManagerEnv(): Record<string, string> {
  return {
    LINETH_ROLLUP_ADDRESS: linethRollupAddress,
    L1_SECURITY_COUNCIL: securityCouncil,
    NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS: role(30),
    VAULT_HUB: role(31),
    VAULT_FACTORY: role(32),
    STETH: role(33),
    MINIMUM_WITHDRAWAL_RESERVE_PERCENTAGE_BPS: "2000",
    TARGET_WITHDRAWAL_RESERVE_PERCENTAGE_BPS: "2500",
    MINIMUM_WITHDRAWAL_RESERVE_AMOUNT: "1000000000000000000000",
    TARGET_WITHDRAWAL_RESERVE_AMOUNT: "2000000000000000000000",
  };
}

describe("Ignition deployment modules", () => {
  useFreshChain();

  describe("L1", () => {
    it("PlonkVerifier deploys Mimc and the linked verifier, then reuses an existing Mimc", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();

      const parameters = await buildParametersFromEnv(connection, "PlonkVerifier", verifierEnv());
      const { verifier, mimc } = await connection.ignition.deploy(
        buildPlonkVerifierModule({ verifierContractName: VERIFIER_CONTRACT_NAME, reuseMimc: false }),
        { parameters },
      );
      await expectCode(publicClient, deployedAddress(verifier));
      await expectCode(publicClient, mimc.address);

      const reuseParameters = await buildParametersFromEnv(connection, "PlonkVerifier", {
        ...verifierEnv(),
        VERIFIER_MIMC_ADDRESS: mimc.address,
      });
      expect(reuseParameters.PlonkVerifier?.mimcLibrary).to.equal(mimc.address);
      const reused = await connection.ignition.deploy(
        buildPlonkVerifierModule({ verifierContractName: VERIFIER_CONTRACT_NAME, reuseMimc: true }),
        { parameters: reuseParameters },
      );
      expect(reused.mimc.address).to.equal(mimc.address);
      await expectCode(publicClient, deployedAddress(reused.verifier));
    });

    it("PlonkVerifier parameters reject a VERIFIER_MIMC_ADDRESS without code", async () => {
      const connection = await getConnection();
      let message = "";
      try {
        await buildParametersFromEnv(connection, "PlonkVerifier", {
          ...verifierEnv(),
          VERIFIER_MIMC_ADDRESS: role(99),
        });
      } catch (error: unknown) {
        message = error instanceof Error ? error.message : String(error);
      }
      expect(message).to.contain("has no contract bytecode on this network");
    });

    it("Timelock", async () => {
      const connection = await getConnection();
      const parameters = await buildParametersFromEnv(connection, "Timelock", timelockEnv());
      const { timelock } = await connection.ignition.deploy(TimelockModule, { parameters });
      expect(await timelock.read.getMinDelay()).to.equal(3600n);
      const proposerRole = await timelock.read.PROPOSER_ROLE();
      expect(await timelock.read.hasRole([proposerRole, operatorA])).to.equal(true);
      expect(await timelock.read.hasRole([await timelock.read.TIMELOCK_ADMIN_ROLE(), securityCouncil])).to.equal(true);
    });

    it("LinethRollup behind the pinned OZ 4.x proxy", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();
      const parameters = await buildParametersFromEnv(connection, "LinethRollup", {
        ...rollupEnv(),
        VERIFIER_ADDRESS: verifierAddress,
      });
      const { linethRollup, linethRollupImplementation, linethRollupProxy, proxyAdmin } =
        await connection.ignition.deploy(LinethRollupModule, { parameters });

      await expectPinnedTransparentProxy(
        publicClient,
        linethRollupProxy.address,
        proxyAdmin.address,
        linethRollupImplementation.address,
      );
      expect(await proxyAdmin.read.getProxyAdmin([linethRollupProxy.address])).to.equal(proxyAdmin.address);
      expect(await linethRollup.read.verifiers([0n])).to.equal(verifierAddress);
      expect(await linethRollup.read.currentL2BlockNumber()).to.equal(123456n);
      expect(await linethRollup.read.yieldManager()).to.equal(yieldManagerAddress);
      expect(await linethRollup.read.addressFilter()).to.equal(addressFilterAddress);
      expect(await linethRollup.read.livenessRecoveryOperator()).to.equal(LIVENESS_RECOVERY_OPERATOR);
      expect(await linethRollup.read.hasRole([zeroHash, securityCouncil])).to.equal(true);
      expect(await linethRollup.read.hasRole([OPERATOR_ROLE, operatorA])).to.equal(true);
      expect(await linethRollup.read.hasRole([OPERATOR_ROLE, operatorB])).to.equal(true);
      expect(await linethRollup.read.hasRole([PAUSE_ALL_ROLE, securityCouncil])).to.equal(true);
    });

    it("Validium behind the pinned OZ 4.x proxy", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();
      const parameters = await buildParametersFromEnv(connection, "Validium", {
        VERIFIER_ADDRESS: verifierAddress,
        INITIAL_L2_STATE_ROOT_HASH: STATE_ROOT_HASH,
        INITIAL_L2_BLOCK_NUMBER: "42",
        L2_GENESIS_TIMESTAMP: "1683325137",
        L1_SECURITY_COUNCIL: securityCouncil,
        VALIDIUM_OPERATORS: operatorA,
        VALIDIUM_RATE_LIMIT_PERIOD: "86400",
        VALIDIUM_RATE_LIMIT_AMOUNT: "1000000000000000000000",
        LINETH_ROLLUP_ADDRESS_FILTER: addressFilterAddress,
      });
      const { validium, validiumImplementation, validiumProxy, proxyAdmin } = await connection.ignition.deploy(
        ValidiumModule,
        { parameters },
      );
      await expectPinnedTransparentProxy(
        publicClient,
        validiumProxy.address,
        proxyAdmin.address,
        validiumImplementation.address,
      );
      expect(await validium.read.verifiers([0n])).to.equal(verifierAddress);
      expect(await validium.read.currentL2BlockNumber()).to.equal(42n);
      expect(await validium.read.hasRole([OPERATOR_ROLE, operatorA])).to.equal(true);
    });

    it("chains/L1Protocol wires the new verifier into LinethRollup (replaces the in-process handoff)", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();
      const parameters = await buildParametersFromEnv(connection, "L1Protocol", {
        ...verifierEnv(),
        ...rollupEnv(),
        ...timelockEnv(),
        // A stale registry/env verifier must not be used by the composite.
        VERIFIER_ADDRESS: role(98),
      });
      expect(parameters.L1Protocol).to.not.have.property("verifier");

      const { verifier, linethRollup, linethRollupProxy, linethRollupImplementation, proxyAdmin, timelock } =
        await connection.ignition.deploy(
          buildL1ProtocolModule({ verifierContractName: VERIFIER_CONTRACT_NAME, reuseMimc: false }),
          { parameters },
        );
      await expectPinnedTransparentProxy(
        publicClient,
        linethRollupProxy.address,
        proxyAdmin.address,
        linethRollupImplementation.address,
      );
      expect(await linethRollup.read.verifiers([0n])).to.equal(deployedAddress(verifier));
      expect(await timelock.read.getMinDelay()).to.equal(3600n);
    });

    it("RecoverFunds behind the pinned OZ 4.x proxy", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();
      const parameters = await buildParametersFromEnv(connection, "RecoverFunds", {
        L1_SECURITY_COUNCIL: securityCouncil,
        RECOVERFUNDS_EXECUTOR_ADDRESS: operatorA,
      });
      const { recoverFunds, recoverFundsImplementation, recoverFundsProxy, proxyAdmin } =
        await connection.ignition.deploy(RecoverFundsModule, { parameters });
      await expectPinnedTransparentProxy(
        publicClient,
        recoverFundsProxy.address,
        proxyAdmin.address,
        recoverFundsImplementation.address,
      );
      expect(await recoverFunds.read.hasRole([zeroHash, securityCouncil])).to.equal(true);
    });

    it("CallForwardingProxy, L1LineaTokenBurner, SafeExecutionConditions", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();

      const forwarding = await connection.ignition.deploy(CallForwardingProxyModule, {
        parameters: await buildParametersFromEnv(connection, "CallForwardingProxy", {
          LINETH_ROLLUP_ADDRESS: linethRollupAddress,
        }),
      });
      expect(await forwarding.callForwardingProxy.read.TARGET()).to.equal(linethRollupAddress);

      const burner = await connection.ignition.deploy(L1LineaTokenBurnerModule, {
        parameters: await buildParametersFromEnv(connection, "L1LineaTokenBurner", {
          LINETH_ROLLUP_ADDRESS: linethRollupAddress,
          LINEA_TOKEN: role(40),
        }),
      });
      expect(await burner.l1LineaTokenBurner.read.MESSAGE_SERVICE()).to.equal(linethRollupAddress);
      expect(await burner.l1LineaTokenBurner.read.LINEA_TOKEN()).to.equal(role(40));

      const { safeExecutionConditions } = await connection.ignition.deploy(SafeExecutionConditionsModule);
      await expectCode(publicClient, safeExecutionConditions.address);
    });

    it("AddressFilter: deployer is the temporary admin, precompiles are filtered", async () => {
      const connection = await getConnection();
      const [deployer] = await connection.viem.getWalletClients();
      await buildParametersFromEnv(connection, "AddressFilter", { L1_SECURITY_COUNCIL: securityCouncil });
      const { addressFilter } = await connection.ignition.deploy(AddressFilterModule);
      expect(await addressFilter.read.hasRole([zeroHash, deployer.account.address])).to.equal(true);
      expect(await addressFilter.read.addressIsFiltered(["0x0000000000000000000000000000000000000001"])).to.equal(true);
    });

    it("ForcedTransactionGateway linked against an existing Mimc", async () => {
      const connection = await getConnection();
      const mimc = await connection.viem.deployContract("Mimc");
      const parameters = await buildParametersFromEnv(connection, "ForcedTransactionGateway", {
        LINETH_ROLLUP_ADDRESS: linethRollupAddress,
        FORCED_TRANSACTION_GATEWAY_L2_CHAIN_ID: "59144",
        FORCED_TRANSACTION_GATEWAY_L2_BLOCK_BUFFER: "100",
        FORCED_TRANSACTION_GATEWAY_MAX_GAS_LIMIT: "10000000",
        FORCED_TRANSACTION_GATEWAY_MAX_INPUT_LENGTH_BUFFER: "1000",
        L1_SECURITY_COUNCIL: securityCouncil,
        FORCED_TRANSACTION_ADDRESS_FILTER: addressFilterAddress,
        MIMC_LIBRARY_ADDRESS: mimc.address,
        FORCED_TRANSACTION_L2_BLOCK_DURATION_SECONDS: "2",
        FORCED_TRANSACTION_BLOCK_NUMBER_DEADLINE_BUFFER: "50",
      });
      const { forcedTransactionGateway } = await connection.ignition.deploy(ForcedTransactionGatewayModule, {
        parameters,
      });
      expect(await forcedTransactionGateway.read.LINEA_ROLLUP()).to.equal(linethRollupAddress);
      expect(await forcedTransactionGateway.read.DESTINATION_CHAIN_ID()).to.equal(59144n);
      expect(await forcedTransactionGateway.read.ADDRESS_FILTER()).to.equal(addressFilterAddress);
      expect(await forcedTransactionGateway.read.hasRole([zeroHash, securityCouncil])).to.equal(true);
    });
  });

  describe("Yield", () => {
    it("YieldManager: proxy, verifier, factory and a factory-created yield provider", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();
      const parameters = await buildParametersFromEnv(connection, "YieldManager", yieldManagerEnv());
      const result = await connection.ignition.deploy(YieldManagerModule, { parameters });

      await expectPinnedTransparentProxy(
        publicClient,
        result.yieldManagerProxy.address,
        result.proxyAdmin.address,
        result.yieldManagerImplementation.address,
      );
      expect(await result.yieldManager.read.L1_MESSAGE_SERVICE()).to.equal(linethRollupAddress);
      expect(await result.yieldManager.read.minimumWithdrawalReservePercentageBps()).to.equal(2000n);
      expect(await result.yieldManager.read.targetWithdrawalReserveAmount()).to.equal(2000n * 10n ** 18n);
      expect(await result.yieldManager.read.hasRole([zeroHash, securityCouncil])).to.equal(true);
      await expectCode(publicClient, result.validatorContainerProofVerifier.address);
      await expectCode(publicClient, result.lidoStVaultYieldProvider.address);
    });

    it("YieldManagerArtifacts deploys the 2026-01-14 pinned bytecode", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();
      const parameters = await buildParametersFromEnv(connection, "YieldManagerArtifacts", yieldManagerEnv());
      const result = await connection.ignition.deploy(YieldManagerArtifactsModule, { parameters });

      await expectPinnedTransparentProxy(
        publicClient,
        result.yieldManagerProxy.address,
        result.proxyAdmin.address,
        result.yieldManagerImplementation.address,
      );
      expect(await result.yieldManager.read.L1_MESSAGE_SERVICE()).to.equal(linethRollupAddress);
      expect(await result.yieldManager.read.hasRole([zeroHash, securityCouncil])).to.equal(true);
      await expectCode(publicClient, result.lidoStVaultYieldProvider.address);
    });

    it("YieldManagerUpgrade deploys a new implementation", async () => {
      const connection = await getConnection();
      const parameters = await buildParametersFromEnv(connection, "YieldManagerUpgrade", {
        LINETH_ROLLUP_ADDRESS: linethRollupAddress,
        YIELD_MANAGER_ADDRESS: yieldManagerAddress,
      });
      const { implementation: yieldManagerImplementation } = await connection.ignition.deploy(
        YieldManagerUpgradeModule,
        {
          parameters,
        },
      );
      expect(await yieldManagerImplementation.read.L1_MESSAGE_SERVICE()).to.equal(linethRollupAddress);
    });
  });

  describe("L2", () => {
    it("L2MessageService behind the pinned OZ 4.x proxy", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();
      const parameters = await buildParametersFromEnv(connection, "L2MessageService", l2MessageServiceEnv());
      const { l2MessageService, l2MessageServiceImplementation, l2MessageServiceProxy, proxyAdmin } =
        await connection.ignition.deploy(L2MessageServiceModule, { parameters });
      await expectPinnedTransparentProxy(
        publicClient,
        l2MessageServiceProxy.address,
        proxyAdmin.address,
        l2MessageServiceImplementation.address,
      );
      expect(await l2MessageService.read.hasRole([L1_L2_MESSAGE_SETTER_ROLE, l1l2MessageSetter])).to.equal(true);
      expect(await l2MessageService.read.hasRole([SECURITY_COUNCIL_ROLE, securityCouncil])).to.equal(true);
    });

    it("L2MessageServiceV1 deploys the pinned Linea mainnet V1 implementation", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();
      const parameters = await buildParametersFromEnv(connection, "L2MessageServiceV1", l2MessageServiceEnv());
      const { l2MessageService, l2MessageServiceImplementation, l2MessageServiceProxy, proxyAdmin } =
        await connection.ignition.deploy(L2MessageServiceV1Module, { parameters });
      await expectPinnedTransparentProxy(
        publicClient,
        l2MessageServiceProxy.address,
        proxyAdmin.address,
        l2MessageServiceImplementation.address,
      );
      expect((await publicClient.getCode({ address: l2MessageServiceImplementation.address }))?.toLowerCase()).to.equal(
        pinnedL2MessageServiceV1Artifact.deployedBytecode.toLowerCase(),
      );
      expect(await l2MessageService.read.hasRole([zeroHash, securityCouncil])).to.equal(true);
      expect(
        await l2MessageService.read.hasRole([
          await l2MessageService.read.L1_L2_MESSAGE_SETTER_ROLE(),
          l1l2MessageSetter,
        ]),
      ).to.equal(true);
      expect(await l2MessageService.read.periodInSeconds()).to.equal(86400n);
    });

    it("chains/L2Protocol deploys L2MessageService and Timelock", async () => {
      const connection = await getConnection();
      const parameters = await buildParametersFromEnv(connection, "L2Protocol", {
        ...l2MessageServiceEnv(),
        ...timelockEnv(),
      });
      const { l2MessageService, timelock } = await connection.ignition.deploy(L2ProtocolModule, { parameters });
      expect(await l2MessageService.read.hasRole([L1_L2_MESSAGE_SETTER_ROLE, l1l2MessageSetter])).to.equal(true);
      expect(await timelock.read.getMinDelay()).to.equal(3600n);
    });

    it("Upgradeable predeploys behind the pinned OZ 4.x proxy", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();

      const withdrawal = await connection.ignition.deploy(UpgradeableWithdrawalQueuePredeployModule);
      await expectPinnedTransparentProxy(
        publicClient,
        withdrawal.upgradeableWithdrawalQueuePredeployProxy.address,
        withdrawal.proxyAdmin.address,
        withdrawal.upgradeableWithdrawalQueuePredeployImplementation.address,
      );
      const consolidation = await connection.ignition.deploy(UpgradeableConsolidationQueuePredeployModule);
      await expectPinnedTransparentProxy(
        publicClient,
        consolidation.upgradeableConsolidationQueuePredeployProxy.address,
        consolidation.proxyAdmin.address,
        consolidation.upgradeableConsolidationQueuePredeployImplementation.address,
      );
      const deposit = await connection.ignition.deploy(UpgradeableBeaconChainDepositPredeployModule);
      await expectPinnedTransparentProxy(
        publicClient,
        deposit.upgradeableBeaconChainDepositPredeployProxy.address,
        deposit.proxyAdmin.address,
        deposit.upgradeableBeaconChainDepositPredeployImplementation.address,
      );
    });

    it("RollupRevenueVault behind the pinned OZ 4.x proxy", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();
      const parameters = await buildParametersFromEnv(connection, "RollupRevenueVault", {
        ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE: "1700000000",
        L2_SECURITY_COUNCIL: securityCouncil,
        ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER: role(50),
        ROLLUP_REVENUE_VAULT_BURNER: role(51),
        ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER: role(52),
        TOKEN_BRIDGE_ADDRESS: role(53),
        L2_MESSAGE_SERVICE_ADDRESS: l2MessageService,
        ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER: role(54),
        ROLLUP_REVENUE_VAULT_LINEA_TOKEN: role(55),
        ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER: role(56),
      });
      const { rollupRevenueVault, rollupRevenueVaultImplementation, rollupRevenueVaultProxy, proxyAdmin } =
        await connection.ignition.deploy(RollupRevenueVaultModule, { parameters });
      await expectPinnedTransparentProxy(
        publicClient,
        rollupRevenueVaultProxy.address,
        proxyAdmin.address,
        rollupRevenueVaultImplementation.address,
      );
      expect(await rollupRevenueVault.read.lastInvoiceDate()).to.equal(1700000000n);
      expect(await rollupRevenueVault.read.lineaToken()).to.equal(role(55));
      expect(await rollupRevenueVault.read.dexSwapAdapter()).to.equal(role(56));
    });

    it("LineaSequencerUptimeFeed parses the initial status strictly", async () => {
      const connection = await getConnection();
      const parameters = await buildParametersFromEnv(connection, "LineaSequencerUptimeFeed", {
        LINETH_SEQUENCER_UPTIME_FEED_INITIAL_STATUS: "false",
        LINETH_SEQUENCER_UPTIME_FEED_ADMIN: securityCouncil,
        LINETH_SEQUENCER_UPTIME_FEED_UPDATER: operatorA,
      });
      expect(parameters.LineaSequencerUptimeFeed?.initialStatus).to.equal(false);
      const { lineaSequencerUptimeFeed } = await connection.ignition.deploy(LineaSequencerUptimeFeedModule, {
        parameters,
      });
      expect(await lineaSequencerUptimeFeed.read.latestAnswer()).to.equal(0n);
    });

    it("V3DexSwapAdapter", async () => {
      const connection = await getConnection();
      const parameters = await buildParametersFromEnv(connection, "V3DexSwapAdapter", {
        V3_DEX_SWAP_ADAPTER_ROUTER: role(60),
        V3_DEX_SWAP_ADAPTER_WETH_TOKEN: role(61),
        V3_DEX_SWAP_ADAPTER_LINEA_TOKEN: role(62),
        V3_DEX_SWAP_ADAPTER_POOL_TICK_SPACING: "60",
      });
      const { v3DexSwapAdapter } = await connection.ignition.deploy(V3DexSwapAdapterModule, { parameters });
      expect(await v3DexSwapAdapter.read.ROUTER()).to.equal(role(60));
      expect(await v3DexSwapAdapter.read.POOL_TICK_SPACING()).to.equal(60);
    });
  });

  describe("Bridging", () => {
    it("BridgedToken behind the pinned OZ 4.x beacon, owned by the deployer", async () => {
      const connection = await getConnection();
      const [deployer] = await connection.viem.getWalletClients();
      const publicClient = await connection.viem.getPublicClient();
      const { bridgedTokenBeacon, bridgedTokenImplementation } = await connection.ignition.deploy(BridgedTokenModule);
      expect((await publicClient.getCode({ address: bridgedTokenBeacon.address }))?.toLowerCase()).to.equal(
        pinnedUpgradeableBeaconArtifact.deployedBytecode.toLowerCase(),
      );
      expect(await bridgedTokenBeacon.read.implementation()).to.equal(bridgedTokenImplementation.address);
      expect(await bridgedTokenBeacon.read.owner()).to.equal(getAddress(deployer.account.address));
    });

    it("TokenBridge on L1 behind the pinned OZ 4.x proxy", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();
      const tokenBeacon = role(70);
      const parameters = await buildParametersFromEnv(connection, "TokenBridge", {
        ...tokenBridgeEnv(),
        BRIDGED_TOKEN_ADDRESS: tokenBeacon,
      });
      const { tokenBridge, tokenBridgeImplementation, tokenBridgeProxy, proxyAdmin } = await connection.ignition.deploy(
        TokenBridgeModule,
        { parameters },
      );
      await expectPinnedTransparentProxy(
        publicClient,
        tokenBridgeProxy.address,
        proxyAdmin.address,
        tokenBridgeImplementation.address,
      );
      expect(await tokenBridge.read.tokenBeacon()).to.equal(tokenBeacon);
      // DEPLOY_TOKEN_BRIDGE_ON_L1=true → the LinethRollup is the message service.
      expect(await tokenBridge.read.messageService()).to.equal(linethRollupAddress);
      expect(await tokenBridge.read.sourceChainId()).to.equal(BigInt(await publicClient.getChainId()));
      expect(await tokenBridge.read.targetChainId()).to.equal(59144n);
      expect(await tokenBridge.read.remoteSender()).to.equal(remoteSender);
      expect(await tokenBridge.read.hasRole([zeroHash, securityCouncil])).to.equal(true);
    });

    it("chains/TokenBridgeWithBeacon initializes the TokenBridge with the new beacon", async () => {
      const connection = await getConnection();
      const parameters = await buildParametersFromEnv(connection, "TokenBridgeWithBeacon", {
        ...tokenBridgeEnv(),
        DEPLOY_TOKEN_BRIDGE_ON_L1: "false",
        L2_SECURITY_COUNCIL: securityCouncil,
        // A stale registry/env beacon must not be used by the composite.
        BRIDGED_TOKEN_ADDRESS: role(97),
      });
      expect(parameters.TokenBridgeWithBeacon).to.not.have.property("tokenBeacon");
      const { tokenBridge, bridgedTokenBeacon } = await connection.ignition.deploy(TokenBridgeWithBeaconModule, {
        parameters,
      });
      expect(await tokenBridge.read.tokenBeacon()).to.equal(bridgedTokenBeacon.address);
      expect(await tokenBridge.read.messageService()).to.equal(l2MessageService);
    });

    it("CustomBridgedToken behind the pinned OZ 4.x proxy", async () => {
      const connection = await getConnection();
      const publicClient = await connection.viem.getPublicClient();
      const parameters = await buildParametersFromEnv(connection, "CustomBridgedToken", {
        CUSTOMTOKENBRIDGE_NAME: "Custom Token",
        CUSTOMTOKENBRIDGE_SYMBOL: "CTK",
        CUSTOMTOKENBRIDGE_DECIMALS: "6",
        CUSTOMTOKENBRIDGE_BRIDGE_ADDRESS: role(80),
      });
      const { customBridgedToken, customBridgedTokenImplementation, customBridgedTokenProxy, proxyAdmin } =
        await connection.ignition.deploy(CustomBridgedTokenModule, { parameters });
      await expectPinnedTransparentProxy(
        publicClient,
        customBridgedTokenProxy.address,
        proxyAdmin.address,
        customBridgedTokenImplementation.address,
      );
      expect(await customBridgedToken.read.name()).to.equal("Custom Token");
      expect(await customBridgedToken.read.decimals()).to.equal(6);
      expect(await customBridgedToken.read.bridge()).to.equal(role(80));
    });
  });

  describe("Generic", () => {
    it("TestERC20 mints TEST_ERC20_INITIAL_SUPPLY ether to the deployer", async () => {
      const connection = await getConnection();
      const [deployer] = await connection.viem.getWalletClients();
      const parameters = await buildParametersFromEnv(connection, "TestERC20", {
        TEST_ERC20_NAME: "Test Token",
        TEST_ERC20_SYMBOL: "TTK",
        TEST_ERC20_INITIAL_SUPPLY: "1000",
      });
      const { testERC20 } = await connection.ignition.deploy(TestERC20Module, { parameters });
      expect(await testERC20.read.totalSupply()).to.equal(parseEther("1000"));
      expect(await testERC20.read.balanceOf([deployer.account.address])).to.equal(parseEther("1000"));
    });

    it("PaymentSplitterWrapper", async () => {
      const connection = await getConnection();
      const parameters = await buildParametersFromEnv(connection, "PaymentSplitterWrapper", {
        PAYMENT_SPLITTER_PAYEES: `${role(90)}, ${role(91)}`,
        PAYMENT_SPLITTER_SHARES: "70,30",
      });
      const { paymentSplitterWrapper } = await connection.ignition.deploy(PaymentSplitterWrapperModule, {
        parameters,
      });
      expect(await paymentSplitterWrapper.read.shares([role(90)])).to.equal(70n);
      expect(await paymentSplitterWrapper.read.totalShares()).to.equal(100n);
    });
  });
});
