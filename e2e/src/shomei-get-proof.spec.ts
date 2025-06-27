import { describe, it } from "@jest/globals";
import { toBeHex } from "ethers";
import { config } from "./config/tests-config";
import { awaitUntil, getDockerImageTag, LineaShomeiClient, LineaShomeiFrontendClient } from "./common/utils";

describe("Shomei Linea get proof test suite", () => {
  const lineaRollupV6 = config.getLineaRollupContract();
  const shomeiFrontendEndpoint = config.getShomeiFrontendEndpoint();
  const shomeiEndpoint = config.getShomeiEndpoint();
  const lineaShomeiFrontenedClient = new LineaShomeiFrontendClient(shomeiFrontendEndpoint!);
  const lineaShomeiClient = new LineaShomeiClient(shomeiEndpoint!);

  it.concurrent(
    "Call linea_getProof to Shomei frontend node and get a valid proof",
    async () => {
      const shomeiImageTag = await getDockerImageTag("shomei-frontend", "consensys/linea-shomei");
      logger.debug(`shomeiImageTag=${shomeiImageTag}`);

      let targetL2BlockNumber = await awaitUntil(
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
        2000,
        150000,
      );

      expect(targetL2BlockNumber).toBeGreaterThan(1n);

      const finalizedL2BlockNumbers = [targetL2BlockNumber!];
      const provingAddress = "0xfe3b557e8fb62b89f4916b721be55ceb828dbd73"; // from genesis file
      const getProofResponse = await awaitUntil(
        async () => {
          let getProofResponse;
          // Need to put all the latest currentL2BlockNumber in a list and traverse to get the proof
          // from one of them as we don't know on which finalized L2 block number the shomei frontend
          // was being notified
          for (const finalizedL2BlockNumber of finalizedL2BlockNumbers) {
            getProofResponse = await lineaShomeiFrontenedClient.lineaGetProof(
              provingAddress,
              [],
              toBeHex(finalizedL2BlockNumber),
            );
            if (getProofResponse?.result) {
              targetL2BlockNumber = finalizedL2BlockNumber;
              break;
            }
          }
          if (!getProofResponse?.result) {
            const latestFinalizedL2BlockNumber = await lineaRollupV6.currentL2BlockNumber({ blockTag: "finalized" });
            if (!finalizedL2BlockNumbers.includes(latestFinalizedL2BlockNumber)) {
              finalizedL2BlockNumbers.push(latestFinalizedL2BlockNumber);
              logger.debug(
                `finalizedL2BlockNumbers=${JSON.stringify(finalizedL2BlockNumbers.map((it) => Number(it)))}`,
              );
            }
          }
          return getProofResponse;
        },
        (getProofResponse) => getProofResponse?.result,
        2000,
        150000,
      );

      logger.debug(`targetL2BlockNumber=${targetL2BlockNumber}`);

      const {
        result: { zkEndStateRootHash },
      } = await lineaShomeiClient.rollupGetZkEVMStateMerkleProofV0(
        Number(targetL2BlockNumber),
        Number(targetL2BlockNumber),
        shomeiImageTag,
      );

      logger.debug(`zkEndStateRootHash=${zkEndStateRootHash}`);
      expect(zkEndStateRootHash).toBeDefined();

      const l2SparseMerkleProofContract = config.getL2SparseMerkleProofContract();
      const isValid = await l2SparseMerkleProofContract.verifyProof(
        getProofResponse.result.accountProof.proof.proofRelatedNodes,
        getProofResponse.result.accountProof.leafIndex,
        zkEndStateRootHash,
      );

      expect(isValid).toBeTruthy();

      // Modify the last hex character of the original state root hash should verify the same proof as invalid
      const modifiedStateRootHash =
        zkEndStateRootHash.slice(0, -1) + ((parseInt(zkEndStateRootHash.slice(-1), 16) + 1) % 16).toString(16);

      logger.debug(`originalStateRootHash=${zkEndStateRootHash}`);
      logger.debug(`modifiedStateRootHash=${modifiedStateRootHash}`);

      const isInvalid = !(await l2SparseMerkleProofContract.verifyProof(
        getProofResponse.result.accountProof.proof.proofRelatedNodes,
        getProofResponse.result.accountProof.leafIndex,
        modifiedStateRootHash,
      ));

      expect(isInvalid).toBeTruthy();
    },
    150_000,
  );
});
