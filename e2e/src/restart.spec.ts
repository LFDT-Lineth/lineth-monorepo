import { etherToWei, serialize } from "@consensys/linea-shared-utils";
import { describe, expect, it } from "@jest/globals";

import { MINIMUM_FEE_IN_WEI } from "./common/constants";
import { sendL1ToL2Message } from "./common/test-helpers/messaging";
import { awaitUntil, execDockerCommand, getEvents, waitForEvents, getMessageSentEventFromLogs } from "./common/utils";
import { createTestContext } from "./config/setup";
import { L2MessageServiceV1Abi, LineaRollupV6Abi } from "./generated";

import type { Logger } from "winston";

const COORDINATOR_HEALTH_URL = "http://localhost:9545/health";
const COORDINATOR_READINESS_TIMEOUT_MS = 60_000;
const COORDINATOR_READINESS_POLLING_MS = 500;

async function waitForCoordinatorReadiness(logger: Logger): Promise<void> {
  logger.debug("Waiting for coordinator readiness...");
  await awaitUntil(
    async () => {
      const response = await fetch(COORDINATOR_HEALTH_URL);
      return response.ok;
    },
    (isHealthy) => isHealthy,
    { pollingIntervalMs: COORDINATOR_READINESS_POLLING_MS, timeoutMs: COORDINATOR_READINESS_TIMEOUT_MS },
  );
  logger.debug("Coordinator is ready.");
}

/**
 * Barrier that restarts the coordinator exactly once, after all participants arrive.
 * Early arrivers wait. The last to arrive triggers the restart, waits for readiness,
 * then unblocks everyone.
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
      return execDockerCommand("restart", "coordinator")
        .then(() => {
          logger.debug("Coordinator container restarted. Waiting for health endpoint...");
          return waitForCoordinatorReadiness(logger);
        })
        .then(
          () => {
            waiters.forEach(({ resolve }) => resolve());
          },
          (error) => {
            logger.debug(`Failed to restart coordinator: ${error}`);
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
          pollingIntervalMs: 1_000,
          strict: true,
        }),
        waitForEvents(l1PublicClient, {
          abi: LineaRollupV6Abi,
          address: lineaRollup.address,
          eventName: "DataFinalizedV3",
          fromBlock: 0n,
          toBlock: "latest",
          pollingIntervalMs: 1_000,
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

      const l1MessageSender = await l1AccountManager.generateAccount();

      const messageFee = MINIMUM_FEE_IN_WEI;
      const messageValue = etherToWei("0.0051");

      // Phase 1: Send an L1->L2 message and confirm it is anchored before restart
      logger.debug("Sending L1 -> L2 message before coordinator restart...");

      const { receipt: l1ReceiptBeforeRestart } = await sendL1ToL2Message(context, {
        account: l1MessageSender,
        fee: messageFee,
        value: messageValue,
      });

      const [l1MessageBeforeRestart] = getMessageSentEventFromLogs([l1ReceiptBeforeRestart]);

      logger.debug(
        `Waiting for L1->L2 anchoring before coordinator restart. messageNumber=${l1MessageBeforeRestart.messageNumber}`,
      );

      const l2BlockBeforeAnchoring = await l2PublicClient.getBlockNumber();

      await waitForEvents(l2PublicClient, {
        abi: L2MessageServiceV1Abi,
        address: l2MessageService.address,
        eventName: "RollingHashUpdated",
        fromBlock: l2BlockBeforeAnchoring,
        toBlock: "latest",
        pollingIntervalMs: 1_000,
        strict: true,
        criteria: async (events) => {
          return events.filter((event) => event.args.messageNumber >= l1MessageBeforeRestart.messageNumber);
        },
      });

      logger.info("Successfully anchored L1 -> L2 message before coordinator restart.");

      // Phase 2: Restart coordinator
      await restartBarrier.arrive(logger);

      // Phase 3: Send a new L1->L2 message after restart and wait for anchoring
      const { receipt: l1ReceiptAfterRestart } = await sendL1ToL2Message(context, {
        account: l1MessageSender,
        fee: messageFee,
        value: messageValue,
      });

      const [l1MessageAfterRestart] = getMessageSentEventFromLogs([l1ReceiptAfterRestart]);

      logger.debug(
        `Waiting for L1->L2 anchoring after coordinator restart. messageNumber=${l1MessageAfterRestart.messageNumber}`,
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
          return events.filter((event) => event.args.messageNumber >= l1MessageAfterRestart.messageNumber);
        },
      });

      // Phase 4: Verify anchored data matches on-chain state
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
