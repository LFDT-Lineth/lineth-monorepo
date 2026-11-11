import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, beforeEach } from "node:test";
import { getAddress, zeroAddress } from "viem";
import { generatePrivateKey, privateKeyToAccount } from "viem/accounts";

import { L2_MESSAGE_SERVICE_ROLES } from "../../../common/constants/index.js";
import { generateRoleAssignments } from "../../../common/helpers/index.js";
import {
  INITIALIZED_ERROR_MESSAGE,
  INITIAL_WITHDRAW_LIMIT,
  L1_L2_MESSAGE_SETTER_ROLE,
  ONE_DAY_IN_SECONDS,
  pauseTypeRoles,
  unpauseTypeRoles,
} from "../common/constants/index.js";
import { deployUpgradableFromFactory } from "../common/deployment.js";
import {
  type TestAccount,
  getTestAccounts,
  expectRevertWithCustomError,
  expectRevertWithReason,
  expectEvent,
} from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("MessageServiceBase", () => {
  let messageServiceBase: ContractReturnType<"TestMessageServiceBase">;
  let messageService: ContractReturnType<"TestL2MessageService">;
  let admin: TestAccount;
  let remoteSender: TestAccount;
  let securityCouncil: TestAccount;
  let l1L2MessageSetter: TestAccount;

  async function deployMessageServiceBaseFixture() {
    const roleAddresses = generateRoleAssignments(L2_MESSAGE_SERVICE_ROLES, securityCouncil.address, [
      { role: L1_L2_MESSAGE_SETTER_ROLE, addresses: [l1L2MessageSetter.address] },
    ]);

    const messageService = await deployUpgradableFromFactory("TestL2MessageService", [
      ONE_DAY_IN_SECONDS,
      INITIAL_WITHDRAW_LIMIT,
      securityCouncil.address,
      roleAddresses,
      pauseTypeRoles,
      unpauseTypeRoles,
    ]);

    const messageServiceBase = await deployUpgradableFromFactory("TestMessageServiceBase", [
      messageService.address,
      remoteSender.address,
    ]);
    return { messageService, messageServiceBase };
  }

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    [admin, remoteSender, securityCouncil, l1L2MessageSetter] = await getTestAccounts();
    const contracts = await networkHelpers.loadFixture(deployMessageServiceBaseFixture);
    messageService = contracts.messageService;
    messageServiceBase = contracts.messageServiceBase;
  });

  describe("Initialization checks", () => {
    it("Should revert if message service address is address(0)", async () => {
      await expectRevertWithCustomError(
        messageService,
        deployUpgradableFromFactory("TestMessageServiceBase", [zeroAddress, remoteSender.address]),
        "ZeroAddressNotAllowed",
      );
    });

    it("It should fail when not initializing", async () => {
      await expectRevertWithReason(
        messageServiceBase.write.tryInitialize([messageService.address, remoteSender.address]),
        INITIALIZED_ERROR_MESSAGE,
      );
    });

    it("Should revert if remote sender address is address(0)", async () => {
      await expectRevertWithCustomError(
        messageServiceBase,
        deployUpgradableFromFactory("TestMessageServiceBase", [messageService.address, zeroAddress]),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should set the value of remoteSender variable in storage", async () => {
      expect(await messageServiceBase.read.remoteSender()).to.equal(getAddress(remoteSender.address));
    });

    it("Should set the value of messageService variable in storage", async () => {
      expect(await messageServiceBase.read.messageService()).to.equal(messageService.address);
    });
  });

  describe("RemoteSenderSet event", () => {
    it("Should emit RemoteSenderSet event when testSetRemoteSender is called", async () => {
      const newRemoteSender = privateKeyToAccount(generatePrivateKey()).address;
      await expectEvent(
        messageServiceBase,
        messageServiceBase.write.testSetRemoteSender([newRemoteSender]),
        "RemoteSenderSet",
        [newRemoteSender, admin.address],
      );
    });
  });

  describe("onlyMessagingService() modifier", () => {
    it("Should revert if msg.sender is not the message service address", async () => {
      await expectRevertWithCustomError(
        messageServiceBase,
        messageServiceBase.write.withOnlyMessagingService(),
        "CallerIsNotMessageService",
      );
    });

    it("Should succeed if msg.sender is the message service address", async () => {
      const { viem } = await network.getOrCreate();
      const publicClient = await viem.getPublicClient();
      const hash = await messageService.write.callMessageServiceBase([messageServiceBase.address]);
      expect((await publicClient.waitForTransactionReceipt({ hash })).status).to.equal("success");
    });
  });

  describe("onlyAuthorizedRemoteSender() modifier", () => {
    it("Should revert if sender is not allowed", async () => {
      await expectRevertWithCustomError(
        messageServiceBase,
        messageServiceBase.write.withOnlyAuthorizedRemoteSender(),
        "SenderNotAuthorized",
      );
    });

    it("Should succeed if original sender is allowed", async () => {
      // Construct a call A from `remoteSender` to `messageService`
      // Call A will created a nested call from `messageService` to `messageServiceBase`, invoking onlyAuthorizedRemoteSender modifier
      const { viem } = await network.getOrCreate();
      const publicClient = await viem.getPublicClient();
      const hash = await messageService.write.claimMessageWithoutChecks([
        remoteSender.address,
        messageServiceBase.address,
        0n,
        "0xfcd38105", // keccak256("withOnlyAuthorizedRemoteSender()")
      ]);
      expect((await publicClient.waitForTransactionReceipt({ hash })).status).to.equal("success");
    });
  });
});
