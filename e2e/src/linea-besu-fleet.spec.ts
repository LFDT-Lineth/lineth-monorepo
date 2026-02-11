import { describe, expect, it } from "@jest/globals";
import { randomBytes } from "crypto";
import { encodeFunctionData, toHex } from "viem";

import { awaitUntil, estimateLineaGas } from "./common/utils";
import { L2RpcEndpoint } from "./config/clients/l2-client";
import { createTestContext } from "./config/setup";
import { DummyContractAbi } from "./generated";

const context = createTestContext();
const l2AccountManager = context.getL2AccountManager();

describe("Linea besu fleet test suite", () => {
  const lineaRollupV6 = context.l1Contracts.lineaRollup(context.l1PublicClient());
  const gasLeaderClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuNode });
  const gasFollowerClient = context.l2PublicClient({ type: L2RpcEndpoint.BesuFollower });

  it.concurrent("Responses from leader and follower should match", async () => {
    // Wait until the finalized L2 block number on L1 is greater than one
    await awaitUntil(
      async () => {
        try {
          return await lineaRollupV6.read.currentL2BlockNumber({ blockTag: "finalized" });
        } catch (err) {
          if (!(err as Error).message.includes("could not decode result data")) {
            throw err;
          } // else means the currentL2BlockNumber is not ready in the L1 rollup contract yet
          return -1n;
        }
      },
      (currentL2BlockNumber: bigint) => currentL2BlockNumber > 1n,
      { pollingIntervalMs: 1_000, timeoutMs: 150_000 },
    );

    const account = await l2AccountManager.generateAccount();
    const dummyContract = context.l2Contracts.dummyContract(context.l2PublicClient());
    const randomPayload = toHex(randomBytes(1000).toString("hex"));

    // linea_estimateGas responses from leader and follower should match
    const gasFeesFromLeader = await estimateLineaGas(gasLeaderClient, {
      account,
      to: dummyContract.address,
      data: encodeFunctionData({
        abi: DummyContractAbi,
        functionName: "setPayload",
        args: [randomPayload],
      }),
    });
    logger.debug(
      `Fetched fee data from leader. maxPriorityFeePerGas=${gasFeesFromLeader.maxPriorityFeePerGas} maxFeePerGas=${gasFeesFromLeader.maxFeePerGas}`,
    );

    const gasFeesFromFollower = await estimateLineaGas(gasFollowerClient, {
      account,
      to: dummyContract.address,
      data: encodeFunctionData({
        abi: DummyContractAbi,
        functionName: "setPayload",
        args: [randomPayload],
      }),
    });
    logger.debug(
      `Fetched fee data from follower. maxPriorityFeePerGas=${gasFeesFromFollower.maxPriorityFeePerGas} maxFeePerGas=${gasFeesFromFollower.maxFeePerGas}`,
    );

    expect(gasFeesFromLeader.maxPriorityFeePerGas).toEqual(gasFeesFromFollower.maxPriorityFeePerGas);
    expect(gasFeesFromLeader.maxFeePerGas).toEqual(gasFeesFromFollower.maxFeePerGas);

    // eth_estimateGas responses from leader and follower should match
    const estimatedGasFromLeader = await gasLeaderClient.estimateGas({
      account: account.address,
      to: dummyContract.address,
      data: encodeFunctionData({
        abi: DummyContractAbi,
        functionName: "setPayload",
        args: [randomPayload],
      }),
    });
    logger.debug(`Fetched fee data from leader. estimatedGasFromLeader=${estimatedGasFromLeader}`);

    const estimatedGasFromFollower = await gasFollowerClient.estimateGas({
      account: account.address,
      to: dummyContract.address,
      data: encodeFunctionData({
        abi: DummyContractAbi,
        functionName: "setPayload",
        args: [randomPayload],
      }),
    });
    logger.debug(`Fetched fee data from follower. estimatedGasFromFollower=${estimatedGasFromFollower}`);

    expect(estimatedGasFromLeader).toEqual(estimatedGasFromFollower);

    let getBlockFromLeader = await gasLeaderClient.getBlock({
      blockTag: "finalized",
      includeTransactions: false,
    });
    let getBlockFromFollower = await gasFollowerClient.getBlock({
      blockTag: "finalized",
      includeTransactions: false,
    });

    // Finalized block numbers and hashes from leader and follower should match
    logger.debug(
      `Fetched finalized block and hash from leader. finalizedBlockNumber=${getBlockFromLeader.number} hash=${getBlockFromLeader.hash}`,
    );
    logger.debug(
      `Fetched finalized block and hash from follower. finalizedBlockNumber=${getBlockFromFollower.number} hash=${getBlockFromFollower.hash}`,
    );

    expect(getBlockFromFollower!.number).toEqual(getBlockFromLeader!.number);
    expect(getBlockFromFollower!.hash).toEqual(getBlockFromLeader!.hash);

    getBlockFromLeader = await gasLeaderClient.getBlock({
      blockTag: "latest",
      includeTransactions: false,
    });
    getBlockFromFollower = await gasFollowerClient.getBlock({
      blockTag: "latest",
      includeTransactions: false,
    });

    // Latest block numbers and hashes from leader and follower should match
    logger.debug(
      `Fetched latest block and hash from leader. finalizedBlockNumber=${getBlockFromLeader.number} hash=${getBlockFromLeader.hash}`,
    );
    logger.debug(
      `Fetched latest block and hash from follower. finalizedBlockNumber=${getBlockFromFollower.number} hash=${getBlockFromFollower.hash}`,
    );

    expect(getBlockFromFollower!.number).toEqual(getBlockFromLeader!.number);
    expect(getBlockFromFollower!.hash).toEqual(getBlockFromLeader!.hash);
  });
});
