import { task } from "hardhat/config";
import { delay } from "../../../../common/helpers/general";
import { prepareAndAddMessageMerkleRoot } from "./addAndClaimMessageHelper";

/*
  *******************************************************************************************
  Setup and execute TestLineaRollup.claimMessageWithProofAndWithdrawLST.

  1) Signer must have DEFAULT_ADMIN_ROLE role for TestLineaRollup
  2) L1MessageService balance must be < `value` (LST withdrawal requires deficit)
  3) Caller must be the `to` address (LST withdrawal recipient)

  -------------------------------------------------------------------------------------------
  Example (Hoodi):
  -------------------------------------------------------------------------------------------
  DEPLOYER_PRIVATE_KEY=<key> \
  CUSTOM_RPC_URL=https://0xrpc.io/hoodi \
  npx hardhat addAndClaimMessageForLST \
    --linea-rollup-address <address> \
    --to <address> \
    --value <uint256> \
    --data <hex_string> \
    --yield-provider <address> \
    --network custom
  *******************************************************************************************
*/

// TASKS
task(
  "addAndClaimMessageForLST",
  "Setup and execute TestLineaRollup.claimMessageWithProofAndWithdrawLST by adding L2->L1 message merkle tree root",
)
  .addOptionalParam("lineaRollupAddress")
  .addOptionalParam("from")
  .addOptionalParam("to")
  .addOptionalParam("value")
  .addOptionalParam("data")
  .addOptionalParam("yieldProvider")
  .setAction(async (taskArgs, hre) => {
    const { claimParams, lineaRollup } = await prepareAndAddMessageMerkleRoot(taskArgs, hre, true);

    if (!claimParams.yieldProvider) {
      throw new Error("yieldProvider is required but was not provided");
    }

    {
      console.log("Waiting for 10 seconds...");
      await delay(10000);
      console.log("Claiming message with LST withdrawal...");
      const tx = await lineaRollup.claimMessageWithProofAndWithdrawLST(
        {
          proof: claimParams.proof,
          messageNumber: claimParams.messageNumber,
          leafIndex: claimParams.leafIndex,
          from: claimParams.from,
          to: claimParams.to,
          fee: claimParams.fee,
          value: claimParams.value,
          feeRecipient: claimParams.feeRecipient,
          merkleRoot: claimParams.merkleRoot,
          data: claimParams.data,
        },
        claimParams.yieldProvider,
      );
      console.log("  Transaction hash:", tx.hash);
      const receipt = await tx.wait();
      console.log("  Transaction confirmed in block:", receipt?.blockNumber);
    }
  });
