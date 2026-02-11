import { etherToWei } from "@consensys/linea-shared-utils";
import { describe, expect, it } from "@jest/globals";

import { MINIMUM_FEE_IN_WEI } from "./common/constants";
import { sendL1ToL2Message } from "./common/test-helpers/messaging";
import { getMessageSentEventFromLogs, waitForEvents, awaitUntil, getBlockByNumberOrBlockTag } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";
import { L2MessageServiceV1Abi, LineaRollupV6Abi } from "./generated";

const context = createTestContext();
const l1AccountManager = context.getL1AccountManager();

describe("Submission and finalization test suite", () => {
  const sendMessages = async () => {
    const messageFee = MINIMUM_FEE_IN_WEI;
    const messageValue = etherToWei("0.0051");

    const l1MessageSender = await l1AccountManager.generateAccount();

    logger.debug("Sending messages on L1...");

    // Send L1 messages
    const l1Receipts = [];

    for (let i = 0; i < 5; i++) {
      const { receipt } = await sendL1ToL2Message(context, {
        account: l1MessageSender,
        value: messageValue,
        fee: messageFee,
        withCalldata: false,
      });

      l1Receipts.push(receipt);
    }

    logger.debug(`Messages sent on L1. txHashes=${l1Receipts.map((receipt) => receipt.transactionHash)}`);

    // Extract message events
    const l1Messages = getMessageSentEventFromLogs(l1Receipts);

    return { l1Messages, l1Receipts };
  };

  describe("Contracts v6", () => {
    it.concurrent(
      "Check L2 anchoring",
      async () => {
        const lineaRollupV6 = context.l1Contracts.lineaRollup(context.l1PublicClient());
        const l2PublicClient = context.l2PublicClient();
        const l2MessageService = context.l2Contracts.l2MessageService(l2PublicClient);

        const { l1Messages } = await sendMessages();

        // Wait for the last L1->L2 message to be anchored on L2
        const lastNewL1MessageNumber = l1Messages.slice(-1)[0].messageNumber;

        logger.debug(`Waiting for the anchoring using rolling hash... messageNumber=${lastNewL1MessageNumber}`);
        const [rollingHashUpdatedEvent] = await waitForEvents(l2PublicClient, {
          abi: L2MessageServiceV1Abi,
          address: l2MessageService.address,
          eventName: "RollingHashUpdated",
          fromBlock: 0n,
          toBlock: "latest",
          pollingIntervalMs: 1_000,
          strict: true,
          criteria: async (events) => events.filter((event) => event.args.messageNumber >= lastNewL1MessageNumber),
        });

        expect(rollingHashUpdatedEvent).toBeDefined();

        const [lastNewMessageRollingHash, lastAnchoredL1MessageNumber] = await Promise.all([
          lineaRollupV6.read.rollingHashes([rollingHashUpdatedEvent.args.messageNumber]),
          l2MessageService.read.lastAnchoredL1MessageNumber(),
        ]);
        expect(lastNewMessageRollingHash).toEqual(rollingHashUpdatedEvent.args.rollingHash);
        expect(lastAnchoredL1MessageNumber).toEqual(rollingHashUpdatedEvent.args.messageNumber);

        logger.debug(`New anchoring using rolling hash done. rollingHash=${lastNewMessageRollingHash}`);
      },
      150_000,
    );

    it.concurrent(
      "Check L1 data submission and finalization",
      async () => {
        const lineaRollupV6 = context.l1Contracts.lineaRollup(context.l1PublicClient());
        const l1PublicClient = context.l1PublicClient();
        const currentL2BlockNumber = await lineaRollupV6.read.currentL2BlockNumber();

        logger.debug("Waiting for DataSubmittedV3 used to finalize with proof...");
        const [dataSubmittedEvent] = await waitForEvents(l1PublicClient, {
          abi: LineaRollupV6Abi,
          address: lineaRollupV6.address,
          eventName: "DataSubmittedV3",
          fromBlock: 0n,
          toBlock: "latest",
          pollingIntervalMs: 1_000,
          strict: true,
        });

        expect(dataSubmittedEvent).toBeDefined();

        logger.debug("Waiting for DataFinalizedV3 event with proof...");
        const [dataFinalizedEvent] = await waitForEvents(l1PublicClient, {
          abi: LineaRollupV6Abi,
          address: lineaRollupV6.address,
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
          lineaRollupV6.read.currentL2BlockNumber(),
          lineaRollupV6.read.stateRootHashes([dataFinalizedEvent.args.endBlockNumber]),
        ]);

        expect(lastBlockFinalized).toBeGreaterThanOrEqual(dataFinalizedEvent.args.endBlockNumber);
        expect(newStateRootHash).toEqual(dataFinalizedEvent.args.finalStateRootHash);

        logger.debug(`Finalization with proof done. lastFinalizedBlockNumber=${lastBlockFinalized}`);
      },
      150_000,
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
  });
});
