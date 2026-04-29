import { describe, expect, it } from "@jest/globals";

import { waitForEvents, awaitUntil, getBlockByNumberOrBlockTag } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";

const context = createTestContext();

describe("Submission and finalization test suite", () => {
  describe("Contracts v8", () => {
    it.concurrent(
      "Check L1 data submission and finalization",
      async () => {
        const lineaRollup = context.l1Contracts.lineaRollup(context.l1PublicClient());
        const l1PublicClient = context.l1PublicClient();
        const currentL2BlockNumber = await lineaRollup.read.currentL2BlockNumber();

        logger.debug("Waiting for DataSubmittedV3 used to finalize with proof...");
        const [dataSubmittedEvent] = await waitForEvents(l1PublicClient, {
          abi: lineaRollup.abi,
          address: lineaRollup.address,
          eventName: "DataSubmittedV3",
          fromBlock: 0n,
          toBlock: "latest",
          pollingIntervalMs: 1_000,
          strict: true,
        });

        expect(dataSubmittedEvent).toBeDefined();

        logger.debug("Waiting for DataFinalizedV3 event with proof...");
        const [dataFinalizedEvent] = await waitForEvents(l1PublicClient, {
          abi: lineaRollup.abi,
          address: lineaRollup.address,
          eventName: "DataFinalizedV3",
          fromBlock: 0n,
          toBlock: "latest",
          args: {
            startBlockNumber: currentL2BlockNumber + 1n,
          },
          pollingIntervalMs: 1_000,
          strict: true,
        });

        expect(dataFinalizedEvent).toBeDefined();

        const [lastBlockFinalized, newStateRootHash] = await Promise.all([
          lineaRollup.read.currentL2BlockNumber(),
          lineaRollup.read.stateRootHashes([dataFinalizedEvent.args.endBlockNumber]),
        ]);

        expect(lastBlockFinalized).toBeGreaterThanOrEqual(dataFinalizedEvent.args.endBlockNumber);
        expect(newStateRootHash).toEqual(dataFinalizedEvent.args.finalStateRootHash);

        logger.debug(`Finalization with proof done. lastFinalizedBlockNumber=${lastBlockFinalized}`);
      },
      250_000,
    );

    it.concurrent(
      "Check L2 safe/finalized tag update on sequencer",
      async () => {
        const sequencerClient = context.l2PublicClient({ type: L2RpcEndpoint.Sequencer });
        if (!context.isLocal()) {
          logger.warn('Skipped the "Check L2 safe/finalized tag update on sequencer" test');
          return;
        }

        const lastFinalizedL2BlockNumberOnL1 = 0;
        logger.debug(`lastFinalizedL2BlockNumberOnL1=${lastFinalizedL2BlockNumberOnL1}`);

        const { safeL2BlockNumber, finalizedL2BlockNumber } = await awaitUntil(
          async () => {
            const currentSafeL2BlockNumber = (await getBlockByNumberOrBlockTag(sequencerClient, { blockTag: "safe" }))
              ?.number;
            const currentFinalizedL2BlockNumber = (
              await getBlockByNumberOrBlockTag(sequencerClient, { blockTag: "finalized" })
            )?.number;

            const safe = currentSafeL2BlockNumber ? parseInt(currentSafeL2BlockNumber.toString()) : -1;
            const finalized = currentFinalizedL2BlockNumber ? parseInt(currentFinalizedL2BlockNumber.toString()) : -1;

            return { safeL2BlockNumber: safe, finalizedL2BlockNumber: finalized };
          },
          ({ safeL2BlockNumber, finalizedL2BlockNumber }) =>
            safeL2BlockNumber >= lastFinalizedL2BlockNumberOnL1 &&
            finalizedL2BlockNumber >= lastFinalizedL2BlockNumberOnL1,
          { pollingIntervalMs: 1_000, timeoutMs: 140_000 },
        );

        logger.debug(`safeL2BlockNumber=${safeL2BlockNumber} finalizedL2BlockNumber=${finalizedL2BlockNumber}`);

        expect(safeL2BlockNumber).toBeGreaterThanOrEqual(lastFinalizedL2BlockNumberOnL1);
        expect(finalizedL2BlockNumber).toBeGreaterThanOrEqual(lastFinalizedL2BlockNumberOnL1);

        logger.debug("L2 safe/finalized tag update on sequencer done.");
      },
      150_000,
    );

    it.concurrent(
      "Check latest L2 block number has been finalized on L1",
      async () => {
        const sequencerClient = context.l2PublicClient({ type: L2RpcEndpoint.Sequencer });
        if (!context.isLocal() || process.env.PARTIAL_PROVER != "true") {
          logger.warn('Skipped the "Check latest L2 block number has been finalized on L1" test');
          return;
        }

        const latestL2BlockNumber = (await getBlockByNumberOrBlockTag(sequencerClient, { blockTag: "latest" }))?.number;
        const targetL2BlockNumnber = latestL2BlockNumber ? parseInt(latestL2BlockNumber.toString()) : -1;
        logger.debug(`targetL2BlockNumnber=${targetL2BlockNumnber}`);

        const { finalizedL2BlockNumber } = await awaitUntil(
          async () => {
            const currentFinalizedL2BlockNumber = (
              await getBlockByNumberOrBlockTag(sequencerClient, { blockTag: "finalized" })
            )?.number;

            const finalized = currentFinalizedL2BlockNumber ? parseInt(currentFinalizedL2BlockNumber.toString()) : -1;

            return { finalizedL2BlockNumber: finalized };
          },
          ({ finalizedL2BlockNumber }) => finalizedL2BlockNumber >= targetL2BlockNumnber,
          { pollingIntervalMs: 1_000, timeoutMs: 1800_000 },
        );

        logger.debug(`finalizedL2BlockNumber=${finalizedL2BlockNumber}`);

        expect(finalizedL2BlockNumber).toBeGreaterThanOrEqual(targetL2BlockNumnber);

        logger.debug(`The target L2 block number ${targetL2BlockNumnber} has been finalized on L1`);
      },
      1800_000,
    );
  });
});
