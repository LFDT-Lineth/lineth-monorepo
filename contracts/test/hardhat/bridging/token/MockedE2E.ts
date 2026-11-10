import { expect } from "chai";
import { network } from "hardhat";
import { describe, it } from "node:test";
import { type Address, encodeAbiParameters, maxUint256, parseAbiParameters, zeroAddress } from "viem";

import { deployTokenBridgeWithMockMessaging } from "../../../../scripts/tokenBridge/test/deployTokenBridges.js";
import { deployTokens } from "../../../../scripts/tokenBridge/test/deployTokens.js";
import {
  COMPLETE_TOKEN_BRIDGING_PAUSE_TYPE,
  INITIATE_TOKEN_BRIDGING_PAUSE_TYPE,
  SET_MESSAGE_SERVICE_ROLE,
} from "../../common/constants/index.js";
import {
  TOKEN_BRIDGE_RESERVED_STATUS,
  TOKEN_BRIDGE_NATIVE_STATUS,
  TOKEN_BRIDGE_DEPLOYED_STATUS,
} from "../../common/constants/index.js";
import { deployFromFactory } from "../../common/deployment.js";
import {
  buildAccessErrorMessage,
  expectRevertWhenPaused,
  getTestAccounts,
  expectRevertWithReason,
  expectRevertWithCustomError,
  expectEvent,
} from "../../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

const initialUserBalance = BigInt(10 ** 9);
// Use shared constants from common/constants/general.ts
const RESERVED_STATUS = TOKEN_BRIDGE_RESERVED_STATUS;
const NATIVE_STATUS = TOKEN_BRIDGE_NATIVE_STATUS;
const DEPLOYED_STATUS = TOKEN_BRIDGE_DEPLOYED_STATUS;
const mockName = "L1 DAI";
const mockSymbol = "L1DAI";
const mockDecimals = 18;

async function loadFixture<T>(fixture: () => Promise<T>): Promise<T> {
  const { networkHelpers } = await network.getOrCreate();
  return networkHelpers.loadFixture(fixture);
}

async function getContractAt<ContractName extends string>(
  contractName: ContractName,
  address: Address,
): Promise<ContractReturnType<ContractName>> {
  const { viem } = await network.getOrCreate();
  return viem.getContractAt(contractName, address);
}

describe("Mocked E2E tests", function () {
  async function deployContractsFixture() {
    const [deployer, user] = await getTestAccounts();

    // Deploy and configure bridges
    const deploymentFixture = await deployTokenBridgeWithMockMessaging();

    // Deploy tokens
    const tokens = await deployTokens();

    // Mint tokens for user and approve bridge
    for (const name in tokens) {
      const token = tokens[name];
      await token.write.mint([user.address, initialUserBalance]);

      let bridgeAddress: Address | undefined;
      if ((await token.read.name()).includes("L1")) {
        bridgeAddress = deploymentFixture.l1TokenBridge.address;
      }
      if ((await token.read.name()).includes("L2")) {
        bridgeAddress = deploymentFixture.l2TokenBridge.address;
      }

      if (bridgeAddress === undefined) {
        throw new Error(`No bridge for token ${name}`);
      }

      await token.write.approve([bridgeAddress, maxUint256], { account: user.account });
    }

    const encodedData = encodeAbiParameters(parseAbiParameters("string, string, uint8"), [
      mockName,
      mockSymbol,
      mockDecimals,
    ]);

    return { deployer, user, ...deploymentFixture, tokens, encodedData };
  }

  describe("Bridging", function () {
    it("Should have the correct balance and totalSupply on both chains after bridging", async function () {
      const {
        user,
        l1TokenBridge,
        l2TokenBridge,
        tokens: { L1DAI },
        chainIds,
      } = await loadFixture(deployContractsFixture);
      const bridgeAmount = 100n;

      await l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });

      const userBalance = initialUserBalance - BigInt(bridgeAmount);
      expect(await L1DAI.read.balanceOf([user.address])).to.be.equal(userBalance);
      expect(await L1DAI.read.balanceOf([l1TokenBridge.address])).to.be.equal(bridgeAmount);

      const l2TokenAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address]);
      const l2Token = await getContractAt("BridgedToken", l2TokenAddress);

      expect(await l2Token.read.balanceOf([user.address])).to.be.equal(bridgeAmount);
      expect(await l2Token.read.totalSupply()).to.be.equal(bridgeAmount);
    });

    it("Should not allow to initiate bridging if paused", async function () {
      const {
        user,
        deployer,
        l2TokenBridge,
        tokens: { L1DAI },
      } = await loadFixture(deployContractsFixture);
      const bridgeAmount = 100n;

      await l2TokenBridge.write.pauseByType([INITIATE_TOKEN_BRIDGING_PAUSE_TYPE], { account: deployer.account });
      await expectRevertWhenPaused(
        l2TokenBridge,
        l2TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account }),
        INITIATE_TOKEN_BRIDGING_PAUSE_TYPE,
      );
    });

    it("Should not allow complete bridging if paused", async function () {
      const {
        user,
        deployer,
        l1TokenBridge,
        l2TokenBridge,
        tokens: { L1DAI },
      } = await loadFixture(deployContractsFixture);
      const bridgeAmount = 100n;

      await l2TokenBridge.write.pauseByType([COMPLETE_TOKEN_BRIDGING_PAUSE_TYPE], { account: deployer.account });
      await expectRevertWhenPaused(
        l2TokenBridge,
        l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account }),
        COMPLETE_TOKEN_BRIDGING_PAUSE_TYPE,
      );
    });

    it("Should have the correct balance and totalSupply on both chains after back and forth bridging", async function () {
      const {
        user,
        l1TokenBridge,
        l2TokenBridge,
        tokens: { L1DAI },
        chainIds,
      } = await loadFixture(deployContractsFixture);
      const firstBridgeAmount = 100n;
      const secondBridgeAmount = 30n;
      const netBridgedAmount = firstBridgeAmount - secondBridgeAmount;

      await l1TokenBridge.write.bridgeToken([L1DAI.address, firstBridgeAmount, user.address], {
        account: user.account,
      });

      const l2TokenAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address]);
      const l2Token = await getContractAt("BridgedToken", l2TokenAddress);

      expect(await l2Token.read.allowance([user.address, l2TokenBridge.address])).to.be.equal(0n);

      await expectRevertWithReason(
        l2TokenBridge.write.bridgeToken([l2TokenAddress, secondBridgeAmount, user.address], { account: user.account }),
        "ERC20: insufficient allowance",
      );

      await l2Token.write.approve([l2TokenBridge.address, secondBridgeAmount], { account: user.account });
      await l2TokenBridge.write.bridgeToken([l2TokenAddress, secondBridgeAmount, user.address], {
        account: user.account,
      });

      const userBalance = initialUserBalance - BigInt(netBridgedAmount);
      expect(await L1DAI.read.balanceOf([user.address])).to.be.equal(userBalance);
      expect(await L1DAI.read.balanceOf([l1TokenBridge.address])).to.be.equal(netBridgedAmount);
      expect(await l2Token.read.balanceOf([user.address])).to.be.equal(netBridgedAmount);
      expect(await l2Token.read.totalSupply()).to.be.equal(netBridgedAmount);
    });

    it("Should support fee tokens and set balances correctly", async function () {
      const { user, l1TokenBridge, l2TokenBridge, chainIds } = await loadFixture(deployContractsFixture);

      const feeToken = await deployFromFactory("ERC20Fees", ["Token with fee", "FEE", 200]); // 2% fee

      // Mint tokens for user and approve bridge
      await feeToken.write.mint([user.address, initialUserBalance]);
      await feeToken.write.approve([l1TokenBridge.address, maxUint256], { account: user.account });

      // Transfer some tokens from the user to the contract
      const amountToTransfer = 100n;
      await l1TokenBridge.write.bridgeToken([feeToken.address, amountToTransfer, user.address], {
        account: user.account,
      });

      // Calculate actual transfered amount
      const burnAmount = (amountToTransfer * 2n) / 100n; // 2% fee
      const transferredAmount = amountToTransfer - burnAmount;

      // Get bridged token address
      const l2FeeTokenAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), feeToken.address]);
      const l2FeeToken = await getContractAt("ERC20Fees", l2FeeTokenAddress);

      // Check balances and total supply
      expect(await feeToken.read.balanceOf([user.address])).to.be.equal(initialUserBalance - amountToTransfer);
      expect(await feeToken.read.balanceOf([l1TokenBridge.address])).to.be.equal(transferredAmount);
      expect(await l2FeeToken.read.balanceOf([user.address])).to.be.equal(transferredAmount);
      expect(await l2FeeToken.read.totalSupply()).to.be.equal(transferredAmount);
    });

    it("Should not be able to bridge 0 tokens", async function () {
      const {
        user,
        l1TokenBridge,
        tokens: { L1DAI },
      } = await loadFixture(deployContractsFixture);
      const bridgeAmount = 0n;

      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account }),
        "ZeroAmountNotAllowed",
        [bridgeAmount],
      );
    });

    it("Should not be able to bridge if token is the 0 address", async function () {
      const { user, l1TokenBridge } = await loadFixture(deployContractsFixture);
      const bridgeAmount = 10n;

      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.bridgeToken([zeroAddress, bridgeAmount, user.address], { account: user.account }),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should create bridged token on the targeted layer even if both native tokens on both layers have the same address and are bridged at the same time", async function () {
      const {
        user,
        l1TokenBridge,
        l2TokenBridge,
        messageService,
        tokens: { L1DAI },
        chainIds,
      } = await loadFixture(deployContractsFixture);
      const balanceTokenUser = await L1DAI.read.balanceOf([user.address]);
      const bridgeAmount = 10n;

      // Deploy another message service that will not send the message along to the other
      // layer so that we can simulate the same state we could get if 2 tokens are sent
      // at the same time, meaning that the function bridgeToken() has been called
      // on both layers while the completeBridging has not been called yet
      const mockMessageServiceV2 = await deployFromFactory("MockMessageServiceV2");

      await L1DAI.write.approve([l2TokenBridge.address, maxUint256], { account: user.account });
      await l2TokenBridge.write.setMessageService([mockMessageServiceV2.address]);
      await l2TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });

      expect(await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[1]), L1DAI.address])).to.be.equal(
        NATIVE_STATUS,
      );

      await l1TokenBridge.write.setMessageService([mockMessageServiceV2.address]);
      await l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });

      expect(await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address])).to.be.equal(
        NATIVE_STATUS,
      );

      // We check that the brigedToken have not been created yet
      expect(await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address])).to.be.equal(
        zeroAddress,
      );
      expect(await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[1]), L1DAI.address])).to.be.equal(
        zeroAddress,
      );

      // Here we are in the situation where both tokens are native to the both chains but neither
      // of them have arrived to their destination yet, completeBridging() has not been called yet

      // We reassign the normal message service
      await l2TokenBridge.write.setMessageService([messageService.address]);
      await l1TokenBridge.write.setMessageService([messageService.address]);

      // This initial amount has to be ignored for the test since in reality
      // the first message would go through the message service and funds would not be stuck
      const initialBalanceTokenL1Bridge = await L1DAI.read.balanceOf([l1TokenBridge.address]);
      const initialBalanceTokenL2Bridge = await L1DAI.read.balanceOf([l2TokenBridge.address]);

      expect(initialBalanceTokenL1Bridge).to.be.equal(bridgeAmount);
      expect(initialBalanceTokenL2Bridge).to.be.equal(bridgeAmount);

      // Now we try to bridge with the right message service
      // It should create the bridgedToken on the other layer for each native token
      await l2TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });

      // BridgedToken on L1 is created
      const bridgedTokenL1Addr = await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[1]), L1DAI.address]);
      expect(bridgedTokenL1Addr).to.not.equal(zeroAddress);
      const bridgedTokenL1 = await getContractAt("MockERC20MintBurn", bridgedTokenL1Addr);
      expect(await L1DAI.read.balanceOf([l2TokenBridge.address])).to.be.equal(bridgeAmount * 2n);
      expect(await bridgedTokenL1.read.balanceOf([user.address])).to.be.equal(bridgeAmount);

      await l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });

      // BridgedToken on L2 is created
      const bridgedTokenL2Addr = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address]);
      expect(bridgedTokenL2Addr).to.not.equal(zeroAddress);
      const bridgedTokenL2 = await getContractAt("MockERC20MintBurn", bridgedTokenL2Addr);
      expect(await L1DAI.read.balanceOf([l1TokenBridge.address])).to.be.equal(bridgeAmount * 2n);
      expect(await bridgedTokenL2.read.balanceOf([user.address])).to.be.equal(bridgeAmount);

      // At this point the user bridged DAI 4 times so he should have 4 times
      // the bridge amount less in his balance
      expect(await L1DAI.read.balanceOf([user.address])).to.be.equal(balanceTokenUser - BigInt(bridgeAmount) * 4n);

      // The user has received the minted bridgedToken on both chains
      // Now we get back our native tokens on both chains by bridging the bridged tokens that the user has received
      await bridgedTokenL1.write.approve([l1TokenBridge.address, maxUint256], { account: user.account });
      await bridgedTokenL2.write.approve([l2TokenBridge.address, maxUint256], { account: user.account });

      await l1TokenBridge.write.bridgeToken([bridgedTokenL1.address, bridgeAmount, user.address], {
        account: user.account,
      });
      await l2TokenBridge.write.bridgeToken([bridgedTokenL2.address, bridgeAmount, user.address], {
        account: user.account,
      });

      expect(await bridgedTokenL1.read.balanceOf([user.address])).to.be.equal(0n);
      expect(await bridgedTokenL2.read.balanceOf([user.address])).to.be.equal(0n);
      expect(await L1DAI.read.balanceOf([user.address])).to.be.equal(balanceTokenUser - BigInt(bridgeAmount) * 2n);
    });

    describe("BridgedToken deployment", function () {
      it("Should return NO_NAME and NO_SYMBOL when the token does not have them.", async function () {
        const { user, l1TokenBridge, l2TokenBridge, chainIds } = await loadFixture(deployContractsFixture);

        const noNameToken = await deployFromFactory("MockERC20NoNameMintBurn");
        await noNameToken.write.mint([user.address, initialUserBalance]);
        await noNameToken.write.approve([l1TokenBridge.address, maxUint256], { account: user.account });

        const bridgeAmount = 100n;

        await l1TokenBridge.write.bridgeToken([noNameToken.address, bridgeAmount, user.address], {
          account: user.account,
        });

        const l2TokenAddress = await l2TokenBridge.read.nativeToBridgedToken([
          BigInt(chainIds[0]),
          noNameToken.address,
        ]);
        const l2Token = await getContractAt("BridgedToken", l2TokenAddress);

        await l2Token.write.approve([l2TokenBridge.address, bridgeAmount], { account: user.account });
        await l2TokenBridge.write.bridgeToken([l2TokenAddress, bridgeAmount, user.address], { account: user.account });

        expect(await l2Token.read.name()).to.be.equal("NO_NAME");
        expect(await l2Token.read.symbol()).to.be.equal("NO_SYMBOL");
      });
      it("Should return UNKNOWN or create a byte array for weird name and symbol.", async function () {
        const { user, l1TokenBridge, l2TokenBridge, chainIds } = await loadFixture(deployContractsFixture);

        const noNameToken = await deployFromFactory("MockERC20WeirdNameSymbol");
        await noNameToken.write.mint([user.address, initialUserBalance]);
        await noNameToken.write.approve([l1TokenBridge.address, maxUint256], { account: user.account });

        const bridgeAmount = 100n;

        await l1TokenBridge.write.bridgeToken([noNameToken.address, bridgeAmount, user.address], {
          account: user.account,
        });

        const l2TokenAddress = await l2TokenBridge.read.nativeToBridgedToken([
          BigInt(chainIds[0]),
          noNameToken.address,
        ]);
        const l2Token = await getContractAt("BridgedToken", l2TokenAddress);

        await l2Token.write.approve([l2TokenBridge.address, bridgeAmount], { account: user.account });
        await l2TokenBridge.write.bridgeToken([l2TokenAddress, bridgeAmount, user.address], { account: user.account });

        expect(await l2Token.read.name()).to.be.equal("NOT_VALID_ENCODING");
        expect(await l2Token.read.symbol()).to.be.equal("\u0001");
      });

      it("Should revert if decimals are unknown.", async function () {
        const { user, l1TokenBridge } = await loadFixture(deployContractsFixture);

        const noDecimalToken = await deployFromFactory("ERC20UnknownDecimals", ["Token with fee", "FEE"]);
        const bridgeAmount = 1000n;

        await noDecimalToken.write.mint([user.address, bridgeAmount]);
        await noDecimalToken.write.approve([l1TokenBridge.address, bridgeAmount], { account: user.account });

        await expectRevertWithCustomError(
          l1TokenBridge,
          l1TokenBridge.write.bridgeToken([noDecimalToken.address, bridgeAmount, user.address], {
            account: user.account,
          }),
          "DecimalsAreUnknown",
          [noDecimalToken.address],
        );
      });

      it("Should not revert if a token being bridged exists as a native token on the other layer", async function () {
        const {
          user,
          l1TokenBridge,
          l2TokenBridge,
          tokens: { L1DAI },
        } = await loadFixture(deployContractsFixture);

        const bridgeAmount = 100n;
        await l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });

        await L1DAI.write.approve([l2TokenBridge.address, bridgeAmount], { account: user.account });
        await l2TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });
      });
    });
  });

  describe("setCustomContract", function () {
    it("Should mint and burn correctly from a target contract", async function () {
      const {
        user,
        l1TokenBridge,
        l2TokenBridge,
        tokens: { L1USDT },
      } = await loadFixture(deployContractsFixture);
      const amountBridged = 100n;
      const amountBridgedBack = 100n;

      // Deploy custom contract
      const customContract = await deployFromFactory("MockERC20MintBurn", ["CustomContract", "CC"]);

      // Set custom contract
      await l2TokenBridge.write.setCustomContract([L1USDT.address, customContract.address]);

      // Bridge token (allowance has been set in the fixture)
      await l1TokenBridge.write.bridgeToken([L1USDT.address, amountBridged, user.address], { account: user.account });

      expect(await L1USDT.read.balanceOf([l1TokenBridge.address])).to.be.equal(amountBridged);
      expect(await customContract.read.balanceOf([user.address])).to.be.equal(amountBridged);
      expect(await customContract.read.totalSupply()).to.be.equal(amountBridged);

      // Bridge back
      const USDTBalanceBefore = await L1USDT.read.balanceOf([user.address]);
      await l2TokenBridge.write.bridgeToken([customContract.address, amountBridgedBack, user.address], {
        account: user.account,
      });
      const USDTBalanceAfter = await L1USDT.read.balanceOf([user.address]);

      expect(await customContract.read.balanceOf([user.address])).to.be.equal(amountBridged - amountBridgedBack);
      expect(await customContract.read.totalSupply()).to.be.equal(amountBridged - amountBridgedBack);
      expect(USDTBalanceAfter - USDTBalanceBefore).to.be.equal(amountBridgedBack);
    });

    it("Should not be able to set a custom contract that is already a bridged token contract or one of the STATUS", async function () {
      const {
        user,
        l1TokenBridge,
        l2TokenBridge,
        tokens: { L1USDT, L2UNI, L2SHIBA },
        chainIds,
      } = await loadFixture(deployContractsFixture);
      const amountBridged = 100n;

      await l1TokenBridge.write.bridgeToken([L1USDT.address, amountBridged, user.address], { account: user.account });

      const brigedToNativeContract = await l2TokenBridge.read.nativeToBridgedToken([
        BigInt(chainIds[0]),
        L1USDT.address,
      ]);

      // Try to set custom contract with a bridgedTokenContract, should revert
      await expectRevertWithCustomError(
        l2TokenBridge,
        l2TokenBridge.write.setCustomContract([L2UNI.address, brigedToNativeContract]),
        "AlreadyBrigedToNativeTokenSet",
        [brigedToNativeContract],
      );

      const randomTokenAddress = L2SHIBA.address;

      await expectRevertWithCustomError(
        l2TokenBridge,
        l2TokenBridge.write.setCustomContract([L1USDT.address, randomTokenAddress]),
        "NativeToBridgedTokenAlreadySet",
        [L1USDT.address],
      );

      await expectRevertWithCustomError(
        l2TokenBridge,
        l2TokenBridge.write.setCustomContract([L2UNI.address, RESERVED_STATUS]),
        "StatusAddressNotAllowed",
        [RESERVED_STATUS],
      );

      await expectRevertWithCustomError(
        l2TokenBridge,
        l2TokenBridge.write.setCustomContract([L2UNI.address, NATIVE_STATUS]),
        "StatusAddressNotAllowed",
        [NATIVE_STATUS],
      );

      await expectRevertWithCustomError(
        l2TokenBridge,
        l2TokenBridge.write.setCustomContract([L2UNI.address, DEPLOYED_STATUS]),
        "StatusAddressNotAllowed",
        [DEPLOYED_STATUS],
      );
    });
  });

  // NOTE: Reserved token tests removed - they're duplicated in TokenBridge.ts
  // with more comprehensive coverage (8 tests vs 3 here). See TEST_CONSOLIDATION_CHECKLIST.md

  describe("Modifying Message Service", function () {
    it("Should be able to modify the Message Service and bridge properly", async function () {
      const {
        deployer,
        user,
        l1TokenBridge,
        l2TokenBridge,
        tokens: { L1DAI },
        chainIds,
      } = await loadFixture(deployContractsFixture);
      // Deploy new Message Service
      const messageService = await deployFromFactory("MockMessageService");

      await l1TokenBridge.write.setMessageService([messageService.address], { account: deployer.account });
      await l2TokenBridge.write.setMessageService([messageService.address], { account: deployer.account });

      const bridgeAmount = 100n;

      await l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });

      const userBalance = initialUserBalance - BigInt(bridgeAmount);
      expect(await L1DAI.read.balanceOf([user.address])).to.be.equal(userBalance);
      expect(await L1DAI.read.balanceOf([l1TokenBridge.address])).to.be.equal(bridgeAmount);

      const l2TokenAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address]);
      const l2Token = await getContractAt("BridgedToken", l2TokenAddress);

      expect(await l2Token.read.balanceOf([user.address])).to.be.equal(bridgeAmount);
      expect(await l2Token.read.totalSupply()).to.be.equal(bridgeAmount);
    });

    it("Should no be able to set Message Service by non-owner caller", async function () {
      const { user, l1TokenBridge } = await loadFixture(deployContractsFixture);
      // Deploy new Message Service
      const messageService = await deployFromFactory("MockMessageService");

      await expectRevertWithReason(
        l1TokenBridge.write.setMessageService([messageService.address], { account: user.account }),
        buildAccessErrorMessage(user, SET_MESSAGE_SERVICE_ROLE),
      );
    });

    it("Should emit en event when updating the Message Service", async function () {
      const { l1TokenBridge, deployer } = await loadFixture(deployContractsFixture);
      // Deploy new Message Service
      const messageService = await deployFromFactory("MockMessageService");

      const oldMessageService = await l1TokenBridge.read.messageService();

      await expectEvent(
        l1TokenBridge,
        l1TokenBridge.write.setMessageService([messageService.address]),
        "MessageServiceUpdated",
        [messageService.address, oldMessageService, deployer.address],
      );
    });
  });

  describe("Token deployment status", function () {
    it("Should set correct deployment status on both chain", async function () {
      const {
        user,
        l1TokenBridge,
        l2TokenBridge,
        tokens: { L1DAI, L1USDT },
        chainIds,
      } = await loadFixture(deployContractsFixture);

      const bridgeAmount = 100n;
      await l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });
      await l1TokenBridge.write.bridgeToken([L1USDT.address, bridgeAmount, user.address], { account: user.account });

      expect(await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address])).to.be.equal(
        NATIVE_STATUS,
      );
      expect(await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1USDT.address])).to.be.equal(
        NATIVE_STATUS,
      );

      const L2DAIBridgedAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address]);
      const L2USDTBridgedAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1USDT.address]);

      await l2TokenBridge.write.confirmDeployment([[L2DAIBridgedAddress, L2USDTBridgedAddress]]);

      expect(await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address])).to.be.equal(
        DEPLOYED_STATUS,
      );
      expect(await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1USDT.address])).to.be.equal(
        DEPLOYED_STATUS,
      );

      // Should not revert
      await l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });

      // @TODO: check metadata are not sent for new L1DAI tx
    });

    it("Should revert confirm deployment if token list is empty", async function () {
      const { l2TokenBridge } = await loadFixture(deployContractsFixture);

      await expectRevertWithCustomError(l2TokenBridge, l2TokenBridge.write.confirmDeployment([[]]), "TokenListEmpty");
    });

    it("Should revert when trying to change the status of a token that has not been deployed yet", async function () {
      const {
        user,
        l1TokenBridge,
        l2TokenBridge,
        tokens: { L1DAI, L1USDT },
        chainIds,
      } = await loadFixture(deployContractsFixture);

      const bridgeAmount = 100n;
      await l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });

      const L2DAIBridgedAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address]);

      await expectRevertWithCustomError(
        l2TokenBridge,
        l2TokenBridge.write.confirmDeployment([[L2DAIBridgedAddress, L1USDT.address]]),
        "TokenNotDeployed",
        [L1USDT.address],
      );
    });

    it("Should be able to bridge back to the original layer if that token is marked as DEPLOYED", async function () {
      const {
        user,
        l1TokenBridge,
        l2TokenBridge,
        tokens: { L1DAI },
        chainIds,
      } = await loadFixture(deployContractsFixture);
      const bridgeAmount = 10n;

      const initialAmount = await L1DAI.read.balanceOf([user.address]);
      await l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });
      const L2DAIBridgedAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address]);
      await l2TokenBridge.write.confirmDeployment([[L2DAIBridgedAddress]]);

      expect(await L1DAI.read.balanceOf([user.address])).to.be.equal(initialAmount - BigInt(bridgeAmount));
      expect(
        await (await getContractAt("BridgedToken", L2DAIBridgedAddress)).read.balanceOf([user.address]),
      ).to.be.equal(bridgeAmount);
      expect(await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address])).to.be.equal(
        DEPLOYED_STATUS,
      );

      await (
        await getContractAt("BridgedToken", L2DAIBridgedAddress)
      ).write.approve([l2TokenBridge.address, bridgeAmount], { account: user.account });
      await l2TokenBridge.write.bridgeToken([L2DAIBridgedAddress, bridgeAmount, user.address], {
        account: user.account,
      });

      expect(await L1DAI.read.balanceOf([user.address])).to.be.equal(initialAmount);
    });
  });
});
