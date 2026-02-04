import { describe, expect, it } from "@jest/globals";
import { config } from "./config/tests-config";
import { awaitUntil, execDockerCommand, getBlockByNumberOrBlockTag, GetEthLogsClient, wait } from "./common/utils";
import { Wallet } from "ethers";
import { LIVENESS_ACCOUNT_INDEX } from "./common/constants";

// should remove skip only when the linea-sequencer plugin supports liveness
describe("Liveness test suite", () => {
  it.concurrent(
    "Should succeed to send liveness transactions after sequencer restarted",
    async () => {
      /**
       * Account index {@link LIVENESS_ACCOUNT_INDEX} is reserved for liveness testing to avoid nonce conflicts with other concurrent e2e tests.
       */
      const livenessSigner = config.getL2AccountManager().whaleAccount(LIVENESS_ACCOUNT_INDEX);

      const livenessContract = config.getL2LineaSequencerUptimeFeedContract(livenessSigner.signer as Wallet);
      const livenessContractAddress = await livenessContract.getAddress();

      const latestAnswer = await livenessContract.latestAnswer();
      logger.debug(`Latest Status is ${latestAnswer == 1n ? "Down" : "Up"}`);

      let lastBlockTimestamp: number | undefined = 0;
      let lastBlockNumber: number | undefined = 0;
      const l2BesuNodeProvider = config.getL2Provider();
      const ethGetLogsClientBesu = new GetEthLogsClient(config.getL2BesuNodeEndpoint()!);
      const ethGetLogsClientSequencer = new GetEthLogsClient(config.getSequencerEndpoint()!);

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
              await (i % 2 == 0 ? ethGetLogsClientBesu : ethGetLogsClientSequencer).getLogs(
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

        (ethLogs: any[] | null) => ethLogs != null && ethLogs.length >= 2,
        1000,
        150000,
      );

      logger.debug(`livenessEvents=${JSON.stringify(livenessEvents)}`);
      expect(livenessEvents?.length).toBeGreaterThanOrEqual(2);

      // The first two transactions of the target block should be the transactions
      // with "to" as the liveness contract address
      const targetBlock = await getBlockByNumberOrBlockTag(
        i % 2 == 0 ? config.getL2BesuNodeEndpoint()! : config.getSequencerEndpoint()!,
        targetBlockNumber,
        true,
      );
      logger.debug(`targetBlock=${JSON.stringify(targetBlock)}`);
      expect(targetBlock?.transactions.length).toBeGreaterThanOrEqual(2);

      const downtimeTransaction = targetBlock?.transactions.at(0);
      const uptimeTransaction = targetBlock?.transactions.at(1);
      const downtimeEvent = livenessEvents?.find((tx) => tx.transactionHash === downtimeTransaction);
      const uptimeEvent = livenessEvents?.find((tx) => tx.transactionHash === uptimeTransaction);

      // check the first AnswerUpdated event is for downtime
      expect(downtimeEvent?.transactionIndex).toEqual("0x0");
      expect(parseInt(downtimeEvent?.topics[1] ?? "", 16)).toEqual(1); // topics[1] was the given status to update, should be 1 for downtime
      expect(parseInt(downtimeEvent?.data ?? "", 16)).toEqual(lastBlockTimestamp); // data should contain the timestamp of the last block before restart as downtime

      // check the second AnswerUpdated event is for uptime
      expect(uptimeEvent?.transactionIndex).toEqual("0x1");
      expect(parseInt(uptimeEvent?.topics[1] ?? "", 16)).toEqual(0); // topics[1] was the given status to update, should be 0 for uptime
      expect(parseInt(uptimeEvent?.data ?? "", 16)).toBeGreaterThan(lastBlockTimestamp ?? 0); // data should contain a timestamp later than the last block before restart as uptime
    },
    150000,
  );
});
