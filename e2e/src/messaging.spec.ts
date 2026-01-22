import { ethers, toBeHex, Wallet } from "ethers";
import { describe, expect, it } from "@jest/globals";
import type { Logger } from "winston";
import { config } from "./config/tests-config";
import { encodeFunctionCall, etherToWei, LineaEstimateGasClient, waitForEvents } from "./common/utils";
import { MESSAGE_SENT_EVENT_SIGNATURE } from "./common/constants";

async function sendL1ToL2Message(
  logger: Logger,
  {
    l1Account,
    l2Account,
    fee = 0n,
    withCalldata = false,
  }: {
    l1Account: Wallet;
    l2Account: Wallet;
    fee: bigint;
    withCalldata: boolean;
  },
) {
  const dummyContract = config.getL2DummyContract(l2Account);
  const lineaRollup = config.getLineaRollupContract(l1Account);

  const calldata = withCalldata
    ? encodeFunctionCall(dummyContract.interface, "setPayload", [ethers.randomBytes(100)])
    : "0x";
  const destinationAddress = withCalldata
    ? await dummyContract.getAddress()
    : "0x8D97689C9818892B700e27F316cc3E41e17fBeb9";

  const l1Provider = config.getL1Provider();
  const { maxPriorityFeePerGas, maxFeePerGas } = await l1Provider.getFeeData();

  logger.debug(`Fetched fee data. maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas}`);

  const tx = await lineaRollup.sendMessage(destinationAddress, fee, calldata, {
    value: fee,
    maxPriorityFeePerGas,
    maxFeePerGas,
  });

  logger.debug(`sendMessage transaction sent. transactionHash=${tx.hash}`);

  let receipt = await tx.wait();
  while (!receipt) {
    logger.debug(`Waiting for transaction to be mined... transactionHash=${tx.hash}`);
    receipt = await tx.wait();
  }

  logger.debug(`Transaction mined. transactionHash=${tx.hash} status=${receipt.status}`);

  return { tx, receipt };
}

async function sendL2ToL1Message(
  logger: Logger,
  {
    l1Account,
    l2Account,
    fee = 0n,
    withCalldata = false,
  }: {
    l1Account: Wallet;
    l2Account: Wallet;
    fee: bigint;
    withCalldata: boolean;
  },
) {
  const dummyContract = config.getL1DummyContract(l1Account);
  const l2MessageService = config.getL2MessageServiceContract(l2Account);
  const lineaEstimateGasClient = new LineaEstimateGasClient(config.getL2BesuNodeEndpoint()!);

  const calldata = withCalldata
    ? encodeFunctionCall(dummyContract.interface, "setPayload", [ethers.randomBytes(100)])
    : "0x";

  const destinationAddress = withCalldata ? await dummyContract.getAddress() : l1Account.address;

  const { maxPriorityFeePerGas, maxFeePerGas, gasLimit } = await lineaEstimateGasClient.lineaEstimateGas(
    l2Account.address,
    await l2MessageService.getAddress(),
    l2MessageService.interface.encodeFunctionData("sendMessage", [destinationAddress, fee, calldata]),
    toBeHex(etherToWei("0.001")),
  );
  logger.debug(`Fetched fee data. maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas}`);

  const tx = await l2MessageService.sendMessage(destinationAddress, fee, calldata, {
    value: fee,
    maxPriorityFeePerGas,
    maxFeePerGas,
    gasLimit,
  });

  logger.debug(`sendMessage transaction sent. transactionHash=${tx.hash}`);

  let receipt = await tx.wait();

  while (!receipt) {
    logger.debug(`Waiting for transaction to be mined... transactionHash=${tx.hash}`);
    receipt = await tx.wait();
  }

  logger.debug(`Transaction mined. transactionHash=${tx.hash} status=${receipt.status}`);

  return { tx, receipt };
}

const l1AccountManager = config.getL1AccountManager();
const l2AccountManager = config.getL2AccountManager();

describe("Messaging test suite", () => {
  it.concurrent(
    "Should send a transaction with fee and calldata to L1 message service, be successfully claimed it on L2",
    async () => {
      const [l1Account, l2Account] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const { tx, receipt } = await sendL1ToL2Message(logger, {
        l1Account,
        l2Account,
        fee: etherToWei("1.1"),
        withCalldata: true,
      });

      const [messageSentEvent] = receipt.logs.filter((log) => log.topics[0] === MESSAGE_SENT_EVENT_SIGNATURE);
      const messageHash = messageSentEvent.topics[3];
      logger.debug(`L1 message sent. messageHash=${messageHash} transaction=${JSON.stringify(tx)}`);

      logger.debug(`Waiting for MessageClaimed event on L2. messageHash=${messageHash}`);
      const l2MessageService = config.getL2MessageServiceContract();
      const [messageClaimedEvent] = await waitForEvents(
        l2MessageService,
        l2MessageService.filters.MessageClaimed(messageHash),
      );

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
      const [l1Account, l2Account] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const { tx, receipt } = await sendL1ToL2Message(logger, {
        l1Account,
        l2Account,
        fee: etherToWei("1.1"),
        withCalldata: false,
      });

      const [messageSentEvent] = receipt.logs.filter((log) => log.topics[0] === MESSAGE_SENT_EVENT_SIGNATURE);
      const messageHash = messageSentEvent.topics[3];
      logger.debug(`L1 message sent. messageHash=${messageHash} transactionHash=${tx.hash}`);

      logger.debug(`Waiting for MessageClaimed event on L2. messageHash=${messageHash}`);
      const l2MessageService = config.getL2MessageServiceContract();
      const [messageClaimedEvent] = await waitForEvents(
        l2MessageService,
        l2MessageService.filters.MessageClaimed(messageHash),
      );
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
      const [l1Account, l2Account] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const { tx, receipt } = await sendL1ToL2Message(logger, {
        l1Account,
        l2Account,
        fee: 0n,
        withCalldata: false,
      });

      const [messageSentEvent] = receipt.logs.filter((log) => log.topics[0] === MESSAGE_SENT_EVENT_SIGNATURE);
      const messageHash = messageSentEvent.topics[3];
      logger.debug(`L1 message sent. messageHash=${messageHash} transactionHash=${tx.hash}`);

      logger.debug(`Waiting for MessageClaimed event on L2. messageHash=${messageHash}`);
      const l2MessageService = config.getL2MessageServiceContract();
      const [messageClaimedEvent] = await waitForEvents(
        l2MessageService,
        l2MessageService.filters.MessageClaimed(messageHash),
      );
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
      const [l1Account, l2Account] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const lineaRollup = config.getLineaRollupContract();
      const { tx, receipt } = await sendL2ToL1Message(logger, {
        l1Account,
        l2Account,
        fee: etherToWei("0.001"),
        withCalldata: true,
      });

      const [messageSentEvent] = receipt.logs.filter((log) => log.topics[0] === MESSAGE_SENT_EVENT_SIGNATURE);
      const messageHash = messageSentEvent.topics[3];
      logger.debug(`L2 message sent. messageHash=${messageHash} transaction=${JSON.stringify(tx)}`);

      logger.debug(`Waiting for L2MessagingBlockAnchored event... blockNumber=${messageSentEvent.blockNumber}`);
      await waitForEvents(
        lineaRollup,
        lineaRollup.filters.L2MessagingBlockAnchored(messageSentEvent.blockNumber),
        1_000,
      );

      logger.debug(`Waiting for MessageClaimed event on L1... messageHash=${messageHash}`);
      const [messageClaimedEvent] = await waitForEvents(
        lineaRollup,
        lineaRollup.filters.MessageClaimed(messageHash),
        1_000,
      );

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
      const [l1Account, l2Account] = await Promise.all([
        l1AccountManager.generateAccount(),
        l2AccountManager.generateAccount(),
      ]);

      const lineaRollup = config.getLineaRollupContract();
      const { tx, receipt } = await sendL2ToL1Message(logger, {
        l1Account,
        l2Account,
        fee: etherToWei("0.001"),
        withCalldata: false,
      });

      const [messageSentEvent] = receipt.logs.filter((log) => log.topics[0] === MESSAGE_SENT_EVENT_SIGNATURE);
      const messageHash = messageSentEvent.topics[3];
      logger.debug(`L2 message sent. messageHash=${messageHash} transaction=${JSON.stringify(tx)}`);

      logger.debug(`Waiting for L2MessagingBlockAnchored event... blockNumber=${messageSentEvent.blockNumber}`);
      await waitForEvents(
        lineaRollup,
        lineaRollup.filters.L2MessagingBlockAnchored(messageSentEvent.blockNumber),
        1_000,
      );

      logger.debug(`Waiting for MessageClaimed event on L1. messageHash=${messageHash}`);
      const [messageClaimedEvent] = await waitForEvents(
        lineaRollup,
        lineaRollup.filters.MessageClaimed(messageHash),
        1_000,
      );

      expect(messageClaimedEvent).toBeDefined();

      logger.debug(
        `Message claimed on L1. messageHash=${messageClaimedEvent.args._messageHash} transactionHash=${messageClaimedEvent.transactionHash}`,
      );
    },
    150_000,
  );
});
