import { etherToWei } from "@consensys/linea-shared-utils";
import { describe, expect, it } from "@jest/globals";

import { sendL1ToL2Message, sendL2ToL1Message } from "./common/test-helpers/messaging";
import { getMessageSentEventFromLogs, waitForEvents } from "./common/utils";
import { createTestContext } from "./config/setup";
import { L2MessageServiceV1Abi, LineaRollupV6Abi } from "./generated";

const context = createTestContext();
const l1AccountManager = context.getL1AccountManager();
const l2AccountManager = context.getL2AccountManager();

describe("Messaging test suite", () => {
  it.concurrent(
    "Should send a transaction with fee and calldata to L1 message service, be successfully claimed it on L2",
    async () => {
      const l1Account = await l1AccountManager.generateAccount();
      const l2PublicClient = context.l2PublicClient();
      const l2BlockBeforeSend = await l2PublicClient.getBlockNumber();

      const { txHash, receipt } = await sendL1ToL2Message(context, {
        account: l1Account,
        fee: etherToWei("0.1"),
        value: etherToWei("0.1"),
        withCalldata: true,
      });

      const [messageSentEvent] = getMessageSentEventFromLogs([receipt]);
      const messageHash = messageSentEvent.messageHash;
      logger.debug(`L1 message sent. messageHash=${messageHash} transactionHash=${txHash}`);

      logger.debug(`Waiting for MessageClaimed event on L2. messageHash=${messageHash}`);
      const [messageClaimedEvent] = await waitForEvents(l2PublicClient, {
        abi: L2MessageServiceV1Abi,
        address: context.l2Contracts.l2MessageService(l2PublicClient).address,
        eventName: "MessageClaimed",
        args: {
          _messageHash: messageHash,
        },
        fromBlock: l2BlockBeforeSend,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
      });

      expect(messageClaimedEvent).toBeDefined();
      logger.debug(
        `Message claimed on L2. messageHash=${messageClaimedEvent.args._messageHash} transactionHash=${messageClaimedEvent.transactionHash}`,
      );
    },
    150_000,
  );

  it.concurrent(
    "Should send a transaction with fee and without calldata to L1 message service, be successfully claimed it on L2",
    async () => {
      const l1Account = await l1AccountManager.generateAccount();
      const l2PublicClient = context.l2PublicClient();
      const l2BlockBeforeSend = await l2PublicClient.getBlockNumber();

      const { txHash, receipt } = await sendL1ToL2Message(context, {
        account: l1Account,
        fee: etherToWei("0.1"),
        value: etherToWei("0.2"),
        withCalldata: false,
      });

      const [messageSentEvent] = getMessageSentEventFromLogs([receipt]);
      const messageHash = messageSentEvent.messageHash;
      logger.debug(`L1 message sent. messageHash=${messageHash} transactionHash=${txHash}`);

      logger.debug(`Waiting for MessageClaimed event on L2. messageHash=${messageHash}`);
      const [messageClaimedEvent] = await waitForEvents(l2PublicClient, {
        abi: L2MessageServiceV1Abi,
        address: context.l2Contracts.l2MessageService(l2PublicClient).address,
        eventName: "MessageClaimed",
        args: {
          _messageHash: messageHash,
        },
        fromBlock: l2BlockBeforeSend,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
      });
      expect(messageClaimedEvent).toBeDefined();
      logger.debug(
        `Message claimed on L2. messageHash=${messageClaimedEvent.args._messageHash} transactionHash=${messageClaimedEvent.transactionHash}`,
      );
    },
    150_000,
  );

  // Test that Postman sponsoring works for L1->L2
  it.concurrent(
    "Should send a transaction without fee and without calldata to L1 message service, be successfully claimed it on L2",
    async () => {
      const l1Account = await l1AccountManager.generateAccount();
      const l2PublicClient = context.l2PublicClient();
      const l2BlockBeforeSend = await l2PublicClient.getBlockNumber();

      const { txHash, receipt } = await sendL1ToL2Message(context, {
        account: l1Account,
        withCalldata: false,
      });

      const [messageSentEvent] = getMessageSentEventFromLogs([receipt]);
      const messageHash = messageSentEvent.messageHash;
      logger.debug(`L1 message sent. messageHash=${messageHash} transactionHash=${txHash}`);

      logger.debug(`Waiting for MessageClaimed event on L2. messageHash=${messageHash}`);
      const [messageClaimedEvent] = await waitForEvents(l2PublicClient, {
        abi: L2MessageServiceV1Abi,
        address: context.l2Contracts.l2MessageService(l2PublicClient).address,
        eventName: "MessageClaimed",
        args: {
          _messageHash: messageHash,
        },
        fromBlock: l2BlockBeforeSend,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
      });

      expect(messageClaimedEvent).toBeDefined();
      logger.debug(
        `Message claimed on L2. messageHash=${messageClaimedEvent.args._messageHash} transactionHash=${messageClaimedEvent.transactionHash}`,
      );
    },
    150_000,
  );

  it.concurrent(
    "Should send a transaction with fee and calldata to L2 message service, be successfully claimed it on L1",
    async () => {
      const l2Account = await l2AccountManager.generateAccount();
      const l1PublicClient = context.l1PublicClient();
      const lineaRollup = context.l1Contracts.lineaRollup(l1PublicClient);
      const l1BlockBeforeSend = await l1PublicClient.getBlockNumber();

      const { txHash, receipt } = await sendL2ToL1Message(context, {
        account: l2Account,
        fee: etherToWei("0.001"),
        value: etherToWei("0.001"),
        withCalldata: true,
      });

      const [messageSentEvent] = getMessageSentEventFromLogs([receipt]);
      const messageHash = messageSentEvent.messageHash;
      logger.debug(`L2 message sent. messageHash=${messageHash} transactionHash=${txHash}`);

      logger.debug(`Waiting for L2MessagingBlockAnchored event... blockNumber=${messageSentEvent.blockNumber}`);
      const [l2MessagingBlockAnchoredEvent] = await waitForEvents(l1PublicClient, {
        abi: LineaRollupV6Abi,
        address: lineaRollup.address,
        eventName: "L2MessagingBlockAnchored",
        args: {
          l2Block: messageSentEvent.blockNumber,
        },
        fromBlock: l1BlockBeforeSend,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
      });

      expect(l2MessagingBlockAnchoredEvent).toBeDefined();

      logger.debug(`Waiting for MessageClaimed event on L1... messageHash=${messageHash}`);
      const [messageClaimedEvent] = await waitForEvents(l1PublicClient, {
        abi: LineaRollupV6Abi,
        address: lineaRollup.address,
        eventName: "MessageClaimed",
        args: {
          _messageHash: messageHash,
        },
        fromBlock: l1BlockBeforeSend,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
      });

      expect(messageClaimedEvent).toBeDefined();
      logger.debug(
        `Message claimed on L1. messageHash=${messageClaimedEvent.args._messageHash} transactionHash=${messageClaimedEvent.transactionHash}`,
      );
    },
    150_000,
  );

  it.concurrent(
    "Should send a transaction with fee and without calldata to L2 message service, be successfully claimed it on L1",
    async () => {
      const l2Account = await l2AccountManager.generateAccount();
      const l1PublicClient = context.l1PublicClient();
      const lineaRollup = context.l1Contracts.lineaRollup(l1PublicClient);
      const l1BlockBeforeSend = await l1PublicClient.getBlockNumber();

      const { txHash, receipt } = await sendL2ToL1Message(context, {
        account: l2Account,
        fee: etherToWei("0.001"),
        value: etherToWei("0.01"),
        withCalldata: false,
      });

      const [messageSentEvent] = getMessageSentEventFromLogs([receipt]);
      const messageHash = messageSentEvent.messageHash;
      logger.debug(`L2 message sent. messageHash=${messageHash} transactionHash=${txHash}`);

      logger.debug(`Waiting for L2MessagingBlockAnchored event... blockNumber=${messageSentEvent.blockNumber}`);
      const [l2MessagingBlockAnchoredEvent] = await waitForEvents(l1PublicClient, {
        abi: LineaRollupV6Abi,
        address: lineaRollup.address,
        eventName: "L2MessagingBlockAnchored",
        args: {
          l2Block: messageSentEvent.blockNumber,
        },
        fromBlock: l1BlockBeforeSend,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
      });

      expect(l2MessagingBlockAnchoredEvent).toBeDefined();

      logger.debug(`Waiting for MessageClaimed event on L1. messageHash=${messageHash}`);
      const [messageClaimedEvent] = await waitForEvents(l1PublicClient, {
        abi: LineaRollupV6Abi,
        address: lineaRollup.address,
        eventName: "MessageClaimed",
        args: {
          _messageHash: messageHash,
        },
        fromBlock: l1BlockBeforeSend,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
      });

      expect(messageClaimedEvent).toBeDefined();

      logger.debug(
        `Message claimed on L1. messageHash=${messageClaimedEvent.args._messageHash} transactionHash=${messageClaimedEvent.transactionHash}`,
      );
    },
    150_000,
  );
});
