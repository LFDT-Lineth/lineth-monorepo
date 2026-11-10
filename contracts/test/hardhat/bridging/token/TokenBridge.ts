import { expect } from "chai";
import hre, { network } from "hardhat";
import { describe, it } from "node:test";
import {
  type Address,
  bytesToHex,
  concat,
  encodeAbiParameters,
  encodeFunctionData,
  getAddress,
  maxUint256,
  pad,
  parseAbiParameters,
  stringToHex,
} from "viem";
import { generatePrivateKey, privateKeyToAccount } from "viem/accounts";

import { getPermitData } from "./helpers/permitHelper.js";
import { SupportedChainIds } from "../../../../common/supportedNetworks.js";
import { deployTokenBridgeWithMockMessaging } from "../../../../scripts/tokenBridge/test/deployTokenBridges.js";
import { deployTokens } from "../../../../scripts/tokenBridge/test/deployTokens.js";
import {
  ADDRESS_ZERO,
  COMPLETE_TOKEN_BRIDGING_PAUSE_TYPE,
  INITIATE_TOKEN_BRIDGING_PAUSE_TYPE,
  PAUSE_INITIATE_TOKEN_BRIDGING_ROLE,
  REMOVE_RESERVED_TOKEN_ROLE,
  SET_CUSTOM_CONTRACT_ROLE,
  SET_MESSAGE_SERVICE_ROLE,
  SET_RESERVED_TOKEN_ROLE,
  UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE,
  pauseTypeRoles,
  unpauseTypeRoles,
} from "../../common/constants/index.js";
import { TOKEN_BRIDGE_RESERVED_STATUS } from "../../common/constants/index.js";
import { deployFromFactory, deployUpgradableFromFactory, getUpgrades } from "../../common/deployment.js";
import {
  expectAccessControlRevert,
  expectEventDirectFromReceiptData,
  expectNoEvent,
  expectRevertWhenPaused,
  expectPaused,
  expectNotPaused,
  generateRandomBytes,
  getTestAccounts,
  serializeTokenBridgeInitData,
  expectRevertWithCustomError,
  expectEvent,
  expectRevertWithReason,
} from "../../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

const initialUserBalance = BigInt(10 ** 9);
const mockName = "L1 DAI";
const mockSymbol = "L1DAI";
const mockDecimals = 18;
// Use shared constant from common/constants/general.ts
const RESERVED_STATUS = TOKEN_BRIDGE_RESERVED_STATUS;
const PLACEHOLDER_ADDRESS = getAddress("0x5555555555555555555555555555555555555555");
const CUSTOM_ADDRESS = getAddress("0x9999999999999999999999999999999999999999");
const EMPTY_PERMIT_DATA = "0x";

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

/** Receipt of the proxy deployment transaction: the first transaction that emitted a log from the proxy address. */
async function getDeploymentReceipt(contract: { address: Address }) {
  const { viem } = await network.getOrCreate();
  const publicClient = await viem.getPublicClient();
  const [firstLog] = await publicClient.getLogs({ address: contract.address, fromBlock: 0n });
  expect(firstLog, `no log emitted by ${contract.address}`).to.not.equal(undefined);
  return publicClient.getTransactionReceipt({ hash: firstLog.transactionHash });
}

function randomAddress(): Address {
  return privateKeyToAccount(generatePrivateKey()).address;
}

describe("TokenBridge", function () {
  async function deployContractsFixture() {
    const [owner, user] = await getTestAccounts();

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
    const encodedTokenMetadata = encodeAbiParameters(parseAbiParameters("string, string, uint8"), [
      mockName,
      mockSymbol,
      mockDecimals,
    ]);
    return { owner, user, ...deploymentFixture, tokens, encodedTokenMetadata };
  }

  describe("initialize", async function () {
    it("Should revert if it has already been intialized", async function () {
      const { user, l1TokenBridge, chainIds } = await loadFixture(deployContractsFixture);
      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.initialize(
          [
            {
              defaultAdmin: PLACEHOLDER_ADDRESS,
              messageService: PLACEHOLDER_ADDRESS,
              tokenBeacon: PLACEHOLDER_ADDRESS,
              sourceChainId: BigInt(chainIds[0]),
              targetChainId: BigInt(chainIds[1]),
              remoteSender: PLACEHOLDER_ADDRESS,
              reservedTokens: [],
              roleAddresses: [],
              pauseTypeRoles: [],
              unpauseTypeRoles: [],
            },
          ],
          { account: user.account },
        ),
        "InitializedVersionWrong",
        [0n, 3n],
      );
    });

    it("Should revert if one of the initializing parameters is address 0", async function () {
      const { user, owner, chainIds } = await loadFixture(deployContractsFixture);
      const TokenBridge = await hre.artifacts.readArtifact("TokenBridge");

      await expectRevertWithCustomError(
        TokenBridge,
        deployUpgradableFromFactory("TokenBridge", [
          {
            defaultAdmin: PLACEHOLDER_ADDRESS,
            messageService: ADDRESS_ZERO,
            tokenBeacon: PLACEHOLDER_ADDRESS,
            sourceChainId: chainIds[0],
            targetChainId: chainIds[1],
            remoteSender: PLACEHOLDER_ADDRESS,
            reservedTokens: [],
            roleAddresses: [],
            pauseTypeRoles: [],
            unpauseTypeRoles: [],
          },
        ]),
        "ZeroAddressNotAllowed",
      );

      await expectRevertWithCustomError(
        TokenBridge,
        deployUpgradableFromFactory("TokenBridge", [
          {
            defaultAdmin: PLACEHOLDER_ADDRESS,
            messageService: PLACEHOLDER_ADDRESS,
            tokenBeacon: ADDRESS_ZERO,
            sourceChainId: chainIds[0],
            targetChainId: chainIds[1],
            remoteSender: PLACEHOLDER_ADDRESS,
            reservedTokens: [],
            roleAddresses: [],
            pauseTypeRoles: [],
            unpauseTypeRoles: [],
          },
        ]),
        "ZeroAddressNotAllowed",
      );

      await expectRevertWithCustomError(
        TokenBridge,
        deployUpgradableFromFactory("TokenBridge", [
          {
            defaultAdmin: PLACEHOLDER_ADDRESS,
            messageService: PLACEHOLDER_ADDRESS,
            tokenBeacon: PLACEHOLDER_ADDRESS,
            sourceChainId: chainIds[0],
            targetChainId: chainIds[1],
            remoteSender: ADDRESS_ZERO,
            reservedTokens: [],
            roleAddresses: [],
            pauseTypeRoles: [],
            unpauseTypeRoles: [],
          },
        ]),
        "ZeroAddressNotAllowed",
      );

      await expectRevertWithCustomError(
        TokenBridge,
        deployUpgradableFromFactory("TokenBridge", [
          {
            defaultAdmin: PLACEHOLDER_ADDRESS,
            messageService: PLACEHOLDER_ADDRESS,
            tokenBeacon: PLACEHOLDER_ADDRESS,
            sourceChainId: chainIds[0],
            targetChainId: chainIds[1],
            remoteSender: PLACEHOLDER_ADDRESS,
            reservedTokens: [PLACEHOLDER_ADDRESS, ADDRESS_ZERO],
            roleAddresses: [
              { addressWithRole: user.address, role: SET_RESERVED_TOKEN_ROLE },
              { addressWithRole: owner.address, role: SET_RESERVED_TOKEN_ROLE },
            ],
            pauseTypeRoles: pauseTypeRoles,
            unpauseTypeRoles: unpauseTypeRoles,
          },
        ]),
        "ZeroAddressNotAllowed",
      );

      await expectRevertWithCustomError(
        TokenBridge,
        deployUpgradableFromFactory("TokenBridge", [
          {
            defaultAdmin: PLACEHOLDER_ADDRESS,
            messageService: PLACEHOLDER_ADDRESS,
            tokenBeacon: PLACEHOLDER_ADDRESS,
            sourceChainId: chainIds[0],
            targetChainId: chainIds[1],
            remoteSender: PLACEHOLDER_ADDRESS,
            reservedTokens: [PLACEHOLDER_ADDRESS],
            roleAddresses: [
              { addressWithRole: ADDRESS_ZERO, role: SET_RESERVED_TOKEN_ROLE },
              { addressWithRole: owner.address, role: SET_RESERVED_TOKEN_ROLE },
            ],
            pauseTypeRoles: pauseTypeRoles,
            unpauseTypeRoles: unpauseTypeRoles,
          },
        ]),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert if one of the initializing parameters is chainId 0", async function () {
      const { chainIds } = await loadFixture(deployContractsFixture);
      const TokenBridge = await hre.artifacts.readArtifact("TokenBridge");

      await expectRevertWithCustomError(
        TokenBridge,
        deployUpgradableFromFactory("TokenBridge", [
          {
            defaultAdmin: PLACEHOLDER_ADDRESS,
            messageService: PLACEHOLDER_ADDRESS,
            tokenBeacon: PLACEHOLDER_ADDRESS,
            sourceChainId: 0,
            targetChainId: chainIds[1],
            remoteSender: PLACEHOLDER_ADDRESS,
            reservedTokens: [],
            roleAddresses: [],
            pauseTypeRoles: [],
            unpauseTypeRoles: [],
          },
        ]),
        "ZeroChainIdNotAllowed",
      );

      await expectRevertWithCustomError(
        TokenBridge,
        deployUpgradableFromFactory("TokenBridge", [
          {
            defaultAdmin: PLACEHOLDER_ADDRESS,
            messageService: PLACEHOLDER_ADDRESS,
            tokenBeacon: PLACEHOLDER_ADDRESS,
            sourceChainId: chainIds[0],
            targetChainId: 0,
            remoteSender: PLACEHOLDER_ADDRESS,
            reservedTokens: [],
            roleAddresses: [],
            pauseTypeRoles: [],
            unpauseTypeRoles: [],
          },
        ]),
        "ZeroChainIdNotAllowed",
      );
    });

    it("Should revert if the sourceChainId is the same as the targetChainId", async function () {
      const { chainIds } = await loadFixture(deployContractsFixture);
      const TokenBridge = await hre.artifacts.readArtifact("TokenBridge");

      await expectRevertWithCustomError(
        TokenBridge,
        deployUpgradableFromFactory("TokenBridge", [
          {
            defaultAdmin: PLACEHOLDER_ADDRESS,
            messageService: PLACEHOLDER_ADDRESS,
            tokenBeacon: PLACEHOLDER_ADDRESS,
            sourceChainId: chainIds[0],
            targetChainId: chainIds[0],
            remoteSender: PLACEHOLDER_ADDRESS,
            reservedTokens: [],
            roleAddresses: [],
            pauseTypeRoles: [],
            unpauseTypeRoles: [],
          },
        ]),
        "SourceChainSameAsTargetChain",
      );
    });

    it("Should revert if the default admin is empty", async function () {
      const { chainIds } = await loadFixture(deployContractsFixture);
      const TokenBridge = await hre.artifacts.readArtifact("TokenBridge");

      await expectRevertWithCustomError(
        TokenBridge,
        deployUpgradableFromFactory("TokenBridge", [
          {
            defaultAdmin: ADDRESS_ZERO,
            messageService: PLACEHOLDER_ADDRESS,
            tokenBeacon: PLACEHOLDER_ADDRESS,
            sourceChainId: chainIds[0],
            targetChainId: chainIds[1],
            remoteSender: PLACEHOLDER_ADDRESS,
            reservedTokens: [],
            roleAddresses: [],
            pauseTypeRoles: [],
            unpauseTypeRoles: [],
          },
        ]),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should return 'NOT_VALID_ENCODING' for invalid data in _returnDataToString", async function () {
      const initData = {
        defaultAdmin: PLACEHOLDER_ADDRESS,
        messageService: PLACEHOLDER_ADDRESS,
        tokenBeacon: PLACEHOLDER_ADDRESS,
        sourceChainId: SupportedChainIds.SEPOLIA,
        targetChainId: SupportedChainIds.LINEA_TESTNET,
        remoteSender: PLACEHOLDER_ADDRESS,
        reservedTokens: [],
        roleAddresses: [],
        pauseTypeRoles: [],
        unpauseTypeRoles: [],
      };

      const l1TestTokenBridge = await deployUpgradableFromFactory("TestTokenBridge", [initData]);

      // Test case 1: Data length is not 32 and less than 64
      const invalidData1 = generateRandomBytes(33); // 33 bytes
      expect(await l1TestTokenBridge.read.testReturnDataToString([invalidData1])).to.equal("NOT_VALID_ENCODING");

      // Test case 2: Data length is 32 but starts with a zero byte
      const invalidData2 = concat([
        bytesToHex(new Uint8Array(1)), // One zero byte
        generateRandomBytes(31), // 31 random bytes
      ]);
      expect(await l1TestTokenBridge.read.testReturnDataToString([invalidData2])).to.equal("NOT_VALID_ENCODING");

      // Test case 3: Valid data for comparison
      const validString = "ValidString";
      const encodedValidData = encodeAbiParameters(parseAbiParameters("string"), [validString]);
      expect(await l1TestTokenBridge.read.testReturnDataToString([encodedValidData])).to.equal(validString);
    });

    it("Should have the correct contract version", async () => {
      const { l1TokenBridge, l2TokenBridge, l1TokenBridgeInitializationData, l2TokenBridgeInitializationData } =
        await loadFixture(deployContractsFixture);

      // Serialize init data to positional array format that ethers v6 toArray() produces
      const l1InitDataSerialized = serializeTokenBridgeInitData(l1TokenBridgeInitializationData);
      const l2InitDataSerialized = serializeTokenBridgeInitData(l2TokenBridgeInitializationData);

      let receipt = await getDeploymentReceipt(l1TokenBridge);

      await expectEventDirectFromReceiptData(
        l1TokenBridge,
        receipt,
        "TokenBridgeBaseInitialized",
        [pad(stringToHex("1.1"), { dir: "right", size: 8 }), l1InitDataSerialized],
        29,
      );

      receipt = await getDeploymentReceipt(l2TokenBridge);
      await expectEventDirectFromReceiptData(
        l2TokenBridge,
        receipt,
        "TokenBridgeBaseInitialized",
        [pad(stringToHex("1.1"), { dir: "right", size: 8 }), l2InitDataSerialized],
        29,
      );

      expect(await l1TokenBridge.read.CONTRACT_VERSION()).to.equal("1.1");

      expect(await l2TokenBridge.read.CONTRACT_VERSION()).to.equal("1.1");
    });
  });

  describe("Permissions", function () {
    it("Should revert if completeBridging  is not called by the messageService", async function () {
      const {
        user,
        l1TokenBridge,
        tokens: { L1DAI },
        encodedTokenMetadata,
        chainIds,
      } = await loadFixture(deployContractsFixture);

      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.completeBridging(
          [L1DAI.address, 1n, user.address, BigInt(chainIds[1]), encodedTokenMetadata],
          { account: user.account },
        ),
        "CallerIsNotMessageService",
      );
    });

    it("Should revert if completeBridging  message does not come from the remote Token Bridge", async function () {
      const {
        user,
        messageService,
        l1TokenBridge,
        l2TokenBridge,
        tokens: { L1DAI },
        encodedTokenMetadata,
        chainIds,
      } = await loadFixture(deployContractsFixture);

      const sendCalldata = messageService.write.sendMessage(
        [
          l2TokenBridge.address,
          0n,
          encodeFunctionData({
            abi: l1TokenBridge.abi,
            functionName: "completeBridging",
            args: [L1DAI.address, 1n, user.address, BigInt(chainIds[1]), encodedTokenMetadata],
          }),
        ],
        { account: user.account },
      );
      await expectRevertWithCustomError(l1TokenBridge, sendCalldata, "SenderNotAuthorized");
    });

    describe("setMessageService", function () {
      it("Should revert if trying to set message service to zero address", async function () {
        const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);

        await expectRevertWithCustomError(
          l1TokenBridge,
          l1TokenBridge.write.setMessageService([ADDRESS_ZERO], { account: owner.account }),
          "ZeroAddressNotAllowed",
        );
      });

      it("Should revert if called by non-owner", async function () {
        const { user, l1TokenBridge } = await loadFixture(deployContractsFixture);

        await expectAccessControlRevert(
          l1TokenBridge.write.setMessageService([PLACEHOLDER_ADDRESS], { account: user.account }),
          user,
          SET_MESSAGE_SERVICE_ROLE,
        );
      });

      it("Should successfully set new message service address", async function () {
        const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);
        const newMessageServiceAddress = randomAddress();
        const oldMessageServiceAddress = await l1TokenBridge.read.messageService();

        await expectEvent(
          l1TokenBridge,
          l1TokenBridge.write.setMessageService([newMessageServiceAddress], { account: owner.account }),
          "MessageServiceUpdated",
          [newMessageServiceAddress, oldMessageServiceAddress, owner.address],
        );

        expect(await l1TokenBridge.read.messageService()).to.equal(newMessageServiceAddress);
      });
    });

    describe("setCustomContract", function () {
      it("Should bridge EIP712-compliant-token with permit", async function () {
        const {
          user,
          l1TokenBridge,
          l2TokenBridge,
          tokens: { L1DAI },
          chainIds,
        } = await loadFixture(deployContractsFixture);

        const l1Token = L1DAI;
        const bridgeAmount = 70n;

        // Bridge token L1 to L2
        await l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });
        const l2TokenAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), l1Token.address]);
        const l2Token = await getContractAt("BridgedToken", l2TokenAddress);

        // Check that no allowance exist for l2Token (User => l2TokenBridge)
        expect(await l2Token.read.allowance([user.address, l2TokenBridge.address])).to.be.equal(0n);

        // Create EIP712-signature
        const deadline = maxUint256;
        const chainId = await user.getChainId();
        const nonce = await l2Token.read.nonces([user.address]);
        expect(nonce).to.be.equal(0n);

        // Try to bridge back without permit data
        await expectRevertWithReason(
          l2TokenBridge.write.bridgeToken([l2Token.address, bridgeAmount, user.address], { account: user.account }),
          "ERC20: insufficient allowance",
        );

        // Capture balances before bridging back
        const l1TokenUserBalanceBefore = await l1Token.read.balanceOf([user.address]);
        const l2TokenUserBalanceBefore = await l2Token.read.balanceOf([user.address]);

        // Prepare data for permit calldata
        const permitData = await getPermitData(
          user,
          l2Token,
          nonce,
          chainId,
          l2TokenBridge.address,
          bridgeAmount,
          deadline,
        );

        // Bridge back
        await l2TokenBridge.write.bridgeTokenWithPermit([l2Token.address, bridgeAmount, user.address, permitData], {
          account: user.account,
        });

        // Capture balances after bridging back
        const l1TokenUserBalanceAfter = await l1Token.read.balanceOf([user.address]);
        const l2TokenUserBalanceAfter = await l2Token.read.balanceOf([user.address]);

        const diffL1UserBalance = l1TokenUserBalanceAfter - l1TokenUserBalanceBefore;
        const diffL2UserBalance = l2TokenUserBalanceBefore - l2TokenUserBalanceAfter;

        expect(diffL1UserBalance).to.be.equal(bridgeAmount);
        expect(diffL2UserBalance).to.be.equal(bridgeAmount);
      });

      it("Should revert if setCustomContract is not called by the owner", async function () {
        const { user, l1TokenBridge } = await loadFixture(deployContractsFixture);
        await expectAccessControlRevert(
          l1TokenBridge.write.setCustomContract([CUSTOM_ADDRESS, CUSTOM_ADDRESS], { account: user.account }),
          user,
          SET_CUSTOM_CONTRACT_ROLE,
        );
      });

      it("Should revert if a native token has already been bridged", async function () {
        const {
          user,
          owner,
          l1TokenBridge,
          l2TokenBridge,
          tokens: { L1DAI },
          chainIds,
        } = await loadFixture(deployContractsFixture);
        const L1DAIAddress = L1DAI.address;
        // First bridge token (user has L1DAI balance set in the fixture)
        await l1TokenBridge.write.bridgeToken([L1DAIAddress, 1n, user.address], { account: user.account });
        const l2TokenAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAIAddress]);

        await expectRevertWithCustomError(
          l1TokenBridge,
          l1TokenBridge.write.setCustomContract([L1DAIAddress, CUSTOM_ADDRESS], { account: owner.account }),
          "AlreadyBridgedToken",
          [L1DAIAddress],
        );

        await expectRevertWithCustomError(
          l2TokenBridge,
          l2TokenBridge.write.setCustomContract([l2TokenAddress, CUSTOM_ADDRESS], { account: owner.account }),
          "AlreadyBridgedToken",
          [l2TokenAddress],
        );
      });

      it("Should revert if _nativeToken is zero address", async function () {
        const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);

        await expectRevertWithCustomError(
          l1TokenBridge,
          l1TokenBridge.write.setCustomContract([ADDRESS_ZERO, CUSTOM_ADDRESS], { account: owner.account }),
          "ZeroAddressNotAllowed",
        );
      });

      it("Should revert if _targetContract is zero address", async function () {
        const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);
        const validNativeToken = randomAddress();

        await expectRevertWithCustomError(
          l1TokenBridge,
          l1TokenBridge.write.setCustomContract([validNativeToken, ADDRESS_ZERO], { account: owner.account }),
          "ZeroAddressNotAllowed",
        );
      });

      it("Should successfully set custom contract for valid addresses", async function () {
        const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);
        const validNativeToken = randomAddress();
        const validTargetContract = randomAddress();

        await expectEvent(
          l1TokenBridge,
          l1TokenBridge.write.setCustomContract([validNativeToken, validTargetContract], { account: owner.account }),
          "CustomContractSet",
          [validNativeToken, validTargetContract, owner.address],
        );
      });
    });

    describe("Pause / unpause", function () {
      it("Should pause the contract when INITIATE_TOKEN_BRIDGING_PAUSE_TYPE() is called", async function () {
        const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);

        await l1TokenBridge.write.pauseByType([INITIATE_TOKEN_BRIDGING_PAUSE_TYPE], { account: owner.account });
        await expectPaused(l1TokenBridge, INITIATE_TOKEN_BRIDGING_PAUSE_TYPE);
      });

      it("Should unpause the contract when unpause() is called", async function () {
        const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);

        await l1TokenBridge.write.pauseByType([INITIATE_TOKEN_BRIDGING_PAUSE_TYPE], { account: owner.account });

        await l1TokenBridge.write.unPauseByType([INITIATE_TOKEN_BRIDGING_PAUSE_TYPE], { account: owner.account });

        await expectNotPaused(l1TokenBridge, INITIATE_TOKEN_BRIDGING_PAUSE_TYPE);
      });
      it("Should revert bridgeToken if paused", async function () {
        const {
          owner,
          l1TokenBridge,
          tokens: { L1DAI },
        } = await loadFixture(deployContractsFixture);

        await l1TokenBridge.write.pauseByType([INITIATE_TOKEN_BRIDGING_PAUSE_TYPE], { account: owner.account });

        await expectRevertWhenPaused(
          l1TokenBridge,
          l1TokenBridge.write.bridgeToken([L1DAI.address, 10n, owner.address]),
          INITIATE_TOKEN_BRIDGING_PAUSE_TYPE,
        );
      });
      it("Should allow bridgeToken if unpaused", async function () {
        const {
          owner,
          user,
          l1TokenBridge,
          tokens: { L1DAI },
        } = await loadFixture(deployContractsFixture);

        await l1TokenBridge.write.pauseByType([INITIATE_TOKEN_BRIDGING_PAUSE_TYPE], { account: owner.account });
        await l1TokenBridge.write.unPauseByType([INITIATE_TOKEN_BRIDGING_PAUSE_TYPE], { account: owner.account });
        await l1TokenBridge.write.bridgeToken([L1DAI.address, 10n, user.address], { account: user.account });
      });
      // TODO: COMPLETE_TOKEN_BRIDGING_PAUSE_TYPE tests
      it("Should pause the contract when pause() is called", async function () {
        const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);

        await l1TokenBridge.write.pauseByType([COMPLETE_TOKEN_BRIDGING_PAUSE_TYPE], { account: owner.account });
        await expectPaused(l1TokenBridge, COMPLETE_TOKEN_BRIDGING_PAUSE_TYPE);
      });

      it("Should unpause the contract when unpause() is called", async function () {
        const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);

        await l1TokenBridge.write.pauseByType([COMPLETE_TOKEN_BRIDGING_PAUSE_TYPE], { account: owner.account });

        await l1TokenBridge.write.unPauseByType([COMPLETE_TOKEN_BRIDGING_PAUSE_TYPE], { account: owner.account });

        await expectNotPaused(l1TokenBridge, COMPLETE_TOKEN_BRIDGING_PAUSE_TYPE);
      });

      it("Should emit BridgingInitiatedV2 when bridging", async function () {
        const {
          user,
          l1TokenBridge,
          tokens: { L1DAI },
        } = await loadFixture(deployContractsFixture);

        const L1DAIAddress = L1DAI.address;

        await expectEvent(
          l1TokenBridge,
          l1TokenBridge.write.bridgeToken([L1DAIAddress, BigInt(10), user.address], { account: user.account }),
          "BridgingInitiatedV2",
          [user.address, user.address, L1DAIAddress, 10n],
        );
      });

      it("Should emit BridgingFinalizedV2 event when bridging is complete", async function () {
        const { viem } = await network.getOrCreate();
        const {
          user,
          l1TokenBridge,
          l2TokenBridge,
          tokens: { L1DAI },
          chainIds,
        } = await loadFixture(deployContractsFixture);
        const bridgeAmount = 10n;

        const L1DAIAddress = L1DAI.address;

        // const initialAmount = await L1DAI.read.balanceOf([user.address]);
        await l1TokenBridge.write.bridgeToken([L1DAIAddress, bridgeAmount, user.address], { account: user.account });
        const L2DAIBridgedAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAIAddress]);
        await l2TokenBridge.write.confirmDeployment([[L2DAIBridgedAddress]]);

        const bridgedToken = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAIAddress]);

        // Filtering for indexed fields by default validates they are correct when events are not null
        const events = await (
          await viem.getPublicClient()
        ).getContractEvents({
          address: l2TokenBridge.address,
          abi: l2TokenBridge.abi,
          eventName: "BridgingFinalizedV2",
          args: { nativeToken: L1DAIAddress, bridgedToken, recipient: user.address },
          fromBlock: 0n,
        });
        expect(events).to.not.be.null;
        expect(events).to.not.be.empty;
        expect(events[0].args.amount).to.equal(10n);
      });
    });

    describe("Owner", function () {
      it("Should revert if setReservedToken is called by a non-owner", async function () {
        const { user, l1TokenBridge } = await loadFixture(deployContractsFixture);
        await expectAccessControlRevert(
          l1TokenBridge.write.setReserved([user.address], { account: user.account }),
          user,
          SET_RESERVED_TOKEN_ROLE,
        );
      });
      it("Should revert if pause() is called by a non-owner", async function () {
        const { user, l1TokenBridge } = await loadFixture(deployContractsFixture);
        await expectAccessControlRevert(
          l1TokenBridge.write.pauseByType([INITIATE_TOKEN_BRIDGING_PAUSE_TYPE], { account: user.account }),
          user,
          PAUSE_INITIATE_TOKEN_BRIDGING_ROLE,
        );
      });
      it("Should revert if unpause() is called by a non-owner", async function () {
        const { user, l1TokenBridge } = await loadFixture(deployContractsFixture);
        await expectAccessControlRevert(
          l1TokenBridge.write.unPauseByType([INITIATE_TOKEN_BRIDGING_PAUSE_TYPE], { account: user.account }),
          user,
          UNPAUSE_INITIATE_TOKEN_BRIDGING_ROLE,
        );
      });
      it("Should revert if removeReserved is called by a non-owner", async function () {
        const {
          user,
          l1TokenBridge,
          tokens: { L1DAI },
        } = await loadFixture(deployContractsFixture);
        await expectAccessControlRevert(
          l1TokenBridge.write.removeReserved([L1DAI.address], { account: user.account }),
          user,
          REMOVE_RESERVED_TOKEN_ROLE,
        );
      });
    });
  });

  describe("Reserved tokens", function () {
    it("Should be possible for the admin to reserve a token", async function () {
      const {
        owner,
        l1TokenBridge,
        tokens: { L1DAI },
        chainIds,
      } = await loadFixture(deployContractsFixture);
      // Must not revert (the former `.not.to.be.revertedWith("TokenBridge: token already bridged")` check).
      await l1TokenBridge.write.setReserved([L1DAI.address], { account: owner.account });
      expect(await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address])).to.be.equal(
        RESERVED_STATUS,
      );
    });

    it("Should be possible for the admin to remove token from the reserved list", async function () {
      // @TODO this test can probably be rewritten, avoiding to set the token as reserved in the first place

      const {
        owner,
        l1TokenBridge,
        tokens: { L1DAI },
        chainIds,
      } = await loadFixture(deployContractsFixture);

      const L1DAIAddress = L1DAI.address;

      await l1TokenBridge.write.setReserved([L1DAIAddress], { account: owner.account });
      expect(await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAIAddress])).to.be.equal(
        RESERVED_STATUS,
      );
      await expectEvent(
        l1TokenBridge,
        l1TokenBridge.write.removeReserved([L1DAIAddress], { account: owner.account }),
        "ReservationRemoved",
        [L1DAIAddress],
      );
      expect(await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAIAddress])).to.be.equal(
        ADDRESS_ZERO,
      );
    });

    it("Should not be possible to bridge reserved tokens", async function () {
      const {
        owner,
        user,
        l1TokenBridge,
        tokens: { L1DAI },
      } = await loadFixture(deployContractsFixture);
      await l1TokenBridge.write.setReserved([L1DAI.address], { account: owner.account });

      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.bridgeToken([L1DAI.address, 1n, user.address], { account: user.account }),
        "ReservedToken",
        [L1DAI.address],
      );
    });

    it("Should only be possible to reserve a token if it has not been bridged before", async function () {
      const {
        owner,
        user,
        l1TokenBridge,
        tokens: { L1DAI },
      } = await loadFixture(deployContractsFixture);
      await l1TokenBridge.write.bridgeToken([L1DAI.address, 1n, user.address], { account: user.account });

      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.setReserved([L1DAI.address], { account: owner.account }),
        "AlreadyBridgedToken",
        [L1DAI.address],
      );
    });

    it("Should set reserved tokens in the initializer", async function () {
      const { chainIds } = await loadFixture(deployContractsFixture);
      const l1TokenBridge = await deployUpgradableFromFactory("TokenBridge", [
        {
          defaultAdmin: PLACEHOLDER_ADDRESS,
          messageService: PLACEHOLDER_ADDRESS,
          tokenBeacon: PLACEHOLDER_ADDRESS,
          sourceChainId: chainIds[0],
          targetChainId: chainIds[1],
          reservedTokens: [CUSTOM_ADDRESS],
          remoteSender: PLACEHOLDER_ADDRESS,
          roleAddresses: [],
          pauseTypeRoles: pauseTypeRoles,
          unpauseTypeRoles: unpauseTypeRoles,
        },
      ]);
      expect(await l1TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), CUSTOM_ADDRESS])).to.be.equal(
        RESERVED_STATUS,
      );
    });

    it("Should only be possible to call removeReserved if the token is in the reserved list", async function () {
      const {
        owner,
        l1TokenBridge,
        tokens: { L1DAI },
      } = await loadFixture(deployContractsFixture);

      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.removeReserved([L1DAI.address], { account: owner.account }),
        "NotReserved",
        [L1DAI.address],
      );
    });

    it("Should revert if token is the 0 address", async function () {
      const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);
      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.setReserved([ADDRESS_ZERO], { account: owner.account }),
        "ZeroAddressNotAllowed",
      );
    });
    it("Should revert if trying to remove reservation for zero address", async function () {
      const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);

      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.removeReserved([ADDRESS_ZERO], { account: owner.account }),
        "ZeroAddressNotAllowed",
      );
    });
  });

  describe("bridgeTokenWithPermit", function () {
    it("Should revert if contract is paused", async function () {
      const {
        user,
        tokens: { L1DAI },
        l1TokenBridge,
      } = await loadFixture(deployContractsFixture);
      await l1TokenBridge.write.pauseByType([INITIATE_TOKEN_BRIDGING_PAUSE_TYPE]);
      await expectRevertWhenPaused(
        l1TokenBridge,
        l1TokenBridge.write.bridgeTokenWithPermit([L1DAI.address, 1n, user.address, EMPTY_PERMIT_DATA], {
          account: user.account,
        }),
        INITIATE_TOKEN_BRIDGING_PAUSE_TYPE,
      );
    });

    it("Should revert if token is the 0 address", async function () {
      const { user, l1TokenBridge } = await loadFixture(deployContractsFixture);
      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.bridgeTokenWithPermit([ADDRESS_ZERO, 1n, user.address, EMPTY_PERMIT_DATA], {
          account: user.account,
        }),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert if token is the amount is 0", async function () {
      const {
        user,
        l1TokenBridge,
        tokens: { L1DAI },
      } = await loadFixture(deployContractsFixture);
      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.bridgeTokenWithPermit([L1DAI.address, 0n, user.address, EMPTY_PERMIT_DATA], {
          account: user.account,
        }),
        "ZeroAmountNotAllowed",
        [0n],
      );
    });

    it("Should not revert if permitData is empty", async function () {
      const {
        user,
        l1TokenBridge,
        tokens: { L1DAI },
      } = await loadFixture(deployContractsFixture);
      await l1TokenBridge.write.bridgeTokenWithPermit([L1DAI.address, 10n, user.address, EMPTY_PERMIT_DATA], {
        account: user.account,
      });
    });

    it("Should revert if permitData is invalid", async function () {
      const {
        owner,
        user,
        l1TokenBridge,
        l2TokenBridge,
        tokens: { L1DAI },
        chainIds,
      } = await loadFixture(deployContractsFixture);
      // Test when the permitData has an invalid format
      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.bridgeTokenWithPermit([L1DAI.address, 10n, user.address, "0x111111111111"], {
          account: user.account,
        }),
        "InvalidPermitData",
      );

      // Test when the spender passed is invalid
      // Prepare data for permit calldata
      const bridgeAmount = 70n;

      // Bridge token L1 to L2
      await l1TokenBridge.write.bridgeToken([L1DAI.address, bridgeAmount, user.address], { account: user.account });
      const l2TokenAddress = await l2TokenBridge.read.nativeToBridgedToken([BigInt(chainIds[0]), L1DAI.address]);
      const l2Token = await getContractAt("BridgedToken", l2TokenAddress);

      // Create EIP712-signature
      const deadline = maxUint256;
      const chainId = await user.getChainId();
      const nonce = await l2Token.read.nonces([user.address]);

      let permitData = await getPermitData(user, l2Token, nonce, chainId, user.address, bridgeAmount, deadline);
      await expectRevertWithCustomError(
        l1TokenBridge,
        l2TokenBridge.write.bridgeTokenWithPermit([L1DAI.address, bridgeAmount, user.address, permitData], {
          account: user.account,
        }),
        "PermitNotAllowingBridge",
      );

      // Test when the sender is not the owner of the tokens
      permitData = await getPermitData(user, l2Token, nonce, chainId, l2TokenBridge.address, bridgeAmount, deadline);
      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.bridgeTokenWithPermit([L1DAI.address, bridgeAmount, user.address, permitData], {
          account: owner.account,
        }),
        "PermitNotFromSender",
      );
    });
  });

  describe("bridgeToken", function () {
    it("Should not emit event NewToken if the token has already been bridged once", async function () {
      const {
        user,
        tokens: { L1DAI },
        l1TokenBridge,
      } = await loadFixture(deployContractsFixture);
      await expectEvent(
        l1TokenBridge,
        l1TokenBridge.write.bridgeToken([L1DAI.address, 1n, user.address], { account: user.account }),
        "NewToken",
      );
      await expectNoEvent(
        l1TokenBridge,
        l1TokenBridge.write.bridgeToken([L1DAI.address, 1n, user.address], { account: user.account }),
        "NewToken",
      );
    });

    it("Should revert if recipient is set at 0 address", async function () {
      const {
        user,
        tokens: { L1DAI },
        l1TokenBridge,
      } = await loadFixture(deployContractsFixture);
      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.bridgeToken([L1DAI.address, 1n, ADDRESS_ZERO], { account: user.account }),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should not be able to call bridgeToken by reentrancy", async function () {
      const { owner, l1TokenBridge } = await loadFixture(deployContractsFixture);

      const reentrancyContract = await deployFromFactory("ReentrancyContract", [l1TokenBridge.address]);

      const maliciousERC777 = await deployFromFactory("MaliciousERC777", [reentrancyContract.address]);
      await maliciousERC777.write.mint([reentrancyContract.address, 100n]);
      await maliciousERC777.write.mint([owner.address, 100n]);

      await reentrancyContract.write.setToken([maliciousERC777.address]);

      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.bridgeToken([maliciousERC777.address, 1n, owner.address]),
        "ReentrantCall",
      );
    });
  });

  describe("setDeployed", function () {
    it("Should revert if not called by the messageService", async function () {
      const { user, l1TokenBridge } = await loadFixture(deployContractsFixture);
      await expectRevertWithCustomError(
        l1TokenBridge,
        l1TokenBridge.write.setDeployed([[]], { account: user.account }),
        "CallerIsNotMessageService",
      );
    });

    it("Should revert if message does not come from the remote Token Bridge", async function () {
      const { user, messageService, l1TokenBridge, l2TokenBridge } = await loadFixture(deployContractsFixture);
      await expectRevertWithCustomError(
        l2TokenBridge,
        messageService.write.sendMessage(
          [
            l2TokenBridge.address,
            0n,
            encodeFunctionData({ abi: l1TokenBridge.abi, functionName: "setDeployed", args: [[]] }),
          ],
          { account: user.account },
        ),
        "SenderNotAuthorized",
      );
    });
  });

  describe("TokenBridge Upgradeable Tests", function () {
    it("Should set initialized version to 2 on fresh deploy", async function () {
      const initData = {
        defaultAdmin: PLACEHOLDER_ADDRESS,
        messageService: PLACEHOLDER_ADDRESS,
        tokenBeacon: PLACEHOLDER_ADDRESS,
        sourceChainId: SupportedChainIds.SEPOLIA,
        targetChainId: SupportedChainIds.LINEA_TESTNET,
        remoteSender: PLACEHOLDER_ADDRESS,
        reservedTokens: [],
        roleAddresses: [],
        pauseTypeRoles: [],
        unpauseTypeRoles: [],
      };

      const testTokenBridge = await deployUpgradableFromFactory("TestTokenBridge", [initData]);

      const slotValue = await testTokenBridge.read.getSlotValue([0n]);
      expect(slotValue).equal(3n);
    });

    it("Should deploy and manually upgrade the TokenBridge contract", async function () {
      const initData = {
        defaultAdmin: PLACEHOLDER_ADDRESS,
        messageService: PLACEHOLDER_ADDRESS,
        tokenBeacon: PLACEHOLDER_ADDRESS,
        sourceChainId: SupportedChainIds.SEPOLIA,
        targetChainId: SupportedChainIds.LINEA_TESTNET,
        remoteSender: PLACEHOLDER_ADDRESS,
        reservedTokens: [],
        roleAddresses: [],
        pauseTypeRoles: [],
        unpauseTypeRoles: [],
      };

      const testTokenBridge = await deployUpgradableFromFactory("TestTokenBridge", [initData]);

      // Simulate a pre-upgrade state by lowering the initialized version
      await testTokenBridge.write.setSlotValue([0n, 1n]);

      let slotValue = await testTokenBridge.read.getSlotValue([1n]);
      expect(slotValue).equal(0n);
      await testTokenBridge.write.setSlotValue([1n, 1n]);

      // simulating reentry value at slot 1
      slotValue = await testTokenBridge.read.getSlotValue([1n]);
      expect(slotValue).equal(1n);

      // Deploy new TokenBridge implementation
      const oz = await getUpgrades();
      await oz.upgradeProxy(testTokenBridge, "src/_testing/mocks/bridging/TestTokenBridge.sol:TestTokenBridge", {
        call: { fn: "reinitializeV3" },
        kind: "transparent",
        unsafeAllowRenames: true,
        unsafeAllow: ["incorrect-initializer-order"],
      });

      // reentry slot cleared
      slotValue = await testTokenBridge.read.getSlotValue([1n]);
      expect(slotValue).equal(0n);

      // version changed
      slotValue = await testTokenBridge.read.getSlotValue([0n]);
      expect(slotValue).equal(3n);
    });

    it("Should revert reinitializeV3 if old reentrancy guard slot indicates ENTERED", async function () {
      const initData = {
        defaultAdmin: PLACEHOLDER_ADDRESS,
        messageService: PLACEHOLDER_ADDRESS,
        tokenBeacon: PLACEHOLDER_ADDRESS,
        sourceChainId: SupportedChainIds.SEPOLIA,
        targetChainId: SupportedChainIds.LINEA_TESTNET,
        remoteSender: PLACEHOLDER_ADDRESS,
        reservedTokens: [],
        roleAddresses: [],
        pauseTypeRoles: [],
        unpauseTypeRoles: [],
      };

      const testTokenBridge = await deployUpgradableFromFactory("TestTokenBridge", [initData]);

      // Simulate a pre-upgrade state by lowering the initialized version
      await testTokenBridge.write.setSlotValue([0n, 1n]);

      // Set legacy reentrancy guard slot to OZ ENTERED value (2)
      await testTokenBridge.write.setSlotValue([1n, 2n]);

      const oz = await getUpgrades();

      await expectRevertWithCustomError(
        testTokenBridge,
        oz.upgradeProxy(testTokenBridge, "src/_testing/mocks/bridging/TestTokenBridge.sol:TestTokenBridge", {
          call: { fn: "reinitializeV3" },
          kind: "transparent",
          unsafeAllowRenames: true,
          unsafeAllow: ["incorrect-initializer-order"],
        }),
        "ReentrantCall",
      );
    });
  });
});
