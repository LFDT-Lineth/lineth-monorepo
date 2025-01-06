import { describe, expect, it } from "@jest/globals";
import type { Logger } from "winston";
import {
  execDockerCommand,
  waitForEvents,
  getMessageSentEventFromLogs,
  sendMessage,
  etherToWei,
  wait,
} from "./common/utils";
import { config } from "./config/tests-config";

let testsWaitingForRestart = 0;
const TOTAL_TESTS_WAITING = 2;
let coordinatorHasRestarted = false;

async function waitForCoordinatorRestart(logger: Logger) {
  testsWaitingForRestart += 1;
  while (testsWaitingForRestart < TOTAL_TESTS_WAITING) {
    logger.info("Both tests have reached the restart point. Restarting coordinator...");
    await wait(1_000);
    if (!coordinatorHasRestarted) {
      coordinatorHasRestarted = true;
      try {
        await execDockerCommand("restart", "coordinator");
        logger.info("Coordinator restarted.");
        return;
      } catch (error) {
        logger.error(`Failed to restart coordinator: ${error}`);
        throw error;
      }
    }
  }
}

const l1AccountManager = config.getL1AccountManager();

describe("Coordinator restart test suite", () => {
  it.concurrent(
    "When the coordinator restarts it should resume blob submission and finalization",
    async () => {
      if (process.env.TEST_ENV !== "local") {
        logger.info("Skipping test because it's not running on a local environment.");
        return;
      }
      const lineaRollup = config.getLineaRollupContract();
      const l1Provider = config.getL1Provider();
      // await for a finalization to happen on L1
      const [dataSubmittedEventsBeforeRestart, dataFinalizedEventsBeforeRestart] = await Promise.all([
        waitForEvents(lineaRollup, lineaRollup.filters.DataSubmittedV3(), 0, "latest"),
        waitForEvents(lineaRollup, lineaRollup.filters.DataFinalizedV3(), 0, "latest"),
      ]);

      const lastDataSubmittedEventBeforeRestart = dataSubmittedEventsBeforeRestart.slice(-1)[0];
      const lastDataFinalizedEventsBeforeRestart = dataFinalizedEventsBeforeRestart.slice(-1)[0];

      // Just some sanity checks
      // Check that the coordinator has submitted and finalized data before the restart
      expect(lastDataSubmittedEventBeforeRestart.blockNumber).toBeGreaterThan(0n);
      expect(lastDataFinalizedEventsBeforeRestart.args.endBlockNumber).toBeGreaterThan(0n);

      await waitForCoordinatorRestart(logger);

      const currentBlockNumberAfterRestart = await l1Provider.getBlockNumber();

      logger.info("Waiting for DataSubmittedV3 event after coordinator restart...");
      const [dataSubmittedV3EventAfterRestart] = await waitForEvents(
        lineaRollup,
        lineaRollup.filters.DataSubmittedV3(),
        1_000,
        currentBlockNumberAfterRestart,
        "latest",
        async (events) => events.filter((event) => event.blockNumber > lastDataSubmittedEventBeforeRestart.blockNumber),
      );
      logger.info(`New DataSubmittedV3 event found. event=${JSON.stringify(dataSubmittedV3EventAfterRestart)}`);

      logger.info("Waiting for DataFinalized event after coordinator restart...");
      const [dataFinalizedEventAfterRestart] = await waitForEvents(
        lineaRollup,
        lineaRollup.filters.DataFinalizedV3(),
        1_000,
        currentBlockNumberAfterRestart,
        "latest",
        async (events) =>
          events.filter(
            (event) => event.args.endBlockNumber > lastDataFinalizedEventsBeforeRestart.args.endBlockNumber,
          ),
      );
      logger.info(`New DataFinalized event found. event=${JSON.stringify(dataFinalizedEventAfterRestart)}`);

      expect(dataFinalizedEventAfterRestart.args.endBlockNumber).toBeGreaterThan(
        lastDataFinalizedEventsBeforeRestart.args.endBlockNumber,
      );
    },
    150_000,
  );

  it.concurrent(
    "When the coordinator restarts it should resume anchoring",
    async () => {
      if (process.env.TEST_ENV !== "local") {
        logger.info("Skipping test because it's not running on a local environment.");
        return;
      }

      const l1Provider = config.getL1Provider();
      const l1MessageSender = await l1AccountManager.generateAccount();

      const lineaRollup = config.getLineaRollupContract();
      const l2MessageService = config.getL2MessageServiceContract();

      // Send Messages L1 -> L2
      const messageFee = etherToWei("0.0001");
      const messageValue = etherToWei("0.0051");
      const destinationAddress = "0x8D97689C9818892B700e27F316cc3E41e17fBeb9";

      const l1MessagesPromises = [];
      let l1MessageSenderNonce = await l1Provider.getTransactionCount(l1MessageSender.address);
      const { maxPriorityFeePerGas, maxFeePerGas } = await l1Provider.getFeeData();

      for (let i = 0; i < 5; i++) {
        l1MessagesPromises.push(
          sendMessage(
            l1MessageSender,
            lineaRollup,
            {
              to: destinationAddress,
              fee: messageFee,
              calldata: "0x",
            },
            {
              value: messageValue,
              nonce: l1MessageSenderNonce,
              maxPriorityFeePerGas,
              maxFeePerGas,
            },
          ),
        );
        l1MessageSenderNonce++;
      }

      const l1Receipts = await Promise.all(l1MessagesPromises);
      const l1Messages = getMessageSentEventFromLogs(lineaRollup, l1Receipts);

      // Wait for L2 Anchoring
      const lastNewL1MessageNumber = l1Messages.slice(-1)[0].messageNumber;

      logger.info(`Waiting for L1->L2 anchoring. messageNumber=${lastNewL1MessageNumber}`);
      await waitForEvents(
        l2MessageService,
        l2MessageService.filters.RollingHashUpdated(),
        1_000,
        0,
        "latest",
        async (events) => {
          return events.filter((event) => event.args.messageNumber >= lastNewL1MessageNumber);
        },
      );

      // Restart Coordinator
      await waitForCoordinatorRestart(logger);
      const l1Fees = await l1Provider.getFeeData();

      // Send more messages L1 -> L2
      for (let i = 0; i < 5; i++) {
        l1MessagesPromises.push(
          sendMessage(
            l1MessageSender,
            lineaRollup.connect(l1MessageSender),
            {
              to: destinationAddress,
              fee: messageFee,
              calldata: "0x",
            },
            {
              value: messageValue,
              nonce: l1MessageSenderNonce,
              maxPriorityFeePerGas: l1Fees.maxPriorityFeePerGas,
              maxFeePerGas: l1Fees.maxFeePerGas,
            },
          ),
        );
        l1MessageSenderNonce++;
      }

      const l1ReceiptsAfterRestart = await Promise.all(l1MessagesPromises);
      const l1MessagesAfterRestart = getMessageSentEventFromLogs(lineaRollup, l1ReceiptsAfterRestart);

      // Wait for messages to be anchored on L2
      const lastNewL1MessageNumberAfterRestart = l1MessagesAfterRestart.slice(-1)[0].messageNumber;

      logger.info(
        `Waiting for L1->L2 anchoring after coordinator restart. messageNumber=${lastNewL1MessageNumberAfterRestart}`,
      );
      const [rollingHashUpdatedEventAfterRestart] = await waitForEvents(
        l2MessageService,
        l2MessageService.filters.RollingHashUpdated(lastNewL1MessageNumberAfterRestart),
        1_000,
        0,
        "latest",
        async (events) => {
          return events.filter((event) => event.args.messageNumber >= lastNewL1MessageNumberAfterRestart);
        },
      );

      const [lastNewMessageRollingHashAfterRestart, lastAnchoredL1MessageNumberAfterRestart] = await Promise.all([
        lineaRollup.rollingHashes(rollingHashUpdatedEventAfterRestart.args.messageNumber),
        l2MessageService.lastAnchoredL1MessageNumber(),
      ]);

      expect(lastNewMessageRollingHashAfterRestart).toEqual(rollingHashUpdatedEventAfterRestart.args.rollingHash);
      expect(lastAnchoredL1MessageNumberAfterRestart).toEqual(rollingHashUpdatedEventAfterRestart.args.messageNumber);
    },
    150_000,
  );
});
