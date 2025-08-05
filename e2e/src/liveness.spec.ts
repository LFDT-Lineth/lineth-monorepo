import { describe, expect, it } from "@jest/globals";
import { config } from "./config/tests-config";
import { execDockerCommand, getBlockByNumberOrBlockTag, pollForBlockNumber, wait } from "./common/utils";

// should remove skip only when the linea-sequencer plugin supports liveness
describe.skip("Liveness test suite", () => {
  const l2AccountManager = config.getL2AccountManager();

  it.concurrent(
    "Should succeed to send liveness transactions after sequencer restarted",
    async () => {
      const account = await l2AccountManager.generateAccount();
      const livenessContract = config.getL2LineaSequencerUptimeFeedContract(account);
      const livenessContractAddress = await livenessContract.getAddress();

      const latestAnswer = await livenessContract.latestAnswer();
      logger.debug(`Latest Status is ${latestAnswer == 1n ? true : false}`);

      let lastBlockTimestamp: number | undefined = 0;
      let lastBlockNumber: number | undefined = 0;

      try {
        await execDockerCommand("stop", "sequencer");
        logger.debug("Sequencer stopped.");

        // sleep for 8 sec
        await wait(8000);

        const block = await config.getL2Provider().getBlock("latest");
        lastBlockTimestamp = block?.timestamp;
        lastBlockNumber = block?.number;
        logger.debug(`lastBlockNumber=${lastBlockNumber} lastBlockTimestamp=${lastBlockTimestamp}`);

        await execDockerCommand("restart", "sequencer");
        logger.debug("Sequencer restarted.");
      } catch (error) {
        logger.error(`Failed to stop and restart sequencer: ${error}`);
        throw error;
      }

      // wait until the target block is available
      await pollForBlockNumber(config.getL2Provider(), lastBlockNumber! + 1);

      const targetBlock = await getBlockByNumberOrBlockTag(config.getL2BesuNodeEndpoint()!, lastBlockNumber! + 1, true);
      const targetBlockTimestamp: number | undefined = targetBlock?.timestamp;
      logger.debug(`targetBlock=${JSON.stringify(targetBlock)}`);

      // The latest status should be true to indicate sequencer is up
      // and the startedAt and updatedAt should be greater than or equal to the target block timestamp
      const latestRoundData = await livenessContract.latestRoundData();
      expect(latestRoundData.answer).toEqual(0n); // which mean sequencer is currently reported as up
      expect(latestRoundData.startedAt).toBeGreaterThanOrEqual(BigInt(targetBlockTimestamp!));
      expect(latestRoundData.updatedAt).toBeGreaterThanOrEqual(BigInt(targetBlockTimestamp!));

      // The first two transactions of the target block should be the transactions
      // with "to" as the liveness contract address
      expect(targetBlock?.transactions.length).toBeGreaterThanOrEqual(2);

      const downtimeTransaction = targetBlock?.transactions.at(0);
      const uptimeTransaction = targetBlock?.transactions.at(1);

      const livenessEvents = await config.getL2Provider().getLogs({
        topics: [
          "0x0559884fd3a460db3073b7fc896cc77986f16e378210ded43186175bf646fc5f", // AnswerUpdated event
        ],
        fromBlock: targetBlock?.number,
        toBlock: targetBlock?.number,
        address: livenessContractAddress,
      });

      logger.debug(`livenessEvents=${JSON.stringify(livenessEvents)}`);

      const downtimeEvent = livenessEvents.find((tx) => tx.transactionHash === downtimeTransaction);
      const uptimeEvent = livenessEvents.find((tx) => tx.transactionHash === uptimeTransaction);

      // check the first AnswerUpdated event is for downtime
      expect(downtimeEvent?.transactionIndex).toEqual(0);
      expect(downtimeEvent?.index).toEqual(0);
      expect(parseInt(downtimeEvent?.topics[1] ?? "", 16)).toEqual(1); // topics[1] was the given status to update, should be 1 for downtime

      // check the second AnswerUpdated event is for uptime
      expect(uptimeEvent?.transactionIndex).toEqual(1);
      expect(uptimeEvent?.index).toEqual(1);
      expect(parseInt(uptimeEvent?.topics[1] ?? "", 16)).toEqual(0); // topics[1] was the given status to update, should be 0 for uptime
    },
    100000,
  );
});
