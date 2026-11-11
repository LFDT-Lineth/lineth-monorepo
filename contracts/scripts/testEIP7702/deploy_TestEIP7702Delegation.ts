import hre, { network } from "hardhat";

import {
  getUiWalletClient,
  withSignerUi,
  warnIfUsingPrivateKeySigning,
} from "../../hardhat-plugins/signer-ui/session.js";

// Deploy on devnet - DEPLOYER_PRIVATE_KEY=<...> L1_RPC_URL=https://rpc.devnet.linea.build pnpm exec hardhat run --network zkevm_dev scripts/testEIP7702/deploy_TestEIP7702Delegation.ts

const func = async function () {
  const connection = await network.getOrCreate();
  warnIfUsingPrivateKeySigning({
    networkName: connection.networkName,
    scriptContext: "scripts/testEIP7702/deploy_TestEIP7702Delegation.ts",
  });

  await withSignerUi(hre, "scripts/testEIP7702/deploy_TestEIP7702Delegation.ts", async () => {
    const contractName = "TestEIP7702Delegation";
    const wallet = await getUiWalletClient(connection);
    const { contract, deploymentTransaction } = await connection.viem.sendDeploymentTransaction(contractName, [], {
      client: { wallet },
    });
    const publicClient = await connection.viem.getPublicClient();
    const receipt = await publicClient.waitForTransactionReceipt({ hash: deploymentTransaction.hash });
    console.log(
      `contract=${contractName} deployed: address=${contract.address} blockNumber=${receipt.blockNumber} chainId=${await publicClient.getChainId()}`,
    );
  });
};

func();
