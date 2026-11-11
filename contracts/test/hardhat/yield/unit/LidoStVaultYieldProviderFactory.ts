import { expect } from "chai";
import hre, { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { type Address, zeroAddress } from "viem";

import { deployFromFactory } from "../../common/deployment.js";
import {
  type TestAccount,
  getAccountsFixture,
  expectZeroAddressRevert,
  expectEvent,
} from "../../common/helpers/index.js";
import { deployLidoStVaultYieldProviderFactory } from "../helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("LidoStVaultYieldProviderFactory", () => {
  let lidoStVaultYieldProviderFactory: ContractReturnType<"LidoStVaultYieldProviderFactory">;
  let mockVaultHub: ContractReturnType<"MockVaultHub">;
  let mockVaultFactory: ContractReturnType<"MockVaultFactory">;
  let mockSTETH: ContractReturnType<"MockSTETH">;
  let mockLinethRollup: ContractReturnType<"MockLinethRollup">;
  let yieldManager: ContractReturnType<"TestYieldManager">;
  let verifier: ContractReturnType<"ValidatorContainerProofVerifier">;

  let nativeYieldOperator: TestAccount;

  let l1MessageServiceAddress: Address;
  let yieldManagerAddress: Address;
  let vaultHubAddress: Address;
  let vaultFactoryAddress: Address;
  let stethAddress: Address;
  let verifierAddress: Address;

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ nativeYieldOperator } = await networkHelpers.loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({
      lidoStVaultYieldProviderFactory,
      mockVaultHub,
      mockVaultFactory,
      mockLinethRollup,
      yieldManager,
      mockSTETH,
      verifier,
      verifierAddress,
    } = await networkHelpers.loadFixture(deployLidoStVaultYieldProviderFactory));
    l1MessageServiceAddress = mockLinethRollup.address;
    yieldManagerAddress = yieldManager.address;
    vaultHubAddress = mockVaultHub.address;
    vaultFactoryAddress = mockVaultFactory.address;
    stethAddress = mockSTETH.address;
    verifierAddress = verifier.address;
  });

  describe("Constructor", () => {
    // Configuration for constructor arguments
    interface ConstructorConfig {
      l1MessageService: Address;
      yieldManager: Address;
      vaultHub: Address;
      vaultFactory: Address;
      steth: Address;
      validatorContainerProofVerifier: Address;
    }

    // Convert config to args tuple
    const toArgs = (config: ConstructorConfig): [Address, Address, Address, Address, Address, Address] => [
      config.l1MessageService,
      config.yieldManager,
      config.vaultHub,
      config.vaultFactory,
      config.steth,
      config.validatorContainerProofVerifier,
    ];

    // Parameterized zero address validation tests
    const zeroAddressValidationCases: Array<{
      name: string;
      field: keyof ConstructorConfig;
    }> = [
      { name: "_l1MessageService", field: "l1MessageService" },
      { name: "_yieldManager", field: "yieldManager" },
      { name: "_vaultHub", field: "vaultHub" },
      { name: "_vaultFactory", field: "vaultFactory" },
      { name: "_steth", field: "steth" },
      { name: "_validatorContainerProofVerifier", field: "validatorContainerProofVerifier" },
    ];

    zeroAddressValidationCases.forEach(({ name, field }) => {
      it(`Should revert if 0 address provided for ${name}`, async () => {
        const { abi } = await hre.artifacts.readArtifact("LidoStVaultYieldProviderFactory");

        const defaultConfig: ConstructorConfig = {
          l1MessageService: l1MessageServiceAddress,
          yieldManager: yieldManagerAddress,
          vaultHub: vaultHubAddress,
          vaultFactory: vaultFactoryAddress,
          steth: stethAddress,
          validatorContainerProofVerifier: verifierAddress,
        };

        const args = toArgs({ ...defaultConfig, [field]: zeroAddress });

        await expectZeroAddressRevert({
          contract: { abi },
          deployOrInitCall: deployFromFactory("LidoStVaultYieldProviderFactory", args),
        });
      });
    });

    it("Should succeed and emit the correct event", async () => {
      const { viem } = await network.getOrCreate();
      const { contract, deploymentTransaction } = await viem.sendDeploymentTransaction(
        "LidoStVaultYieldProviderFactory",
        [
          l1MessageServiceAddress,
          yieldManagerAddress,
          vaultHubAddress,
          vaultFactoryAddress,
          stethAddress,
          verifierAddress,
        ],
      );
      await expectEvent(
        contract,
        Promise.resolve(deploymentTransaction.hash),
        "LidoStVaultYieldProviderFactoryDeployed",
        [
          l1MessageServiceAddress,
          yieldManagerAddress,
          vaultHubAddress,
          vaultFactoryAddress,
          stethAddress,
          verifierAddress,
        ],
      );
    });
  });

  describe("Immutables", () => {
    it("Should deploy with correct VaultHub address", async () => {
      expect(await lidoStVaultYieldProviderFactory.read.VAULT_HUB()).eq(mockVaultHub.address);
    });
    it("Should deploy with correct STETH address", async () => {
      expect(await lidoStVaultYieldProviderFactory.read.STETH()).eq(mockSTETH.address);
    });
    it("Should deploy with correct L1MessageService address", async () => {
      expect(await lidoStVaultYieldProviderFactory.read.L1_MESSAGE_SERVICE()).eq(mockLinethRollup.address);
    });
    it("Should deploy with correct YieldManager address", async () => {
      expect(await lidoStVaultYieldProviderFactory.read.YIELD_MANAGER()).eq(yieldManager.address);
    });
    it("Should deploy with correct VaultFactory address", async () => {
      expect(await lidoStVaultYieldProviderFactory.read.VAULT_FACTORY()).eq(mockVaultFactory.address);
    });
    it("Should deploy with correct ValidatorContainerProofVerifier address", async () => {
      expect(await lidoStVaultYieldProviderFactory.read.VALIDATOR_CONTAINER_PROOF_VERIFIER()).eq(verifierAddress);
    });
  });

  describe("createLidoStVaultYieldProvider", () => {
    it("Should successfully create new LidoStVaultYieldProvider and emit expected event", async () => {
      const { result: yieldProviderAddress } =
        await lidoStVaultYieldProviderFactory.simulate.createLidoStVaultYieldProvider();
      const call = lidoStVaultYieldProviderFactory.write.createLidoStVaultYieldProvider({
        account: nativeYieldOperator.account,
      });
      await expectEvent(lidoStVaultYieldProviderFactory, call, "LidoStVaultYieldProviderCreated", [
        yieldProviderAddress,
      ]);
    });
  });
});
