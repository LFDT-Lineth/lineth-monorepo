import { ethers, Wallet } from "ethers";
import { describe, expect, it } from "@jest/globals";
import type { Logger } from "winston";
import { config } from "./config/tests-config";
import { encodeFunctionCall, etherToWei, waitForEvents } from "./common/utils";
import { MESSAGE_SENT_EVENT_SIGNATURE } from "./common/constants";

async function sendL1ToL2Message(
  logger: Logger,
  {
    l1Account,
    l2Account,
    withCalldata = false,
  }: {
    l1Account: Wallet;
    l2Account: Wallet;
    withCalldata: boolean;
  },
) {
  const dummyContract = config.getL2DummyContract(l2Account);
  const lineaRollup = config.getLineaRollupContract(l1Account);

  const valueAndFee = etherToWei("1.1");
  const calldata = withCalldata
    ? encodeFunctionCall(dummyContract.interface, "setPayload", [ethers.randomBytes(100)])
    : "0x";
  const destinationAddress = withCalldata
    ? await dummyContract.getAddress()
    : "0x8D97689C9818892B700e27F316cc3E41e17fBeb9";

  const l1Provider = config.getL1Provider();
  const { maxPriorityFeePerGas, maxFeePerGas } = await l1Provider.getFeeData();
  const nonce = await l1Provider.getTransactionCount(l1Account.address, "pending");
  const tx = await lineaRollup.sendMessage(destinationAddress, valueAndFee, calldata, {
    value: valueAndFee,
    nonce,
    maxPriorityFeePerGas,
    maxFeePerGas,
  });

  let receipt = await tx.wait();
  while (!receipt) {
    logger.info("Waiting for transaction to be mined...");
    receipt = await tx.wait();
  }

  return { tx, receipt };
}

async function sendL2ToL1Message(
  logger: Logger,
  {
    l1Account,
    l2Account,
    withCalldata = false,
  }: {
    l1Account: Wallet;
    l2Account: Wallet;
    withCalldata: boolean;
  },
) {
  const l2Provider = config.getL2Provider();
  const dummyContract = config.getL1DummyContract(l1Account);
  const l2MessageService = config.getL2MessageServiceContract(l2Account);

  const valueAndFee = etherToWei("0.001");
  const calldata = withCalldata
    ? encodeFunctionCall(dummyContract.interface, "setPayload", [ethers.randomBytes(100)])
    : "0x";

  const destinationAddress = withCalldata ? await dummyContract.getAddress() : l1Account.address;
  const nonce = await l2Provider.getTransactionCount(l2Account.address, "pending");
  const { maxPriorityFeePerGas, maxFeePerGas } = await l2Provider.getFeeData();

  const tx = await l2MessageService.sendMessage(destinationAddress, valueAndFee, calldata, {
    value: valueAndFee,
    nonce,
    maxPriorityFeePerGas,
    maxFeePerGas,
  });

  let receipt = await tx.wait();

  while (!receipt) {
    logger.info("Waiting for transaction to be mined...");
    receipt = await tx.wait();
  }

  return { tx, receipt };
}

const l1AccountManager = config.getL1AccountManager();
const l2AccountManager = config.getL2AccountManager();

describe("Messaging test suite", () => {
  it.concurrent(
    "Should send a transaction with calldata to L1 message service, be successfully claimed it on L2",
    async () => {
      const [l1Account, l2Account] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const { tx, receipt } = await sendL1ToL2Message(logger, { l1Account, l2Account, withCalldata: true });

      const [messageSentEvent] = receipt.logs.filter((log) => log.topics[0] === MESSAGE_SENT_EVENT_SIGNATURE);
      const messageHash = messageSentEvent.topics[3];
      logger.info(`L1 message sent. messageHash=${messageHash} transaction=${JSON.stringify(tx)}`);

      logger.info("Waiting for MessageClaimed event on L2.");
      const l2MessageService = config.getL2MessageServiceContract();
      const [messageClaimedEvent] = await waitForEvents(
        l2MessageService,
        l2MessageService.filters.MessageClaimed(messageHash),
      );

      logger.info(`Message claimed on L2. event=${JSON.stringify(messageClaimedEvent)}`);
      expect(messageClaimedEvent).toBeDefined();
    },
    100_000,
  );

  it.concurrent(
    "Should send a transaction without calldata to L1 message service, be successfully claimed it on L2",
    async () => {
      const [l1Account, l2Account] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const { tx, receipt } = await sendL1ToL2Message(logger, { l1Account, l2Account, withCalldata: false });

      const [messageSentEvent] = receipt.logs.filter((log) => log.topics[0] === MESSAGE_SENT_EVENT_SIGNATURE);
      const messageHash = messageSentEvent.topics[3];
      logger.info(`L1 message sent. messageHash=${messageHash} transaction=${JSON.stringify(tx)}`);

      logger.info("Waiting for MessageClaimed event on L2.");
      const l2MessageService = config.getL2MessageServiceContract();
      const [messageClaimedEvent] = await waitForEvents(
        l2MessageService,
        l2MessageService.filters.MessageClaimed(messageHash),
      );
      logger.info(`Message claimed on L2. event=${JSON.stringify(messageClaimedEvent)}`);
      expect(messageClaimedEvent).toBeDefined();
    },
    100_000,
  );

  it.concurrent(
    "Should send a transaction with calldata to L2 message service, be successfully claimed it on L1",
    async () => {
      const [l1Account, l2Account] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const lineaRollup = config.getLineaRollupContract();
      const { tx, receipt } = await sendL2ToL1Message(logger, { l1Account, l2Account, withCalldata: true });

      const [messageSentEvent] = receipt.logs.filter((log) => log.topics[0] === MESSAGE_SENT_EVENT_SIGNATURE);
      const messageHash = messageSentEvent.topics[3];
      logger.info(`L2 message sent. messageHash=${messageHash} transaction=${JSON.stringify(tx)}`);

      logger.info(`Waiting for L2MessagingBlockAnchored with blockNumber=${messageSentEvent.blockNumber}...`);
      await waitForEvents(
        lineaRollup,
        lineaRollup.filters.L2MessagingBlockAnchored(messageSentEvent.blockNumber),
        1_000,
      );

      logger.info("Waiting for MessageClaimed event on L1.");
      const [messageClaimedEvent] = await waitForEvents(
        lineaRollup,
        lineaRollup.filters.MessageClaimed(messageHash),
        1_000,
      );

      logger.info(`Message claimed on L1. event=${JSON.stringify(messageClaimedEvent)}`);
      expect(messageClaimedEvent).toBeDefined();
    },
    150_000,
  );

  it.concurrent(
    "Should send a transaction without calldata to L2 message service, be successfully claimed it on L1",
    async () => {
      const [l1Account, l2Account] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const lineaRollup = config.getLineaRollupContract();
      const { tx, receipt } = await sendL2ToL1Message(logger, { l1Account, l2Account, withCalldata: false });

      const [messageSentEvent] = receipt.logs.filter((log) => log.topics[0] === MESSAGE_SENT_EVENT_SIGNATURE);
      const messageHash = messageSentEvent.topics[3];
      logger.info(`L2 message sent. messageHash=${messageHash} transaction=${JSON.stringify(tx)}`);

      logger.info(`Waiting for L2MessagingBlockAnchored with blockNumber=${messageSentEvent.blockNumber}...`);
      await waitForEvents(
        lineaRollup,
        lineaRollup.filters.L2MessagingBlockAnchored(messageSentEvent.blockNumber),
        1_000,
      );

      logger.info("Waiting for MessageClaimed event on L1.");
      const [messageClaimedEvent] = await waitForEvents(
        lineaRollup,
        lineaRollup.filters.MessageClaimed(messageHash),
        1_000,
      );

      logger.info(`Message claimed on L1. event=${JSON.stringify(messageClaimedEvent)}`);
      expect(messageClaimedEvent).toBeDefined();
    },
    150_000,
  );
});
