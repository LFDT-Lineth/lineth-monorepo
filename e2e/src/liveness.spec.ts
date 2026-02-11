import { serialize } from "@consensys/linea-shared-utils";
import { describe, expect, it } from "@jest/globals";

import { awaitUntil, execDockerCommand } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";
import { LineaSequencerUptimeFeedAbi } from "./generated";

const context = createTestContext();

// Maximum allowed age (in seconds) of the newest block before the system considers the sequencer unhealthy.
const LIVENESS_MAX_BLOCK_AGE_SECONDS = 8;
// Additional seconds added as a buffer to the maximum block age during testing to avoid flakiness.
const BUFFER_SECONDS = 1;
// The total threshold (in seconds) for the block to be considered old enough to trigger sequencer liveness failure detection (max age + buffer).
const REQUIRED_BLOCK_AGE_SECONDS = LIVENESS_MAX_BLOCK_AGE_SECONDS + BUFFER_SECONDS;

describe("Liveness test suite", () => {
  it.concurrent(
    "Should succeed to send liveness transactions after sequencer restarted",
    async () => {
      const livenessContract = context.l2Contracts.lineaSequencerUptimeFeed(context.l2PublicClient());

      const latestAnswer = await livenessContract.read.latestAnswer();
      logger.debug(`Latest Status is ${latestAnswer == 1n ? "Down" : "Up"}`);

      let lastBlockTimestamp: bigint;
      let lastBlockNumber: bigint;
      const l2BesuNodeClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
      const sequencerClient = context.l2PublicClient({ type: L2RpcEndpoint.Sequencer });

      try {
        await execDockerCommand("stop", "sequencer");
        logger.debug("Sequencer stopped.");

        // Get the last block before sequencer stopped
        const lastBlockBeforeStop = await l2BesuNodeClient.getBlock({ blockTag: "latest" });
        const lastBlockTimestampBeforeStop = lastBlockBeforeStop.timestamp;
        logger.debug(
          `Last block before stop: number=${lastBlockBeforeStop.number} timestamp=${lastBlockTimestampBeforeStop}`,
        );

        // Wait until the block age exceeds the liveness threshold (8 seconds + 1 second buffer = 9 seconds)
        // This ensures the liveness system will detect the sequencer as down
        await awaitUntil(
          async () => {
            const currentBlock = await l2BesuNodeClient.getBlock({ blockTag: "latest" });
            const currentTime = BigInt(Math.floor(Date.now() / 1000));
            const blockAgeSeconds = Number(currentTime - lastBlockTimestampBeforeStop);
            logger.debug(
              `Waiting for block age threshold. currentBlockNumber=${currentBlock.number} blockAgeSeconds=${blockAgeSeconds} required=${REQUIRED_BLOCK_AGE_SECONDS}`,
            );
            return { blockAgeSeconds, currentBlock };
          },
          ({ blockAgeSeconds }) => blockAgeSeconds >= REQUIRED_BLOCK_AGE_SECONDS,
          {
            timeoutMs: 15_000,
          },
        );

        const block = await l2BesuNodeClient.getBlock({ blockTag: "latest" });
        lastBlockTimestamp = block.timestamp;
        lastBlockNumber = block.number;
        logger.debug(`lastBlockNumber=${lastBlockNumber} lastBlockTimestamp=${lastBlockTimestamp}`);

        await execDockerCommand("restart", "sequencer");
        logger.debug("Sequencer restarted.");
      } catch (error) {
        logger.error(`Failed to stop and restart sequencer: ${error}`);
        throw error;
      }

      const targetBlockNumber = lastBlockNumber + 1n;
      logger.debug(`targetBlockNumber=${targetBlockNumber.toString()}`);

      let i = 0;
      const livenessEvents = await awaitUntil(
        async () => {
          try {
            logger.debug(`Trial ${i++} to get liveness events`);
            // using fetch JSON-RPC call to get logs instead of JsonRpcProvider to aviod flaky issue
            // where logs would fail to be retrieve from time to time
            return await (i % 2 == 0 ? l2BesuNodeClient : sequencerClient).getLogs({
              address: livenessContract.address,
              fromBlock: targetBlockNumber,
              toBlock: targetBlockNumber,
              events: LineaSequencerUptimeFeedAbi.filter(
                (value) => value.type === "event" && value.name === "AnswerUpdated",
              ),
            });
          } catch (e) {
            return null;
          }
        },

        (ethLogs) => ethLogs != null && ethLogs.length >= 2,
        { pollingIntervalMs: 1_000, timeoutMs: 150_000 },
      );

      logger.debug(`livenessEvents=${serialize(livenessEvents)}`);
      expect(livenessEvents?.length).toBeGreaterThanOrEqual(2);

      // The first two transactions of the target block should be the transactions
      // with "to" as the liveness contract address
      const targetBlock = await (i % 2 == 0 ? l2BesuNodeClient : sequencerClient).getBlock({
        blockNumber: targetBlockNumber,
        includeTransactions: true,
      });
      logger.debug(`targetBlock=${serialize(targetBlock)}`);
      expect(targetBlock?.transactions.length).toBeGreaterThanOrEqual(2);

      const downtimeTransaction = targetBlock?.transactions.at(0);
      const uptimeTransaction = targetBlock?.transactions.at(1);
      const downtimeEvent = livenessEvents?.find((event) => event.transactionHash === downtimeTransaction?.hash);
      const uptimeEvent = livenessEvents?.find((event) => event.transactionHash === uptimeTransaction?.hash);

      // check the first AnswerUpdated event is for downtime
      expect(downtimeEvent?.transactionIndex).toEqual(0);
      expect(downtimeEvent?.args.current).toEqual(1n); // the given status to update, should be 1 for downtime
      expect(downtimeEvent?.args.updatedAt).toEqual(lastBlockTimestamp); // the timestamp of the last block before restart as downtime

      // check the second AnswerUpdated event is for uptime
      expect(uptimeEvent?.transactionIndex).toEqual(1);
      expect(uptimeEvent?.args.current).toEqual(0n); // the given status to update, should be 0 for uptime
      expect(uptimeEvent?.args.updatedAt).toBeGreaterThan(lastBlockTimestamp ?? 0); // data should contain a timestamp later than the last block before restart as uptime
    },
    150000,
  );
});
