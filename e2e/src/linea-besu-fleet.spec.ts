import { ethers, JsonRpcProvider } from "ethers";
import { describe, expect, it } from "@jest/globals";
import { config } from "./config/tests-config";
import { awaitUntil, LineaEstimateGasClient } from "./common/utils";

const l2AccountManager = config.getL2AccountManager();

describe("Linea besu fleet test suite", () => {
  const lineaRollupV6 = config.getLineaRollupContract();
  const lineaEstimateGasLeaderClient = new LineaEstimateGasClient(config.getL2BesuNodeEndpoint()!);
  const lineaEstimateGasFollowerClient = new LineaEstimateGasClient(config.getL2BesuFollowerNodeEndpoint()!);
  const leaderL2Provider = new JsonRpcProvider(config.getL2BesuNodeEndpoint()!.toString());
  const followerL2Provider = new JsonRpcProvider(config.getL2BesuFollowerNodeEndpoint()!.toString());

  it.concurrent("Responses from leader and follower should match", async () => {
    // Wait until the finalized L2 block number on L1 is greater than one
    await awaitUntil(
      async () => {
        try {
          return await lineaRollupV6.currentL2BlockNumber({ blockTag: "finalized" });
        } catch (err) {
          if (!(err as Error).message.includes("could not decode result data")) {
            throw err;
          } // else means the currentL2BlockNumber is not ready in the L1 rollup contract yet
          return -1n;
        }
      },
      (currentL2BlockNumber: bigint) => currentL2BlockNumber > 1n,
      1000,
      150000,
    );

    const account = await l2AccountManager.generateAccount();
    const dummyContract = config.getL2DummyContract(account);
    const randomBytes = ethers.randomBytes(1000);

    // linea_estimateGas responses from leader and follower should match
    const gasFeesFromLeader = await lineaEstimateGasLeaderClient.lineaEstimateGas(
      account.address,
      await dummyContract.getAddress(),
      dummyContract.interface.encodeFunctionData("setPayload", [randomBytes]),
    );
    logger.debug(
      `Fetched fee data from leader. maxPriorityFeePerGas=${gasFeesFromLeader.maxPriorityFeePerGas} maxFeePerGas=${gasFeesFromLeader.maxFeePerGas}`,
    );

    const gasFeesFromFollower = await lineaEstimateGasFollowerClient.lineaEstimateGas(
      account.address,
      await dummyContract.getAddress(),
      dummyContract.interface.encodeFunctionData("setPayload", [randomBytes]),
    );
    logger.debug(
      `Fetched fee data from follower. maxPriorityFeePerGas=${gasFeesFromFollower.maxPriorityFeePerGas} maxFeePerGas=${gasFeesFromFollower.maxFeePerGas}`,
    );

    expect(gasFeesFromLeader.maxPriorityFeePerGas).toEqual(gasFeesFromFollower.maxPriorityFeePerGas);
    expect(gasFeesFromLeader.maxFeePerGas).toEqual(gasFeesFromFollower.maxFeePerGas);

    // eth_estimateGas responses from leader and follower should match
    const estimatedGasFromLeader = await leaderL2Provider.estimateGas({
      from: account.address,
      to: await dummyContract.getAddress(),
      data: dummyContract.interface.encodeFunctionData("setPayload", [randomBytes]),
    });
    logger.debug(`Fetched fee data from leader. estimatedGasFromLeader=${estimatedGasFromLeader}`);

    const estimatedGasFromFollower = await followerL2Provider.estimateGas({
      from: account.address,
      to: await dummyContract.getAddress(),
      data: dummyContract.interface.encodeFunctionData("setPayload", [randomBytes]),
    });
    logger.debug(`Fetched fee data from follower. estimatedGasFromFollower=${estimatedGasFromFollower}`);

    expect(estimatedGasFromLeader).toEqual(estimatedGasFromFollower);

    const getBlockFromLeader = await leaderL2Provider.getBlock("finalized", false);
    const getBlockFromFollower = await followerL2Provider.getBlock("finalized", false);

    // Finalized block numbers and hashes from leader and follower should match
    logger.debug(
      `Fetched finalized block and hash from leader. finalizedBlockNumber=${getBlockFromLeader!.number} hash=${getBlockFromLeader!.hash}`,
    );
    logger.debug(
      `Fetched finalized block and hash from follower. finalizedBlockNumber=${getBlockFromFollower!.number} hash=${getBlockFromFollower!.hash}`,
    );

    expect(getBlockFromLeader!.number).toEqual(getBlockFromFollower!.number);
    expect(getBlockFromLeader!.hash).toEqual(getBlockFromFollower!.hash);
  });
});
