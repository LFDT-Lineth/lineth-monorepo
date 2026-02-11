import { etherToWei, serialize } from "@consensys/linea-shared-utils";
import { describe, expect, it } from "@jest/globals";

import { MINIMUM_FEE_IN_WEI } from "./common/constants";
import { execDockerCommand, getEvents, waitForEvents, getMessageSentEventFromLogs, sendMessage } from "./common/utils";
import { createTestContext } from "./config/setup";
import { L2MessageServiceV1Abi, LineaRollupV6Abi } from "./generated";

import type { Logger } from "winston";

/**
 * Barrier that restarts the coordinator exactly once, after all participants arrive.
 * Early arrivers wait. The last to arrive triggers the restart and unblocks everyone.
 */
function createRestartBarrier(participantCount: number) {
  let arrivals = 0;
  const waiters: Array<{ resolve: () => void; reject: (error: unknown) => void }> = [];

  return {
    arrive(logger: Logger): Promise<void> {
      arrivals++;
      logger.debug(`Barrier: ${arrivals}/${participantCount} arrived.`);

      if (arrivals < participantCount) {
        return new Promise<void>((resolve, reject) => {
          waiters.push({ resolve, reject });
        });
      }

      logger.debug("All participants arrived. Restarting coordinator...");
      return execDockerCommand("restart", "coordinator").then(
        () => {
          logger.debug("Coordinator restarted.");
          waiters.forEach(({ resolve }) => resolve());
        },
        (error) => {
          logger.error(`Failed to restart coordinator: ${error}`);
          waiters.forEach(({ reject }) => reject(error));
          throw error;
        },
      );
    },
  };
}

const restartBarrier = createRestartBarrier(2);
const context = createTestContext();
const l1AccountManager = context.getL1AccountManager();

describe("Coordinator restart test suite", () => {
  it.concurrent(
    "When the coordinator restarts it should resume blob submission and finalization",
    async () => {
      if (process.env.TEST_ENV !== "local") {
        logger.warn("Skipping test because it's not running on a local environment.");
        return;
      }

      const l1PublicClient = context.l1PublicClient();
      const lineaRollup = context.l1Contracts.lineaRollup(l1PublicClient);

      // Phase 1: Confirm coordinator was working before restart
      const [dataSubmittedEventsSnapshot, dataFinalizedEventsSnapshot] = await Promise.all([
        waitForEvents(l1PublicClient, {
          abi: LineaRollupV6Abi,
          address: lineaRollup.address,
          eventName: "DataSubmittedV3",
          fromBlock: 0n,
          toBlock: "latest",
          strict: true,
        }),
        waitForEvents(l1PublicClient, {
          abi: LineaRollupV6Abi,
          address: lineaRollup.address,
          eventName: "DataFinalizedV3",
          fromBlock: 0n,
          toBlock: "latest",
          strict: true,
        }),
      ]);

      expect(dataSubmittedEventsSnapshot.length).toBeGreaterThan(0);
      expect(dataFinalizedEventsSnapshot.length).toBeGreaterThan(0);

      const [lastSubmittedSnapshot] = dataSubmittedEventsSnapshot.slice(-1);
      const [lastFinalizedSnapshot] = dataFinalizedEventsSnapshot.slice(-1);

      logger.debug(`DataSubmittedV3 snapshot before restart. event=${serialize(lastSubmittedSnapshot)}`);
      logger.debug(`DataFinalizedV3 snapshot before restart. event=${serialize(lastFinalizedSnapshot)}`);

      expect(lastSubmittedSnapshot.blockNumber).toBeGreaterThan(0n);
      expect(lastFinalizedSnapshot.args.endBlockNumber).toBeGreaterThan(0n);

      // Phase 2: Restart coordinator
      await restartBarrier.arrive(logger);

      // Phase 3: Capture the true baseline after restart.
      // The coordinator just restarted, so the L1 chain state reflects everything
      // that happened up to (and including) the restart window.
      // Re-fetch from the last known event block to baselineBlockNumber.
      // Guaranteed to find at least the snapshot event, so waitForEvents resolves immediately.
      const baselineBlockNumber = await l1PublicClient.getBlockNumber();

      const [submittedDelta, finalizedDelta] = await Promise.all([
        getEvents(l1PublicClient, {
          abi: LineaRollupV6Abi,
          address: lineaRollup.address,
          eventName: "DataSubmittedV3",
          fromBlock: lastSubmittedSnapshot.blockNumber + 1n,
          toBlock: baselineBlockNumber,
          strict: true,
        }),
        getEvents(l1PublicClient, {
          abi: LineaRollupV6Abi,
          address: lineaRollup.address,
          eventName: "DataFinalizedV3",
          fromBlock: lastFinalizedSnapshot.blockNumber + 1n,
          toBlock: baselineBlockNumber,
          strict: true,
        }),
      ]);

      const lastSubmittedBeforeResume = submittedDelta.length > 0 ? submittedDelta.slice(-1)[0] : lastSubmittedSnapshot;
      const lastFinalizedBeforeResume = finalizedDelta.length > 0 ? finalizedDelta.slice(-1)[0] : lastFinalizedSnapshot;

      logger.debug(
        `True baseline after restart. lastSubmitted=${serialize(lastSubmittedBeforeResume)} lastFinalized=${serialize(lastFinalizedBeforeResume)}`,
      );

      // Phase 4: Wait for new events produced after coordinator resumes
      logger.debug("Waiting for DataSubmittedV3 event after coordinator restart...");
      const [dataSubmittedV3EventAfterRestart] = await waitForEvents(l1PublicClient, {
        abi: LineaRollupV6Abi,
        address: lineaRollup.address,
        eventName: "DataSubmittedV3",
        fromBlock: baselineBlockNumber,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
        criteria: async (events) => events.filter((e) => e.blockNumber > lastSubmittedBeforeResume.blockNumber),
      });

      expect(dataSubmittedV3EventAfterRestart).toBeDefined();

      logger.debug(
        `DataSubmittedV3 event after coordinator restart found. event=${serialize(dataSubmittedV3EventAfterRestart)}`,
      );

      logger.debug("Waiting for DataFinalizedV3 event after coordinator restart...");
      const [dataFinalizedEventAfterRestart] = await waitForEvents(l1PublicClient, {
        abi: LineaRollupV6Abi,
        address: lineaRollup.address,
        eventName: "DataFinalizedV3",
        fromBlock: baselineBlockNumber,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
        criteria: async (events) =>
          events.filter((e) => e.args.endBlockNumber > lastFinalizedBeforeResume.args.endBlockNumber),
      });

      expect(dataFinalizedEventAfterRestart).toBeDefined();

      logger.debug(
        `DataFinalizedV3 event after coordinator restart found. event=${serialize(dataFinalizedEventAfterRestart)}`,
      );

      expect(dataFinalizedEventAfterRestart.args.endBlockNumber).toBeGreaterThan(
        lastFinalizedBeforeResume.args.endBlockNumber,
      );
    },
    200_000,
  );

  it.concurrent(
    "When the coordinator restarts it should resume anchoring",
    async () => {
      if (process.env.TEST_ENV !== "local") {
        logger.warn("Skipping test because it's not running on a local environment.");
        return;
      }

      const l1PublicClient = context.l1PublicClient();
      const l2PublicClient = context.l2PublicClient();
      const lineaRollup = context.l1Contracts.lineaRollup(l1PublicClient);
      const l2MessageService = context.l2Contracts.l2MessageService(l2PublicClient);

      const [l1MessageSender, { maxPriorityFeePerGas, maxFeePerGas }] = await Promise.all([
        l1AccountManager.generateAccount(),
        l1PublicClient.estimateFeesPerGas(),
      ]);

      const l1WalletClient = context.l1WalletClient({ account: l1MessageSender });
      let l1MessageSenderNonce = await l1PublicClient.getTransactionCount({ address: l1MessageSender.address });

      logger.debug(`Fetched fee data. maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas}`);

      const messageFee = MINIMUM_FEE_IN_WEI;
      const messageValue = etherToWei("0.0051");
      const destinationAddress = "0x8D97689C9818892B700e27F316cc3E41e17fBeb9";

      // Phase 1: Send messages L1 -> L2 and wait for anchoring before restart
      logger.debug("Sending messages L1 -> L2 before coordinator restart...");
      const l1MessagesPromises = [];
      for (let i = 0; i < 5; i++) {
        l1MessagesPromises.push(
          sendMessage(l1WalletClient, {
            account: l1MessageSender,
            chain: l1PublicClient.chain,
            args: {
              to: destinationAddress,
              fee: messageFee,
              calldata: "0x",
            },
            contractAddress: lineaRollup.address,
            value: messageValue,
            nonce: l1MessageSenderNonce,
            maxPriorityFeePerGas,
            maxFeePerGas,
          }),
        );
        l1MessageSenderNonce++;
      }

      const l1Receipts = await Promise.all(l1MessagesPromises);
      const l1Messages = getMessageSentEventFromLogs(l1Receipts);
      const lastNewL1MessageNumber = l1Messages.slice(-1)[0].messageNumber;

      logger.debug(`Waiting for L1->L2 anchoring before coordinator restart. messageNumber=${lastNewL1MessageNumber}`);

      const l2BlockBeforeAnchoring = await l2PublicClient.getBlockNumber();

      const [rollingHashUpdatedEvent] = await waitForEvents(l2PublicClient, {
        abi: L2MessageServiceV1Abi,
        address: l2MessageService.address,
        eventName: "RollingHashUpdated",
        fromBlock: l2BlockBeforeAnchoring,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
        criteria: async (events) => {
          return events.filter((event) => event.args.messageNumber >= lastNewL1MessageNumber);
        },
      });

      expect(rollingHashUpdatedEvent).toBeDefined();

      // Phase 2: Restart coordinator
      await restartBarrier.arrive(logger);

      // Phase 3: Send messages L1 -> L2 after restart and wait for anchoring
      const l1Fees = await l1PublicClient.estimateFeesPerGas();

      logger.debug("Sending messages L1 -> L2 after coordinator restart...");
      const l1MessagesPromisesAfterRestart = [];
      for (let i = 0; i < 5; i++) {
        l1MessagesPromisesAfterRestart.push(
          sendMessage(l1WalletClient, {
            account: l1MessageSender,
            chain: l1PublicClient.chain,
            args: {
              to: destinationAddress,
              fee: messageFee,
              calldata: "0x",
            },
            contractAddress: lineaRollup.address,
            value: messageValue,
            nonce: l1MessageSenderNonce,
            maxPriorityFeePerGas: l1Fees.maxPriorityFeePerGas,
            maxFeePerGas: l1Fees.maxFeePerGas,
          }),
        );
        l1MessageSenderNonce++;
      }

      const l1ReceiptsAfterRestart = await Promise.all(l1MessagesPromisesAfterRestart);
      const l1MessagesAfterRestart = getMessageSentEventFromLogs(l1ReceiptsAfterRestart);
      const lastNewL1MessageNumberAfterRestart = l1MessagesAfterRestart.slice(-1)[0].messageNumber;

      logger.debug(
        `Waiting for L1->L2 anchoring after coordinator restart. messageNumber=${lastNewL1MessageNumberAfterRestart}`,
      );

      const l2BlockAfterRestart = await l2PublicClient.getBlockNumber();

      const [rollingHashUpdatedEventAfterRestart] = await waitForEvents(l2PublicClient, {
        abi: L2MessageServiceV1Abi,
        address: l2MessageService.address,
        eventName: "RollingHashUpdated",
        fromBlock: l2BlockAfterRestart,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
        criteria: async (events) => {
          return events.filter((event) => event.args.messageNumber >= lastNewL1MessageNumberAfterRestart);
        },
      });

      expect(rollingHashUpdatedEventAfterRestart).toBeDefined();

      const [lastNewMessageRollingHashAfterRestart, lastAnchoredL1MessageNumberAfterRestart] = await Promise.all([
        lineaRollup.read.rollingHashes([rollingHashUpdatedEventAfterRestart.args.messageNumber]),
        l2MessageService.read.lastAnchoredL1MessageNumber(),
      ]);

      expect(lastNewMessageRollingHashAfterRestart).toEqual(rollingHashUpdatedEventAfterRestart.args.rollingHash);
      expect(lastAnchoredL1MessageNumberAfterRestart).toEqual(rollingHashUpdatedEventAfterRestart.args.messageNumber);
    },
    200_000,
  );
});
