import { prepareAndAddMessageMerkleRoot } from "./addAndClaimMessageHelper.js";
import { delay } from "../../../../common/helpers/general.js";
import { withSignerUi } from "../../../../hardhat-plugins/signer-ui/session.js";

import type { HardhatRuntimeEnvironment } from "hardhat/types/hre";
import type { TaskArguments } from "hardhat/types/tasks";

/*
  *******************************************************************************************
  Setup and execute TestLinethRollup.claimMessageWithProofAndWithdrawLST.

  1) Signer must have DEFAULT_ADMIN_ROLE role for TestLinethRollup
  2) L1MessageService balance must be < `value` (LST withdrawal requires deficit)
  3) Caller must be the `to` address (LST withdrawal recipient)

  -------------------------------------------------------------------------------------------
  Example (Hoodi):
  -------------------------------------------------------------------------------------------
  DEPLOYER_PRIVATE_KEY=<key> \
  CUSTOM_RPC_URL=https://0xrpc.io/hoodi \
  pnpm exec hardhat addAndClaimMessageForLST \
    --lineth-rollup-address <address> \
    --to <address> \
    --value <uint256> \
    --data <hex_string> \
    --yield-provider <address> \
    --network custom
  *******************************************************************************************
*/

// TASKS
export default async function addAndClaimMessageForLST(taskArgs: TaskArguments, hre: HardhatRuntimeEnvironment) {
  const connection = await hre.network.getOrCreate();
  return withSignerUi(hre, "task:addAndClaimMessageForLST", async () => {
    const { claimParams, linethRollup, publicClient } = await prepareAndAddMessageMerkleRoot(
      taskArgs,
      connection,
      true,
    );

    if (!claimParams.yieldProvider) {
      throw new Error("yieldProvider is required but was not provided");
    }

    {
      console.log("Waiting for 10 seconds...");
      await delay(10000);
      console.log("Claiming message with LST withdrawal...");
      const hash = await linethRollup.write.claimMessageWithProofAndWithdrawLST([
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
      ]);
      console.log("  Transaction hash:", hash);
      const receipt = await publicClient.waitForTransactionReceipt({ hash });
      console.log("  Transaction confirmed in block:", receipt.blockNumber.toString());
    }
  });
}
