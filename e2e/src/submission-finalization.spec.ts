import { describe, expect, it } from "@jest/globals";
import { NonceManager } from "ethers";
import {
  getMessageSentEventFromLogs,
  sendMessage,
  waitForEvents,
  wait,
  getBlockByNumberOrBlockTag,
  etherToWei,
} from "./common/utils";
import { config } from "./config/tests-config";

const l1AccountManager = config.getL1AccountManager();

describe("Submission and finalization test suite", () => {
  const sendMessages = async () => {
    const messageFee = etherToWei("0.0001");
    const messageValue = etherToWei("0.0051");
    const destinationAddress = "0x8D97689C9818892B700e27F316cc3E41e17fBeb9";

    const l1MessageSender = new NonceManager(await l1AccountManager.generateAccount());
    const lineaRollup = config.getLineaRollupContract();

    logger.debug("Sending messages on L1...");

    // Send L1 messages
    const l1MessagesPromises = [];

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
          },
        ),
      );
    }

    const l1Receipts = await Promise.all(l1MessagesPromises);

    logger.debug("Messages sent on L1.");

    // Extract message events
    const l1Messages = getMessageSentEventFromLogs(lineaRollup, l1Receipts);

    return { l1Messages, l1Receipts };
  };

  describe("Contracts v6", () => {
    it.concurrent(
      "Check L2 anchoring",
      async () => {
        const lineaRollupV6 = config.getLineaRollupContract();
        const l2MessageService = config.getL2MessageServiceContract();

        const { l1Messages } = await sendMessages();

        // Wait for the last L1->L2 message to be anchored on L2
        const lastNewL1MessageNumber = l1Messages.slice(-1)[0].messageNumber;

        logger.debug(`Waiting for the anchoring using rolling hash... messageNumber=${lastNewL1MessageNumber}`);
        const [rollingHashUpdatedEvent] = await waitForEvents(
          l2MessageService,
          l2MessageService.filters.RollingHashUpdated(),
          1_000,
          0,
          "latest",
          async (events) => events.filter((event) => event.args.messageNumber >= lastNewL1MessageNumber),
        );

        const [lastNewMessageRollingHash, lastAnchoredL1MessageNumber] = await Promise.all([
          lineaRollupV6.rollingHashes(rollingHashUpdatedEvent.args.messageNumber),
          l2MessageService.lastAnchoredL1MessageNumber(),
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
        const lineaRollupV6 = config.getLineaRollupContract();

        const currentL2BlockNumber = await lineaRollupV6.currentL2BlockNumber();

        logger.debug("Waiting for DataSubmittedV3 used to finalize with proof...");
        await waitForEvents(lineaRollupV6, lineaRollupV6.filters.DataSubmittedV3(), 1_000);

        logger.debug("Waiting for DataFinalizedV3 event with proof...");
        const [dataFinalizedEvent] = await waitForEvents(
          lineaRollupV6,
          lineaRollupV6.filters.DataFinalizedV3(currentL2BlockNumber + 1n),
          1_000,
        );

        const [lastBlockFinalized, newStateRootHash] = await Promise.all([
          lineaRollupV6.currentL2BlockNumber(),
          lineaRollupV6.stateRootHashes(dataFinalizedEvent.args.endBlockNumber),
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
        const sequencerEndpoint = config.getSequencerEndpoint();
        if (!sequencerEndpoint) {
          logger.warn('Skipped the "Check L2 safe/finalized tag update on sequencer" test');
          return;
        }

        const lastFinalizedL2BlockNumberOnL1 = 0;
        logger.debug(`lastFinalizedL2BlockNumberOnL1=${lastFinalizedL2BlockNumberOnL1}`);

        let safeL2BlockNumber = -1,
          finalizedL2BlockNumber = -1;
        while (
          safeL2BlockNumber < lastFinalizedL2BlockNumberOnL1 ||
          finalizedL2BlockNumber < lastFinalizedL2BlockNumberOnL1
        ) {
          safeL2BlockNumber =
            (await getBlockByNumberOrBlockTag(sequencerEndpoint, "safe"))?.number || safeL2BlockNumber;
          finalizedL2BlockNumber =
            (await getBlockByNumberOrBlockTag(sequencerEndpoint, "finalized"))?.number || finalizedL2BlockNumber;
          await wait(1_000);
        }

        logger.debug(`safeL2BlockNumber=${safeL2BlockNumber} finalizedL2BlockNumber=${finalizedL2BlockNumber}`);

        expect(safeL2BlockNumber).toBeGreaterThanOrEqual(lastFinalizedL2BlockNumberOnL1);
        expect(finalizedL2BlockNumber).toBeGreaterThanOrEqual(lastFinalizedL2BlockNumberOnL1);

        logger.debug("L2 safe/finalized tag update on sequencer done.");
      },
      150_000,
    );
  });
});
