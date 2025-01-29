import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";
import { TestL2MessageService, TestMessageServiceBase } from "../../../typechain-types";
import {
  INITIALIZED_ERROR_MESSAGE,
  INITIAL_WITHDRAW_LIMIT,
  L1_L2_MESSAGE_SETTER_ROLE,
  ONE_DAY_IN_SECONDS,
  pauseTypeRoles,
  unpauseTypeRoles,
} from "../common/constants";
import { deployUpgradableFromFactory } from "../common/deployment";
import { expectEvent, expectRevertWithCustomError, expectRevertWithReason } from "../common/helpers";
import { generateRoleAssignments } from "contracts/common/helpers";
import { L2_MESSAGE_SERVICE_ROLES } from "contracts/common/constants";

describe("MessageServiceBase", () => {
  let messageServiceBase: TestMessageServiceBase;
  let messageService: TestL2MessageService;
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  let admin: SignerWithAddress;
  let remoteSender: SignerWithAddress;
  let securityCouncil: SignerWithAddress;
  let l1L2MessageSetter: SignerWithAddress;

  async function deployMessageServiceBaseFixture() {
    const roleAddresses = generateRoleAssignments(L2_MESSAGE_SERVICE_ROLES, securityCouncil.address, [
      { role: L1_L2_MESSAGE_SETTER_ROLE, addresses: [l1L2MessageSetter.address] },
    ]);

    const messageService = (await deployUpgradableFromFactory("TestL2MessageService", [
      ONE_DAY_IN_SECONDS,
      INITIAL_WITHDRAW_LIMIT,
      securityCouncil.address,
      roleAddresses,
      pauseTypeRoles,
      unpauseTypeRoles,
    ])) as unknown as TestL2MessageService;

    const messageServiceBase = (await deployUpgradableFromFactory("TestMessageServiceBase", [
      await messageService.getAddress(),
      remoteSender.address,
    ])) as unknown as TestMessageServiceBase;
    return { messageService, messageServiceBase };
  }

  beforeEach(async () => {
    [admin, remoteSender, securityCouncil, l1L2MessageSetter] = await ethers.getSigners();
    const contracts = await loadFixture(deployMessageServiceBaseFixture);
    messageService = contracts.messageService;
    messageServiceBase = contracts.messageServiceBase;
  });

  describe("Initialization checks", () => {
    it("Should revert if message service address is address(0)", async () => {
      await expectRevertWithCustomError(
        messageService,
        deployUpgradableFromFactory("TestMessageServiceBase", [ethers.ZeroAddress, remoteSender.address]),
        "ZeroAddressNotAllowed",
      );
    });

    it("It should fail when not initializing", async () => {
      await expectRevertWithReason(
        messageServiceBase.tryInitialize(await messageService.getAddress(), remoteSender.address),
        INITIALIZED_ERROR_MESSAGE,
      );
    });

    it("Should revert if remote sender address is address(0)", async () => {
      await expectRevertWithCustomError(
        messageServiceBase,
        deployUpgradableFromFactory("TestMessageServiceBase", [await messageService.getAddress(), ethers.ZeroAddress]),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should set the value of remoteSender variable in storage", async () => {
      expect(await messageServiceBase.remoteSender()).to.equal(remoteSender.address);
    });

    it("Should set the value of messageService variable in storage", async () => {
      expect(await messageServiceBase.messageService()).to.equal(await messageService.getAddress());
    });
  });

  describe("RemoteSenderSet event", () => {
    it("Should emit RemoteSenderSet event when testSetRemoteSender is called", async () => {
      const newRemoteSender = ethers.Wallet.createRandom().address;
      await expectEvent(
        messageServiceBase,
        messageServiceBase.testSetRemoteSender(newRemoteSender),
        "RemoteSenderSet",
        [newRemoteSender, admin.address],
      );
    });
  });

  describe("onlyMessagingService() modifier", () => {
    it("Should revert if msg.sender is not the message service address", async () => {
      await expectRevertWithCustomError(
        messageServiceBase,
        messageServiceBase.withOnlyMessagingService(),
        "CallerIsNotMessageService",
      );
    });

    it("Should succeed if msg.sender is the message service address", async () => {
      expect(await messageService.callMessageServiceBase(await messageServiceBase.getAddress())).to.not.be.reverted;
    });
  });

  describe("onlyAuthorizedRemoteSender() modifier", () => {
    it("Should revert if sender is not allowed", async () => {
      await expectRevertWithCustomError(
        messageServiceBase,
        messageServiceBase.withOnlyAuthorizedRemoteSender(),
        "SenderNotAuthorized",
      );
    });

    it("Should succeed if original sender is allowed", async () => {
      const messageServiceBase = (await deployUpgradableFromFactory("TestMessageServiceBase", [
        await messageService.getAddress(),
        "0x00000000000000000000000000000000075BCd15",
      ])) as unknown as TestMessageServiceBase;
      await expect(messageServiceBase.withOnlyAuthorizedRemoteSender()).to.not.be.reverted;
    });
  });
});
