import { ethers } from "ethers";
import { describe, expect, it } from "@jest/globals";
import { config } from "./config/tests-config";
import { waitForEvents, etherToWei } from "./common/utils";
import { MESSAGE_SENT_EVENT_SIGNATURE } from "./common/constants";

const l1AccountManager = config.getL1AccountManager();
const l2AccountManager = config.getL2AccountManager();
const bridgeAmount = ethers.parseEther("100");
const messageSentEventMessageNumberIndex = 4;
const messageSentEventMessageHashIndex = 6;

describe("Bridge ERC20 Tokens L1 -> L2 and L2 -> L1", () => {
  it.concurrent("Bridge a token from L1 to L2", async () => {
    const [l1Account, l2Account] = await Promise.all([
      l1AccountManager.generateAccount(),
      l2AccountManager.generateAccount(),
    ]);

    const lineaRollup = config.getLineaRollupContract();
    const l2MessageService = config.getL2MessageServiceContract();
    const l1TokenBridge = config.getL1TokenBridgeContract();
    const l2TokenBridge = config.getL2TokenBridgeContract();
    const l1Token = config.getL1TokenContract();
    const l1Provider = config.getL1Provider();

    logger.debug("Minting ERC20 tokens to L1 Account");

    let { maxPriorityFeePerGas: l1MaxPriorityFeePerGas, maxFeePerGas: l1MaxFeePerGas } = await l1Provider.getFeeData();
    let nonce = await l1Provider.getTransactionCount(l1Account.address, "pending");

    logger.debug("Minting and approving tokens to L1 TokenBridge");

    await Promise.all([
      (
        await l1Token.connect(l1Account).mint(l1Account.address, bridgeAmount, {
          nonce: nonce,
          maxPriorityFeePerGas: l1MaxPriorityFeePerGas,
          maxFeePerGas: l1MaxFeePerGas,
        })
      ).wait(),
      (
        await l1Token.connect(l1Account).approve(l1TokenBridge.getAddress(), bridgeAmount, {
          maxPriorityFeePerGas: l1MaxPriorityFeePerGas,
          maxFeePerGas: l1MaxFeePerGas,
          nonce: nonce + 1,
        })
      ).wait(),
    ]);

    const l1TokenBridgeAddress = await l1TokenBridge.getAddress();
    const l1TokenAddress = await l1Token.getAddress();

    const allowanceL1Account = await l1Token.allowance(l1Account.address, l1TokenBridgeAddress);
    logger.info(`Current allowance of L1 account to L1 TokenBridge is ${allowanceL1Account.toString()}`);

    logger.debug("Calling the bridgeToken function on the L1 TokenBridge contract");

    ({ maxPriorityFeePerGas: l1MaxPriorityFeePerGas, maxFeePerGas: l1MaxFeePerGas } = await l1Provider.getFeeData());
    nonce = await l1Provider.getTransactionCount(l1Account.address, "pending");

    const bridgeTokenTx = await l1TokenBridge
      .connect(l1Account)
      .bridgeToken(l1TokenAddress, bridgeAmount, l2Account.address, {
        value: etherToWei("0.01"),
        maxPriorityFeePerGas: l1MaxPriorityFeePerGas,
        maxFeePerGas: l1MaxFeePerGas,
        nonce: nonce,
      });

    const bridgedTxReceipt = await bridgeTokenTx.wait();

    const sentEventLog = bridgedTxReceipt?.logs.find((log) => log.topics[0] == MESSAGE_SENT_EVENT_SIGNATURE);

    const messageSentEvent = lineaRollup.interface.decodeEventLog(
      "MessageSent",
      sentEventLog!.data,
      sentEventLog!.topics,
    );

    const l1TokenBalance = await l1Token.balanceOf(l1Account.address);
    logger.info(`Token balance of L1 account is ${l1TokenBalance.toString()}`);

    expect(l1TokenBalance).toEqual(0n);

    logger.info("Waiting for MessageSent event on L1.");

    const messageNumber = messageSentEvent[messageSentEventMessageNumberIndex];
    const messageHash = messageSentEvent[messageSentEventMessageHashIndex];

    logger.info(`Message sent on L1. messageHash=${messageHash}`);

    logger.info("Waiting for anchoring...");

    const [rollingHashUpdatedEvent] = await waitForEvents(
      l2MessageService,
      l2MessageService.filters.RollingHashUpdated(),
      1_000,
      0,
      "latest",
      async (events) => events.filter((event) => event.args.messageNumber >= messageNumber),
    );
    expect(rollingHashUpdatedEvent).not.toBeNull();

    const anchoredStatus = await l2MessageService.inboxL1L2MessageStatus(messageHash);

    expect(anchoredStatus).toBeGreaterThan(0);

    logger.info(`Message anchored. event=${JSON.stringify(rollingHashUpdatedEvent)}`);

    logger.info("Waiting for MessageClaimed event on L2...");

    const [claimedEvent] = await waitForEvents(l2MessageService, l2MessageService.filters.MessageClaimed(messageHash));
    expect(claimedEvent).not.toBeNull();

    const [newTokenDeployed] = await waitForEvents(l2TokenBridge, l2TokenBridge.filters.NewTokenDeployed());
    expect(newTokenDeployed).not.toBeNull();

    logger.info(`Message claimed on L2. event=${JSON.stringify(claimedEvent)}.`);

    const l2Token = config.getL2BridgedTokenContract(newTokenDeployed.args.bridgedToken);

    logger.info("Verify the token balance on L2");

    const l2TokenBalance = await l2Token.balanceOf(l2Account.address);
    logger.info(`Token balance of L2 account is ${l2TokenBalance.toString()}`);

    expect(l2TokenBalance).toEqual(bridgeAmount);
  });

  it.concurrent("Bridge a token from L2 to L1", async () => {
    const [l1Account, l2Account] = await Promise.all([
      l1AccountManager.generateAccount(),
      l2AccountManager.generateAccount(),
    ]);

    const lineaRollup = config.getLineaRollupContract();
    const l2MessageService = config.getL2MessageServiceContract();
    const l1TokenBridge = config.getL1TokenBridgeContract();
    const l2TokenBridge = config.getL2TokenBridgeContract();
    const l2Token = config.getL2TokenContract();
    const l2Provider = config.getL2Provider();

    const { maxPriorityFeePerGas: l2MaxPriorityFeePerGas, maxFeePerGas: l2MaxFeePerGas } =
      await l2Provider.getFeeData();
    let nonce = await l2Provider.getTransactionCount(l2Account.address, "pending");

    await Promise.all([
      (
        await l2Token.connect(l2Account).mint(l2Account.address, bridgeAmount, {
          nonce: nonce,
          maxPriorityFeePerGas: l2MaxPriorityFeePerGas,
          maxFeePerGas: l2MaxFeePerGas,
        })
      ).wait(),
      (
        await l2Token.connect(l2Account).approve(l2TokenBridge.getAddress(), ethers.parseEther("100"), {
          maxPriorityFeePerGas: l2MaxPriorityFeePerGas,
          maxFeePerGas: l2MaxFeePerGas,
          nonce: nonce + 1,
        })
      ).wait(),
    ]);

    const allowanceL2Account = await l2Token.allowance(l2Account.address, l2TokenBridge.getAddress());
    logger.info(`Current allowance of L2 account to L2 TokenBridge is ${allowanceL2Account.toString()}`);
    logger.info(`Current balance of  L2 account is ${await l2Token.balanceOf(l2Account)}`);

    logger.debug("Calling the bridgeToken function on the L2 TokenBridge contract");

    nonce = await l2Provider.getTransactionCount(l2Account.address, "pending");

    const bridgeTokenTx = await l2TokenBridge
      .connect(l2Account)
      .bridgeToken(await l2Token.getAddress(), bridgeAmount, l1Account.address, {
        value: etherToWei("0.01"),
        maxPriorityFeePerGas: l2MaxPriorityFeePerGas,
        maxFeePerGas: l2MaxFeePerGas,
        nonce: nonce,
      });

    const receipt = await bridgeTokenTx.wait();
    const sentEventLog = receipt?.logs.find((log) => log.topics[0] == MESSAGE_SENT_EVENT_SIGNATURE);

    const messageSentEvent = l2MessageService.interface.decodeEventLog(
      "MessageSent",
      sentEventLog!.data,
      sentEventLog!.topics,
    );
    const messageHash = messageSentEvent[messageSentEventMessageHashIndex];

    logger.info("Waiting for L1 MessageClaimed event.");

    const [claimedEvent] = await waitForEvents(lineaRollup, lineaRollup.filters.MessageClaimed(messageHash));
    expect(claimedEvent).not.toBeNull();

    logger.info(`Message claimed on L1. event=${JSON.stringify(claimedEvent)}`);

    const [newTokenDeployed] = await waitForEvents(l1TokenBridge, l1TokenBridge.filters.NewTokenDeployed());
    expect(newTokenDeployed).not.toBeNull();

    const l1BridgedToken = config.getL1BridgedTokenContract(newTokenDeployed.args.bridgedToken);

    logger.info("Verify the token balance on L1");

    const l1BridgedTokenBalance = await l1BridgedToken.balanceOf(l1Account.address);
    logger.info(`Token balance of L1 account is ${l1BridgedTokenBalance.toString()}`);

    expect(l1BridgedTokenBalance).toEqual(bridgeAmount);
  });
});
