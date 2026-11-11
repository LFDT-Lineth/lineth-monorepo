import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";
import { decodeEventLog, encodeAbiParameters, keccak256, parseAbiParameters, parseUnits } from "viem";

import {
  ADDRESS_ZERO,
  EMPTY_CALLDATA,
  INITIAL_WITHDRAW_LIMIT,
  MESSAGE_VALUE_1ETH,
  ONE_DAY_IN_SECONDS,
  pauseTypeRoles,
  unpauseTypeRoles,
  VALID_MERKLE_PROOF_WITH_ZERO_FEE,
} from "../common/constants/index.js";
import { deployFromFactory, deployUpgradableFromFactory } from "../common/deployment.js";
import {
  type TestAccount,
  getTestAccounts,
  hex,
  expectRevertWithCustomError,
  expectEvent,
} from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

// TODO: Dynamically generate a valid merkle proof for testing instead of using a hardcoded one.
// Should trigger a call to the token bridge to mint tokens on L1 to the burner address as part of the test setup.
describe("L1LineaTokenBurner", () => {
  let l1LineaTokenBurner: ContractReturnType<"L1LineaTokenBurner">;
  let admin: TestAccount;
  let lineaToken: ContractReturnType<"MockL1LineaToken">;
  let messageService: ContractReturnType<"TestL1MessageServiceMerkleProof">;

  const INITIAL_LINEA_TOKEN_SUPPLY = parseUnits("100", 18);

  const VALID_PROOF = VALID_MERKLE_PROOF_WITH_ZERO_FEE.proof.map(hex);
  const VALID_MERKLE_ROOT = hex(VALID_MERKLE_PROOF_WITH_ZERO_FEE.merkleRoot);

  async function deployTestL1MessageServiceFixture() {
    return deployUpgradableFromFactory(
      "TestL1MessageServiceMerkleProof",
      [ONE_DAY_IN_SECONDS, INITIAL_WITHDRAW_LIMIT, pauseTypeRoles, unpauseTypeRoles],
      { unsafeAllow: ["constructor", "incorrect-initializer-order"] },
    );
  }

  async function deployL1LineaTokenBurnerContractFixture() {
    const { networkHelpers } = await network.getOrCreate();
    const messageService = await networkHelpers.loadFixture(deployTestL1MessageServiceFixture);

    const l2LineaToken = await deployFromFactory("TestERC20", ["TestERC20", "TEST", parseUnits("1000000000", 18)]);
    const lineaTokenFn = async () =>
      deployUpgradableFromFactory(
        "MockL1LineaToken",
        [messageService.address, l2LineaToken.address, "TestERC20", "TEST"],
        { unsafeAllow: ["constructor", "incorrect-initializer-order"] },
      );

    const l1LineaToken = await networkHelpers.loadFixture(lineaTokenFn);

    const l1LineaTokenBurnerFn = async () =>
      deployFromFactory("L1LineaTokenBurner", [messageService.address, l1LineaToken.address]);
    const l1LineaTokenBurner = await networkHelpers.loadFixture(l1LineaTokenBurnerFn);

    return { l1LineaTokenBurner, lineaToken: l1LineaToken, messageService };
  }

  before(async () => {
    // Hardhat 2 reset the chain here (`hardhat_reset`), which Hardhat 3's EDR does not support; the fixtures below
    // do not depend on a fresh chain.
    [admin] = await getTestAccounts();
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ l1LineaTokenBurner, lineaToken, messageService } = await networkHelpers.loadFixture(
      deployL1LineaTokenBurnerContractFixture,
    ));
    // Mint some LINEA tokens to the admin so that we can test the burning functionality.
    // It is used to initialize the total supply of the token.
    await lineaToken.write.mint([admin.address, INITIAL_LINEA_TOKEN_SUPPLY]);
  });

  describe("construtor", () => {
    it("Should revert when messageService address is zero", async () => {
      await expectRevertWithCustomError(
        l1LineaTokenBurner,
        deployFromFactory("L1LineaTokenBurner", [ADDRESS_ZERO, lineaToken.address]),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert when LINEA token address is zero", async () => {
      await expectRevertWithCustomError(
        l1LineaTokenBurner,
        deployFromFactory("L1LineaTokenBurner", [messageService.address, ADDRESS_ZERO]),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should emit an event when initialized", async () => {
      const { viem } = await network.getOrCreate();
      const publicClient = await viem.getPublicClient();
      const { deploymentTransaction } = await viem.sendDeploymentTransaction("L1LineaTokenBurner", [
        messageService.address,
        lineaToken.address,
      ]);

      const receipt = await publicClient.waitForTransactionReceipt({ hash: deploymentTransaction.hash });
      const logs = receipt.logs;

      expect(logs).to.have.lengthOf(1);

      const event = decodeEventLog({ abi: l1LineaTokenBurner.abi, data: logs[0].data, topics: logs[0].topics });
      expect(event.eventName).to.equal("L1LineaTokenBurnerInitialized");
      expect(event.args.messageService).to.equal(messageService.address);
      expect(event.args.lineaToken).to.equal(lineaToken.address);
    });

    it("Should set the correct addresses", async () => {
      const lineaTokenAddress = await l1LineaTokenBurner.read.LINEA_TOKEN();
      const messageServiceAddress = await l1LineaTokenBurner.read.MESSAGE_SERVICE();

      expect(lineaTokenAddress).to.equal(lineaToken.address);
      expect(messageServiceAddress).to.equal(messageService.address);
    });
  });

  describe("claimMessageWithProof", () => {
    it("Should revert when message is not anchored on L1", async () => {
      await expectRevertWithCustomError(
        messageService,
        l1LineaTokenBurner.write.claimMessageWithProof([
          {
            proof: VALID_PROOF,
            messageNumber: 1n,
            leafIndex: VALID_MERKLE_PROOF_WITH_ZERO_FEE.index,
            from: admin.address,
            to: admin.address,
            fee: 0n,
            value: MESSAGE_VALUE_1ETH,
            feeRecipient: ADDRESS_ZERO,
            merkleRoot: VALID_MERKLE_ROOT,
            data: EMPTY_CALLDATA,
          },
        ]),
        "L2MerkleRootDoesNotExist",
      );
    });

    it("Should revert when burner contract balance == 0", async () => {
      await messageService.write.setL2L1MessageToClaimed([1n]);

      const burnerBalanceBefore = await lineaToken.read.balanceOf([l1LineaTokenBurner.address]);
      expect(burnerBalanceBefore).to.equal(0n);

      const txPromise = l1LineaTokenBurner.write.claimMessageWithProof([
        {
          proof: VALID_PROOF,
          messageNumber: 1n,
          leafIndex: VALID_MERKLE_PROOF_WITH_ZERO_FEE.index,
          from: admin.address,
          to: admin.address,
          fee: 0n,
          value: MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: VALID_MERKLE_ROOT,
          data: EMPTY_CALLDATA,
        },
      ]);
      await expectRevertWithCustomError(l1LineaTokenBurner, txPromise, "NoTokensToBurn");
    });

    it("Should only burn LINEA tokens and syncTotalSupplyToL2 when message is already claimed and burner contract balance > 0", async () => {
      await messageService.write.setL2L1MessageToClaimed([1n]);

      const lineaTokensBalanceOwnedByBurnerContract = parseUnits("100", 18);
      await lineaToken.write.mint([l1LineaTokenBurner.address, lineaTokensBalanceOwnedByBurnerContract]);

      const totalSupplyBeforeBurning = await lineaToken.read.totalSupply();
      expect(totalSupplyBeforeBurning).to.equal(INITIAL_LINEA_TOKEN_SUPPLY + lineaTokensBalanceOwnedByBurnerContract);

      const burnerBalanceBefore = await lineaToken.read.balanceOf([l1LineaTokenBurner.address]);
      expect(burnerBalanceBefore).to.equal(lineaTokensBalanceOwnedByBurnerContract);

      const txPromise = l1LineaTokenBurner.write.claimMessageWithProof([
        {
          proof: VALID_PROOF,
          messageNumber: 1n,
          leafIndex: VALID_MERKLE_PROOF_WITH_ZERO_FEE.index,
          from: admin.address,
          to: admin.address,
          fee: 0n,
          value: MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: VALID_MERKLE_ROOT,
          data: EMPTY_CALLDATA,
        },
      ]);

      await expectEvent(lineaToken, txPromise, "Transfer", [
        l1LineaTokenBurner.address,
        ADDRESS_ZERO,
        lineaTokensBalanceOwnedByBurnerContract,
      ]);

      const burnerBalanceAfter = await lineaToken.read.balanceOf([l1LineaTokenBurner.address]);
      expect(burnerBalanceAfter).to.equal(0n);
    });

    it("Should claim message, burn LINEA tokens and syncTotalSupplyToL2 when message is not yet claimed and burner contract balance > 0", async () => {
      await messageService.write.addFunds({ value: INITIAL_WITHDRAW_LIMIT * 2n });

      const lineaTokensBalanceOwnedByBurnerContract = parseUnits("100", 18);
      await lineaToken.write.mint([l1LineaTokenBurner.address, lineaTokensBalanceOwnedByBurnerContract]);

      const burnerBalanceBefore = await lineaToken.read.balanceOf([l1LineaTokenBurner.address]);
      expect(burnerBalanceBefore).to.equal(lineaTokensBalanceOwnedByBurnerContract);

      await messageService.write.addL2MerkleRoots([[VALID_MERKLE_ROOT], BigInt(VALID_PROOF.length)]);

      const txPromise = l1LineaTokenBurner.write.claimMessageWithProof([
        {
          proof: VALID_PROOF,
          messageNumber: 1n,
          leafIndex: VALID_MERKLE_PROOF_WITH_ZERO_FEE.index,
          from: admin.address,
          to: admin.address,
          fee: 0n,
          value: MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: VALID_MERKLE_ROOT,
          data: EMPTY_CALLDATA,
        },
      ]);

      const messageLeafHash = keccak256(
        encodeAbiParameters(parseAbiParameters("address, address, uint256, uint256, uint256, bytes"), [
          admin.address,
          admin.address,
          0n,
          MESSAGE_VALUE_1ETH,
          1n,
          EMPTY_CALLDATA,
        ]),
      );

      await expectEvent(messageService, txPromise, "MessageClaimed", [messageLeafHash]);
      await expectEvent(lineaToken, txPromise, "Transfer", [
        l1LineaTokenBurner.address,
        ADDRESS_ZERO,
        lineaTokensBalanceOwnedByBurnerContract,
      ]);
    });
  });
});
