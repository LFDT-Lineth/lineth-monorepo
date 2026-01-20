import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expectRevertWithCustomError, getAccountsFixture } from "../../common/helpers";
import { deployLidoStVaultYieldProviderFactory } from "../helpers";
import {
  LidoStVaultYieldProviderFactory,
  MockLineaRollup,
  MockSTETH,
  MockVaultHub,
  MockVaultFactory,
  TestYieldManager,
  ValidatorContainerProofVerifier,
} from "contracts/typechain-types";
import { expect } from "chai";
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { ethers } from "hardhat";
import { ZeroAddress } from "ethers";

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
    it("Should revert if 0 address provided for _l1MessageService", async () => {
      const contractFactory = await ethers.getContractFactory("LidoStVaultYieldProviderFactory");
      const call = contractFactory.deploy(
        ZeroAddress,
        yieldManagerAddress,
        vaultHubAddress,
        vaultFactoryAddress,
        stethAddress,
        verifierAddress,
      );
      await expectRevertWithCustomError(contractFactory, call, "ZeroAddressNotAllowed");
    });
    it("Should revert if 0 address provided for _yieldManager", async () => {
      const contractFactory = await ethers.getContractFactory("LidoStVaultYieldProviderFactory");
      const call = contractFactory
        .connect(nativeYieldOperator)
        .deploy(
          l1MessageServiceAddress,
          ZeroAddress,
          vaultHubAddress,
          vaultFactoryAddress,
          stethAddress,
          verifierAddress,
        );
      await expectRevertWithCustomError(contractFactory, call, "ZeroAddressNotAllowed");
    });
    it("Should revert if 0 address provided for _vaultHub", async () => {
      const contractFactory = await ethers.getContractFactory("LidoStVaultYieldProviderFactory");
      const call = contractFactory.deploy(
        l1MessageServiceAddress,
        yieldManagerAddress,
        ZeroAddress,
        vaultFactoryAddress,
        stethAddress,
        verifierAddress,
      );
      await expectRevertWithCustomError(contractFactory, call, "ZeroAddressNotAllowed");
    });
    it("Should revert if 0 address provided for _vaultFactory", async () => {
      const contractFactory = await ethers.getContractFactory("LidoStVaultYieldProviderFactory");
      const call = contractFactory.deploy(
        l1MessageServiceAddress,
        yieldManagerAddress,
        vaultHubAddress,
        ZeroAddress,
        stethAddress,
        verifierAddress,
      );
      await expectRevertWithCustomError(contractFactory, call, "ZeroAddressNotAllowed");
    });
    it("Should revert if 0 address provided for _steth", async () => {
      const contractFactory = await ethers.getContractFactory("LidoStVaultYieldProviderFactory");
      const call = contractFactory.deploy(
        l1MessageServiceAddress,
        yieldManagerAddress,
        vaultHubAddress,
        vaultFactoryAddress,
        ZeroAddress,
        verifierAddress,
      );
      await expectRevertWithCustomError(contractFactory, call, "ZeroAddressNotAllowed");
    });
    it("Should revert if 0 address provided for _validatorContainerProofVerifier", async () => {
      const contractFactory = await ethers.getContractFactory("LidoStVaultYieldProviderFactory");
      const call = contractFactory.deploy(
        l1MessageServiceAddress,
        yieldManagerAddress,
        vaultHubAddress,
        vaultFactoryAddress,
        stethAddress,
        ZeroAddress,
      );
      await expectRevertWithCustomError(contractFactory, call, "ZeroAddressNotAllowed");
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
      expect(call.deploymentTransaction)
        .to.emit(lidoStVaultYieldProviderFactory, "LidoStVaultYieldProviderFactoryDeployed")
        .withArgs(
          l1MessageServiceAddress,
          yieldManagerAddress,
          vaultHubAddress,
          vaultFactoryAddress,
          stethAddress,
          verifierAddress,
        );
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
      await expect(call)
        .to.emit(lidoStVaultYieldProviderFactory, "LidoStVaultYieldProviderCreated")
        .withArgs(yieldProviderAddress);
    });
  });
});
