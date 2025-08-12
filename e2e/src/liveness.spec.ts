import { describe, expect, it } from "@jest/globals";
import { config } from "./config/tests-config";
import { awaitUntil, execDockerCommand, getBlockByNumberOrBlockTag, GetEthLogsClient, wait } from "./common/utils";

// should remove skip only when the linea-sequencer plugin supports liveness
describe("Liveness test suite", () => {
  const l2AccountManager = config.getL2AccountManager();

  it.concurrent(
    "Should succeed to send liveness transactions after sequencer restarted",
    async () => {
      const account = await l2AccountManager.generateAccount();
      const livenessContract = config.getL2LineaSequencerUptimeFeedContract(account);
      const livenessContractAddress = await livenessContract.getAddress();

      const latestAnswer = await livenessContract.latestAnswer();
      logger.debug(`Latest Status is ${latestAnswer == 1n ? "Down" : "Up"}`);

      let lastBlockTimestamp: number | undefined = 0;
      let lastBlockNumber: number | undefined = 0;
      const l2BesuNodeProvider = config.getL2Provider();
      const ethGetLogsClient = new GetEthLogsClient(config.getL2BesuNodeEndpoint()!);

      try {
        await execDockerCommand("stop", "sequencer");
        logger.debug("Sequencer stopped.");

        // sleep for 9 sec (1 sec longer than the liveness-max-block-age)
        await wait(9000);

        const block = await l2BesuNodeProvider.getBlock("latest");
        lastBlockTimestamp = block?.timestamp;
        lastBlockNumber = block?.number;
        logger.debug(`lastBlockNumber=${lastBlockNumber} lastBlockTimestamp=${lastBlockTimestamp}`);

        await execDockerCommand("restart", "sequencer");
        logger.debug("Sequencer restarted.");
      } catch (error) {
        logger.error(`Failed to stop and restart sequencer: ${error}`);
        throw error;
      }

      const targetBlockNumber = lastBlockNumber! + 1;
      logger.debug(`targetBlockNumber=${JSON.stringify(targetBlockNumber)}`);

      let i = 0;
      const livenessEvents = await awaitUntil(
        async () => {
          try {
            logger.debug(`Trial ${i++} to get liveness events`);
            // using fetch JSON-RPC call to get logs instead of JsonRpcProvider to aviod flaky issue
            // where logs would fail to be retrieve from time to time
            return (
              await ethGetLogsClient.getLogs(
                livenessContractAddress,
                [
                  "0x0559884fd3a460db3073b7fc896cc77986f16e378210ded43186175bf646fc5f", // AnswerUpdated event
                ],
                targetBlockNumber,
                targetBlockNumber,
              )
            ).result;
          } catch (e) {
            return null;
          }
        },
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (ethLogs: any[] | null) => ethLogs != null && ethLogs.length >= 2,
        1000,
        150000,
      );

      logger.debug(`livenessEvents=${JSON.stringify(livenessEvents)}`);
      expect(livenessEvents?.length).toBeGreaterThanOrEqual(2);

      // The first two transactions of the target block should be the transactions
      // with "to" as the liveness contract address
      const targetBlock = await getBlockByNumberOrBlockTag(config.getL2BesuNodeEndpoint()!, targetBlockNumber, true);
      logger.debug(`targetBlock=${JSON.stringify(targetBlock)}`);
      expect(targetBlock?.transactions.length).toBeGreaterThanOrEqual(2);

      const downtimeTransaction = targetBlock?.transactions.at(0);
      const uptimeTransaction = targetBlock?.transactions.at(1);
      const downtimeEvent = livenessEvents?.find((tx) => tx.transactionHash === downtimeTransaction);
      const uptimeEvent = livenessEvents?.find((tx) => tx.transactionHash === uptimeTransaction);

      // check the first AnswerUpdated event is for downtime
      expect(downtimeEvent?.transactionIndex).toEqual("0x0");
      expect(downtimeEvent?.logIndex).toEqual("0x0");
      expect(parseInt(downtimeEvent?.topics[1] ?? "", 16)).toEqual(1); // topics[1] was the given status to update, should be 1 for downtime
      expect(parseInt(downtimeEvent?.data ?? "", 16)).toEqual(lastBlockTimestamp); // data should contain the timestamp of the last block before restart as downtime

      // check the second AnswerUpdated event is for uptime
      expect(uptimeEvent?.transactionIndex).toEqual("0x1");
      expect(uptimeEvent?.logIndex).toEqual("0x1");
      expect(parseInt(uptimeEvent?.topics[1] ?? "", 16)).toEqual(0); // topics[1] was the given status to update, should be 0 for uptime
      expect(parseInt(uptimeEvent?.data ?? "", 16)).toBeGreaterThan(lastBlockTimestamp ?? 0); // data should contain a timestamp later than the last block before restart as uptime
    },
    150000,
  );
});
