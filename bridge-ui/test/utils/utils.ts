import { Client, Hex, parseEther } from "viem";
import { estimateMaxPriorityFeePerGas, getAddresses, sendTransaction, waitForTransactionReceipt } from "viem/actions";
import { estimateGas } from "viem/linea";

import { localL1Network, localL2Network } from "@/constants/chains";

export async function sendTransactionsToGenerateTrafficWithInterval(client: Client, pollingInterval: number = 1_000) {
  let timeoutId: NodeJS.Timeout | null = null;
  let isRunning = true;

  const [accountAddress] = await getAddresses(client);
  const sendTx = async () => {
    if (!isRunning) return;

    try {
      let transactionHash: Hex;
      if (client?.chain?.id === localL2Network.id) {
        const { priorityFeePerGas, baseFeePerGas } = await estimateGas(client, {
          account: client.account!,
          to: accountAddress,
          value: parseEther("0.000001"),
        });

        transactionHash = await sendTransaction(client, {
          to: accountAddress,
          value: parseEther("0.000001"),
          account: client.account!,
          chain: localL2Network,
          maxFeePerGas: baseFeePerGas + priorityFeePerGas,
          maxPriorityFeePerGas: priorityFeePerGas,
        });
      } else {
        const maxPriorityFeePerGas = await estimateMaxPriorityFeePerGas(client, { chain: localL1Network });

        transactionHash = await sendTransaction(client, {
          to: accountAddress,
          value: parseEther("0.000001"),
          account: client.account!,
          chain: localL1Network,
          maxFeePerGas: BigInt("0x7") + maxPriorityFeePerGas,
          maxPriorityFeePerGas,
        });
      }

      await waitForTransactionReceipt(client, { hash: transactionHash, confirmations: 1 });
      console.log(`Transaction sent successfully. hash=${transactionHash} chainId=${client?.chain?.id}`);
    } catch (error) {
      console.error(`Error sending transaction. error=${JSON.stringify(error)}`);
    } finally {
      if (isRunning) {
        timeoutId = setTimeout(sendTx, pollingInterval);
      }
    }
  };

  const stop = () => {
    isRunning = false;
    if (timeoutId) {
      clearTimeout(timeoutId);
      timeoutId = null;
    }
    console.log("Stopped generating traffic on L2");
  };

  sendTx();

  return stop;
}
