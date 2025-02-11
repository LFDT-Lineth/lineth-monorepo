import { ethers } from "ethers";
import { describe, expect, it } from "@jest/globals";
import { config } from "./config/tests-config";
import { waitForEvents, etherToWei, LineaEstimateGasClient } from "./common/utils";
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
    logger.debug(`Current allowance of L1 account to L1 TokenBridge is ${allowanceL1Account.toString()}`);

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
    logger.debug(`Token balance of L1 account is ${l1TokenBalance.toString()}`);

    expect(l1TokenBalance).toEqual(0n);

    logger.debug("Waiting for MessageSent event on L1.");

    const messageNumber = messageSentEvent[messageSentEventMessageNumberIndex];
    const messageHash = messageSentEvent[messageSentEventMessageHashIndex];

    logger.debug(`Message sent on L1. messageHash=${messageHash}`);

    logger.debug("Waiting for anchoring...");

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

    logger.debug(`Message anchored. event=${JSON.stringify(rollingHashUpdatedEvent)}`);

    logger.debug("Waiting for MessageClaimed event on L2...");

    const [claimedEvent] = await waitForEvents(l2MessageService, l2MessageService.filters.MessageClaimed(messageHash));
    expect(claimedEvent).not.toBeNull();

    const [newTokenDeployed] = await waitForEvents(l2TokenBridge, l2TokenBridge.filters.NewTokenDeployed());
    expect(newTokenDeployed).not.toBeNull();

    logger.debug(`Message claimed on L2. event=${JSON.stringify(claimedEvent)}.`);

    const l2Token = config.getL2BridgedTokenContract(newTokenDeployed.args.bridgedToken);

    logger.debug("Verify the token balance on L2");

    const l2TokenBalance = await l2Token.balanceOf(l2Account.address);
    logger.debug(`Token balance of L2 account is ${l2TokenBalance.toString()}`);

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
    const lineaEstimateGasClient = new LineaEstimateGasClient(config.getL2BesuNodeEndpoint()!);
    const l2TokenAddress = await l2Token.getAddress();
    const l2TokenBridgeAddress = await l2TokenBridge.getAddress();

    // Mint token
    let lineaEstimateGasFee = await lineaEstimateGasClient.lineaEstimateGas(
      l2Account.address,
      l2TokenAddress,
      l2Token.interface.encodeFunctionData("mint", [l2Account.address, bridgeAmount]),
    );
    let nonce = await l2Provider.getTransactionCount(l2Account.address, "pending");
    const mintResponse = await l2Token.connect(l2Account).mint(l2Account.address, bridgeAmount, {
      maxPriorityFeePerGas: lineaEstimateGasFee.maxPriorityFeePerGas,
      maxFeePerGas: lineaEstimateGasFee.maxFeePerGas,
      gasLimit: lineaEstimateGasFee.gasLimit,
      nonce: nonce,
    });
    const mintTxReceipt = await mintResponse.wait();
    logger.debug(`Mint tx receipt received=${JSON.stringify(mintTxReceipt)}`);

    // Approve token
    lineaEstimateGasFee = await lineaEstimateGasClient.lineaEstimateGas(
      l2Account.address,
      l2TokenAddress,
      l2Token.interface.encodeFunctionData("approve", [l2TokenBridgeAddress, ethers.parseEther("100")]),
    );
    nonce = await l2Provider.getTransactionCount(l2Account.address, "pending");
    const approveResponse = await l2Token.connect(l2Account).approve(l2TokenBridgeAddress, ethers.parseEther("100"), {
      maxPriorityFeePerGas: lineaEstimateGasFee.maxPriorityFeePerGas,
      maxFeePerGas: lineaEstimateGasFee.maxFeePerGas,
      gasLimit: lineaEstimateGasFee.gasLimit,
      nonce: nonce,
    });
    const approveTxReceipt = await approveResponse.wait();
    logger.debug(`Approve tx receipt received=${JSON.stringify(approveTxReceipt)}`);

    // Retrieve token allowance
    const allowanceL2Account = await l2Token.allowance(l2Account.address, l2TokenBridgeAddress);
    logger.debug(`Current allowance of L2 account to L2 TokenBridge is ${allowanceL2Account.toString()}`);
    logger.debug(`Current balance of L2 account is ${await l2Token.balanceOf(l2Account)}`);

    logger.debug("Calling the bridgeToken function on the L2 TokenBridge contract");

    // Bridge token
    logger.debug(`0.01 ether = ${etherToWei("0.01").toString(16)}`);
    nonce = await l2Provider.getTransactionCount(l2Account.address, "pending");

    lineaEstimateGasFee = await lineaEstimateGasClient.lineaEstimateGas(
      l2Account.address,
      l2TokenBridgeAddress,
      l2TokenBridge.interface.encodeFunctionData("bridgeToken", [l2TokenAddress, bridgeAmount, l1Account.address]),
      etherToWei("0.01").toString(16),
    );

    const bridgeResponse = await l2TokenBridge
      .connect(l2Account)
      .bridgeToken(await l2Token.getAddress(), bridgeAmount, l1Account.address, {
        value: etherToWei("0.01"),
        maxPriorityFeePerGas: lineaEstimateGasFee.maxPriorityFeePerGas,
        maxFeePerGas: lineaEstimateGasFee.maxFeePerGas,
        gasLimit: lineaEstimateGasFee.gasLimit,
        nonce: nonce,
      });
    const bridgeTxReceipt = await bridgeResponse.wait();
    logger.debug(`Bridge tx receipt received=${JSON.stringify(bridgeTxReceipt)}`);

    const sentEventLog = bridgeTxReceipt?.logs.find((log) => log.topics[0] == MESSAGE_SENT_EVENT_SIGNATURE);

    const messageSentEvent = l2MessageService.interface.decodeEventLog(
      "MessageSent",
      sentEventLog!.data,
      sentEventLog!.topics,
    );
    const messageHash = messageSentEvent[messageSentEventMessageHashIndex];

    logger.debug("Waiting for L1 MessageClaimed event.");

    const [claimedEvent] = await waitForEvents(lineaRollup, lineaRollup.filters.MessageClaimed(messageHash));
    expect(claimedEvent).not.toBeNull();

    logger.debug(`Message claimed on L1. event=${JSON.stringify(claimedEvent)}`);

    const [newTokenDeployed] = await waitForEvents(l1TokenBridge, l1TokenBridge.filters.NewTokenDeployed());
    expect(newTokenDeployed).not.toBeNull();

    const l1BridgedToken = config.getL1BridgedTokenContract(newTokenDeployed.args.bridgedToken);

    logger.debug("Verify the token balance on L1");

    const l1BridgedTokenBalance = await l1BridgedToken.balanceOf(l1Account.address);
    logger.debug(`Token balance of L1 account is ${l1BridgedTokenBalance.toString()}`);

    expect(l1BridgedTokenBalance).toEqual(bridgeAmount);
  });
});
