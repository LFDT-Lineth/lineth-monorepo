import { etherToWei, serialize } from "@consensys/linea-shared-utils";
import { describe, expect, it } from "@jest/globals";
import { encodeFunctionData, parseEther, toHex } from "viem";

import { waitForEvents, getMessageSentEventFromLogs, estimateLineaGas } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { getBridgedTokenContract } from "./config/contracts/contracts";
import { createTestContext } from "./config/setup";
import { L2MessageServiceV1Abi, LineaRollupV6Abi, TestERC20Abi, TokenBridgeV1_1Abi } from "./generated";

const context = createTestContext();
const l1AccountManager = context.getL1AccountManager();
const l2AccountManager = context.getL2AccountManager();
const bridgeAmount = parseEther("100");

describe("Bridge ERC20 Tokens L1 -> L2 and L2 -> L1", () => {
  it.concurrent("Bridge a token from L1 to L2", async () => {
    const [l1Account, l2Account] = await Promise.all([
      l1AccountManager.generateAccount(),
      l2AccountManager.generateAccount(),
    ]);

    const l1PublicClient = context.l1PublicClient();
    const l2PublicClient = context.l2PublicClient();

    const l2MessageService = context.l2Contracts.l2MessageService(l2PublicClient);
    const l1TokenBridge = context.l1Contracts.tokenBridge(l1PublicClient);
    const l1Token = context.l1Contracts.testERC20(l1PublicClient);

    logger.debug("Minting ERC20 tokens to L1 Account");

    let { maxPriorityFeePerGas: l1MaxPriorityFeePerGas, maxFeePerGas: l1MaxFeePerGas } =
      await l1PublicClient.estimateFeesPerGas();
    const nonce = await l1PublicClient.getTransactionCount({ address: l1Account.address, blockTag: "latest" });

    logger.debug("Minting and approving tokens to L1 TokenBridge");

    const [mintTxHash, approveTxHash] = await Promise.all([
      l1Token.write.mint([l1Account.address, bridgeAmount], {
        account: l1Account,
        nonce,
        maxPriorityFeePerGas: l1MaxPriorityFeePerGas,
        maxFeePerGas: l1MaxFeePerGas,
      }),
      l1Token.write.approve([l1TokenBridge.address, bridgeAmount], {
        account: l1Account,
        maxPriorityFeePerGas: l1MaxPriorityFeePerGas,
        maxFeePerGas: l1MaxFeePerGas,
        nonce: nonce + 1,
      }),
    ]);

    await l1PublicClient.waitForTransactionReceipt({ hash: mintTxHash, timeout: 60_000 });
    await l1PublicClient.waitForTransactionReceipt({ hash: approveTxHash, timeout: 60_000 });

    const l1TokenBridgeAddress = l1TokenBridge.address;
    const l1TokenAddress = l1Token.address;

    const allowanceL1Account = await l1Token.read.allowance([l1Account.address, l1TokenBridgeAddress]);
    logger.debug(`Current allowance of L1 account to L1 TokenBridge is ${allowanceL1Account.toString()}`);

    logger.debug("Calling the bridgeToken function on the L1 TokenBridge contract");

    ({ maxPriorityFeePerGas: l1MaxPriorityFeePerGas, maxFeePerGas: l1MaxFeePerGas } =
      await l1PublicClient.estimateFeesPerGas());

    const bridgeTokenTxHash = await l1TokenBridge.write.bridgeToken([l1TokenAddress, bridgeAmount, l2Account.address], {
      account: l1Account,
      value: etherToWei("0.01"),
      maxPriorityFeePerGas: l1MaxPriorityFeePerGas,
      maxFeePerGas: l1MaxFeePerGas,
    });

    const bridgedTxReceipt = await l1PublicClient.waitForTransactionReceipt({
      hash: bridgeTokenTxHash,
      timeout: 60_000,
    });

    const messageSentEvents = getMessageSentEventFromLogs([bridgedTxReceipt]);
    expect(messageSentEvents.length).toBeGreaterThan(0);

    const l1TokenBalance = await l1Token.read.balanceOf([l1Account.address]);
    logger.debug(`Token balance of L1 account is ${l1TokenBalance.toString()}`);

    expect(l1TokenBalance).toEqual(0n);

    logger.debug("Waiting for MessageSent event on L1.");

    const messageNumber = messageSentEvents[0].messageNumber;
    const messageHash = messageSentEvents[0].messageHash;

    logger.debug(`Message sent on L1. messageHash=${messageHash}`);

    logger.debug("Waiting for anchoring...");

    const [rollingHashUpdatedEvent] = await waitForEvents(l2PublicClient, {
      abi: L2MessageServiceV1Abi,
      address: l2MessageService.address,
      eventName: "RollingHashUpdated",
      fromBlock: 0n,
      toBlock: "latest",
      pollingIntervalMs: 1_000,
      strict: true,
      criteria: async (events) => events.filter((event) => event.args.messageNumber >= messageNumber),
    });

    expect(rollingHashUpdatedEvent).toBeDefined();

    const anchoredStatus = await l2MessageService.read.inboxL1L2MessageStatus([messageHash]);

    expect(anchoredStatus).toBeGreaterThan(0);

    logger.debug(`Message anchored. event=${serialize(rollingHashUpdatedEvent)}`);

    logger.debug("Waiting for MessageClaimed event on L2...");

    const [claimedEvent] = await waitForEvents(l2PublicClient, {
      abi: L2MessageServiceV1Abi,
      address: l2MessageService.address,
      eventName: "MessageClaimed",
      args: {
        _messageHash: messageHash,
      },
      strict: true,
    });
    expect(claimedEvent).toBeDefined();

    const [newTokenDeployed] = await waitForEvents(l2PublicClient, {
      abi: TokenBridgeV1_1Abi,
      address: context.l2Contracts.tokenBridge(l2PublicClient).address,
      eventName: "NewTokenDeployed",
      strict: true,
    });
    expect(newTokenDeployed).toBeDefined();

    logger.debug(`Message claimed on L2. event=${serialize(claimedEvent)}.`);

    const l2Token = getBridgedTokenContract(l2PublicClient, newTokenDeployed.args.bridgedToken);

    logger.debug("Verify the token balance on L2");

    const l2TokenBalance = await l2Token.read.balanceOf([l2Account.address]);
    logger.debug(`Token balance of L2 account is ${l2TokenBalance.toString()}`);

    expect(l2TokenBalance).toEqual(bridgeAmount);
  });

  it.concurrent("Bridge a token from L2 to L1", async () => {
    const [l1Account, l2Account] = await Promise.all([
      l1AccountManager.generateAccount(),
      l2AccountManager.generateAccount(),
    ]);

    const l1PublicClient = context.l1PublicClient();
    const l2PublicClient = context.l2PublicClient();

    const l2TokenBridge = context.l2Contracts.tokenBridge(l2PublicClient);
    const l2Token = context.l2Contracts.testERC20(l2PublicClient);
    const lineaEstimateGasClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
    const l2TokenAddress = l2Token.address;
    const l2TokenBridgeAddress = l2TokenBridge.address;

    // Mint token
    let lineaEstimateGasFee = await estimateLineaGas(lineaEstimateGasClient, {
      account: l2Account,
      to: l2TokenAddress,
      data: encodeFunctionData({
        abi: TestERC20Abi,
        functionName: "mint",
        args: [l2Account.address, bridgeAmount],
      }),
    });
    const mintTxHash = await l2Token.write.mint([l2Account.address, bridgeAmount], {
      account: l2Account,
      maxPriorityFeePerGas: lineaEstimateGasFee.maxPriorityFeePerGas,
      maxFeePerGas: lineaEstimateGasFee.maxFeePerGas,
      gas: lineaEstimateGasFee.gasLimit,
    });

    const mintTxReceipt = await l2PublicClient.waitForTransactionReceipt({ hash: mintTxHash, timeout: 60_000 });
    logger.debug(`Mint tx receipt received=${serialize(mintTxReceipt)}`);

    // Approve token
    lineaEstimateGasFee = await estimateLineaGas(lineaEstimateGasClient, {
      account: l2Account,
      to: l2TokenAddress,
      data: encodeFunctionData({
        abi: TestERC20Abi,
        functionName: "approve",
        args: [l2TokenBridgeAddress, bridgeAmount],
      }),
    });
    const approveTxHash = await l2Token.write.approve([l2TokenBridgeAddress, bridgeAmount], {
      account: l2Account,
      maxPriorityFeePerGas: lineaEstimateGasFee.maxPriorityFeePerGas,
      maxFeePerGas: lineaEstimateGasFee.maxFeePerGas,
      gas: lineaEstimateGasFee.gasLimit,
    });
    const approveTxReceipt = await l2PublicClient.waitForTransactionReceipt({ hash: approveTxHash, timeout: 60_000 });
    logger.debug(`Approve tx receipt received=${serialize(approveTxReceipt)}`);

    // Retrieve token allowance
    const allowanceL2Account = await l2Token.read.allowance([l2Account.address, l2TokenBridgeAddress]);
    logger.debug(`Current allowance of L2 account to L2 TokenBridge is ${allowanceL2Account.toString()}`);
    logger.debug(`Current balance of L2 account is ${await l2Token.read.balanceOf([l2Account.address])}`);

    logger.debug("Calling the bridgeToken function on the L2 TokenBridge contract");

    // Bridge token
    logger.debug(`0.01 ether = ${toHex(etherToWei("0.01"))} wei`);

    lineaEstimateGasFee = await estimateLineaGas(lineaEstimateGasClient, {
      account: l2Account,
      to: l2TokenBridgeAddress,
      data: encodeFunctionData({
        abi: TokenBridgeV1_1Abi,
        functionName: "bridgeToken",
        args: [l2TokenAddress, bridgeAmount, l1Account.address],
      }),
      value: etherToWei("0.01"),
    });

    const bridgeTokenTxHash = await l2TokenBridge.write.bridgeToken(
      [l2Token.address, bridgeAmount, l1Account.address],
      {
        account: l2Account,
        value: etherToWei("0.01"),
        maxPriorityFeePerGas: lineaEstimateGasFee.maxPriorityFeePerGas,
        maxFeePerGas: lineaEstimateGasFee.maxFeePerGas,
        gas: lineaEstimateGasFee.gasLimit,
      },
    );
    const bridgeTxReceipt = await l2PublicClient.waitForTransactionReceipt({
      hash: bridgeTokenTxHash,
      timeout: 60_000,
    });
    logger.debug(`Bridge tx receipt received=${serialize(bridgeTxReceipt)}`);

    const messageSentEvents = getMessageSentEventFromLogs([bridgeTxReceipt]);

    expect(messageSentEvents.length).toBeGreaterThan(0);
    const messageHash = messageSentEvents[0].messageHash;

    logger.debug("Waiting for L1 MessageClaimed event.");

    const [claimedEvent] = await waitForEvents(l1PublicClient, {
      abi: LineaRollupV6Abi,
      address: context.l1Contracts.lineaRollup(l1PublicClient).address,
      eventName: "MessageClaimed",
      args: {
        _messageHash: messageHash,
      },
      strict: true,
    });
    expect(claimedEvent).toBeDefined();

    logger.debug(`Message claimed on L1. event=${serialize(claimedEvent)}`);

    const [newTokenDeployed] = await waitForEvents(l1PublicClient, {
      abi: TokenBridgeV1_1Abi,
      address: context.l1Contracts.tokenBridge(l1PublicClient).address,
      eventName: "NewTokenDeployed",
      strict: true,
    });
    expect(newTokenDeployed).toBeDefined();

    const l1BridgedToken = getBridgedTokenContract(l1PublicClient, newTokenDeployed.args.bridgedToken);

    logger.debug("Verify the token balance on L1");

    const l1BridgedTokenBalance = await l1BridgedToken.read.balanceOf([l1Account.address]);
    logger.debug(`Token balance of L1 account is ${l1BridgedTokenBalance.toString()}`);

    expect(l1BridgedTokenBalance).toEqual(bridgeAmount);
  });
});
