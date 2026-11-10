import { prepareAndAddMessageMerkleRoot } from "./addAndClaimMessageHelper.js";
import { delay } from "../../../../common/helpers/general.js";
import { withSignerUi } from "../../../../hardhat-plugins/signer-ui/session.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { TaskArguments } from "hardhat/types/tasks";

/*
  *******************************************************************************************
  Setup and execute TestLinethRollup.claimMessageWithProof.

  1) Signer must have DEFAULT_ADMIN_ROLE role for TestLinethRollup
  2) TestLinethRollup must have >= `value` balance

  -------------------------------------------------------------------------------------------
  Example (Hoodi):
  -------------------------------------------------------------------------------------------
  DEPLOYER_PRIVATE_KEY=<key> \
  CUSTOM_RPC_URL=https://0xrpc.io/hoodi \
  pnpm exec hardhat addAndClaimMessage \
    --lineth-rollup-address <address> \
    --to <address> \
    --value <uint256> \
    --data <hex_string> \
    --network custom
  *******************************************************************************************
*/

// TASKS
export default async function addAndClaimMessage(taskArgs: TaskArguments, hre: HardhatRuntimeEnvironment) {
  const connection = await hre.network.getOrCreate();
  return withSignerUi(hre, "task:addAndClaimMessage", async () => {
    const { claimParams, linethRollup, publicClient } = await prepareAndAddMessageMerkleRoot(
      taskArgs,
      connection,
      false,
    );

    {
      console.log("Waiting for 10 seconds...");
      await delay(10000);
      console.log("Claiming message...");
      const hash = await linethRollup.write.claimMessageWithProof([
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
      ]);
      console.log("  Transaction hash:", hash);
      const receipt = await publicClient.waitForTransactionReceipt({ hash });
      console.log("  Transaction confirmed in block:", receipt.blockNumber.toString());
    }
  });
}
