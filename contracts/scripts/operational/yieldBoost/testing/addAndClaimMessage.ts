import { task } from "hardhat/config";
import { delay } from "../../../../common/helpers/general";
import { prepareAndAddMessageMerkleRoot } from "./addAndClaimMessageHelper";

/*
  *******************************************************************************************
  Setup and execute TestLineaRollup.claimMessageWithProof.

  1) Signer must have DEFAULT_ADMIN_ROLE role for TestLineaRollup
  2) TestLineaRollup must have >= `value` balance

  -------------------------------------------------------------------------------------------
  Example (Hoodi):
  -------------------------------------------------------------------------------------------
  DEPLOYER_PRIVATE_KEY=<key> \
  CUSTOM_RPC_URL=https://0xrpc.io/hoodi \
  npx hardhat addAndClaimMessage \
    --linea-rollup-address <address> \
    --to <address> \
    --value <uint256> \
    --data <hex_string> \
    --network custom
  *******************************************************************************************
*/

// TASKS
task(
  "addAndClaimMessage",
  "Setup and execute TestLineaRollup.claimMessageWithProof by adding L2->L1 message merkle tree root",
)
  .addOptionalParam("lineaRollupAddress")
  .addOptionalParam("from")
  .addOptionalParam("to")
  .addOptionalParam("value")
  .addOptionalParam("data")
  .setAction(async (taskArgs, hre) => {
    const { claimParams, lineaRollup } = await prepareAndAddMessageMerkleRoot(taskArgs, hre, false);

    {
      console.log("Waiting for 10 seconds...");
      await delay(10000);
      console.log("Claiming message...");
      const tx = await lineaRollup.claimMessageWithProof({
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
      });
      console.log("  Transaction hash:", tx.hash);
      const receipt = await tx.wait();
      console.log("  Transaction confirmed in block:", receipt?.blockNumber);
    }
  });
