import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import {
  LidoStVaultYieldProviderFactory,
  MockLineaRollup,
  MockSTETH,
  MockVaultHub,
  MockVaultFactory,
  TestYieldManager,
  ValidatorContainerProofVerifier,
} from "contracts/typechain-types";
import { ZeroAddress } from "ethers";
import { ethers } from "hardhat";

import { getAccountsFixture, expectZeroAddressRevert, expectEvent } from "../../common/helpers";
import { deployLidoStVaultYieldProviderFactory } from "../helpers";

describe("LidoStVaultYieldProviderFactory", () => {
  let lidoStVaultYieldProviderFactory: LidoStVaultYieldProviderFactory;
  let mockVaultHub: MockVaultHub;
  let mockVaultFactory: MockVaultFactory;
  let mockSTETH: MockSTETH;
  let mockLineaRollup: MockLineaRollup;
  let yieldManager: TestYieldManager;
  let verifier: ValidatorContainerProofVerifier;

  let nativeYieldOperator: SignerWithAddress;

  let l1MessageServiceAddress: string;
  let yieldManagerAddress: string;
  let vaultHubAddress: string;
  let vaultFactoryAddress: string;
  let stethAddress: string;
  let verifierAddress: string;

  before(async () => {
    ({ nativeYieldOperator } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({
      lidoStVaultYieldProviderFactory,
      mockVaultHub,
      mockVaultFactory,
      mockLineaRollup,
      yieldManager,
      mockSTETH,
      verifier,
      verifierAddress,
    } = await loadFixture(deployLidoStVaultYieldProviderFactory));
    l1MessageServiceAddress = await mockLineaRollup.getAddress();
    yieldManagerAddress = await yieldManager.getAddress();
    vaultHubAddress = await mockVaultHub.getAddress();
    vaultFactoryAddress = await mockVaultFactory.getAddress();
    stethAddress = await mockSTETH.getAddress();
    verifierAddress = await verifier.getAddress();
  });

  describe("Constructor", () => {
    // Configuration for constructor arguments
    interface ConstructorConfig {
      l1MessageService: string;
      yieldManager: string;
      vaultHub: string;
      vaultFactory: string;
      steth: string;
      validatorContainerProofVerifier: string;
    }

    // Convert config to args tuple
    const toArgs = (config: ConstructorConfig): [string, string, string, string, string, string] => [
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
        const contractFactory = await ethers.getContractFactory("LidoStVaultYieldProviderFactory");

        const defaultConfig: ConstructorConfig = {
          l1MessageService: l1MessageServiceAddress,
          yieldManager: yieldManagerAddress,
          vaultHub: vaultHubAddress,
          vaultFactory: vaultFactoryAddress,
          steth: stethAddress,
          validatorContainerProofVerifier: verifierAddress,
        };

        const args = toArgs({ ...defaultConfig, [field]: ZeroAddress });

        await expectZeroAddressRevert({
          contract: contractFactory,
          deployOrInitCall: contractFactory.deploy(...args),
        });
      });
    });

    it("Should succeed and emit the correct event", async () => {
      const contractFactory = await ethers.getContractFactory("LidoStVaultYieldProviderFactory");
      const call = await contractFactory.deploy(
        l1MessageServiceAddress,
        yieldManagerAddress,
        vaultHubAddress,
        vaultFactoryAddress,
        stethAddress,
        verifierAddress,
      );
      await expectEvent(call, call.deploymentTransaction()!, "LidoStVaultYieldProviderFactoryDeployed", [
        l1MessageServiceAddress,
        yieldManagerAddress,
        vaultHubAddress,
        vaultFactoryAddress,
        stethAddress,
        verifierAddress,
      ]);
    });
  });

  describe("Immutables", () => {
    it("Should deploy with correct VaultHub address", async () => {
      expect(await lidoStVaultYieldProviderFactory.VAULT_HUB()).eq(await mockVaultHub.getAddress());
    });
    it("Should deploy with correct STETH address", async () => {
      expect(await lidoStVaultYieldProviderFactory.STETH()).eq(await mockSTETH.getAddress());
    });
    it("Should deploy with correct L1MessageService address", async () => {
      expect(await lidoStVaultYieldProviderFactory.L1_MESSAGE_SERVICE()).eq(await mockLineaRollup.getAddress());
    });
    it("Should deploy with correct YieldManager address", async () => {
      expect(await lidoStVaultYieldProviderFactory.YIELD_MANAGER()).eq(await yieldManager.getAddress());
    });
    it("Should deploy with correct VaultFactory address", async () => {
      expect(await lidoStVaultYieldProviderFactory.VAULT_FACTORY()).eq(await mockVaultFactory.getAddress());
    });
    it("Should deploy with correct ValidatorContainerProofVerifier address", async () => {
      expect(await lidoStVaultYieldProviderFactory.VALIDATOR_CONTAINER_PROOF_VERIFIER()).eq(verifierAddress);
    });
  });

  describe("createLidoStVaultYieldProvider", () => {
    it("Should successfully create new LidoStVaultYieldProvider and emit expected event", async () => {
      const yieldProviderAddress = await lidoStVaultYieldProviderFactory.createLidoStVaultYieldProvider.staticCall();
      const call = lidoStVaultYieldProviderFactory.connect(nativeYieldOperator).createLidoStVaultYieldProvider();
      await expectEvent(lidoStVaultYieldProviderFactory, call, "LidoStVaultYieldProviderCreated", [
        yieldProviderAddress,
      ]);
    });
  });
});
