import { Client, SignTransactionParameters, Hash, Hex, Chain, Account, Transport, keccak256 } from "viem";
import { signTransaction } from "viem/actions";

export async function getRawTransactionHex<chain extends Chain | undefined, account extends Account | undefined>(
  client: Client<Transport, chain, account>,
  params: SignTransactionParameters<chain, account>,
): Promise<Hex> {
  return signTransaction(client, params);
}

export async function getTransactionHash<chain extends Chain | undefined, account extends Account | undefined>(
  client: Client<Transport, chain, account>,
  params: SignTransactionParameters<chain, account>,
): Promise<Hash> {
  const signedTransaction = await getRawTransactionHex(client, params);
  return keccak256(signedTransaction);
}

type WaitForTransactionReceiptResult = {
  status: string;
};

type TransactionReceiptWaitClient = {
  waitForTransactionReceipt: (params: { hash: Hash }) => Promise<WaitForTransactionReceiptResult>;
};

export async function expectSuccessfulTransaction(
  client: TransactionReceiptWaitClient,
  sendTransactionPromise: Promise<Hash>,
): Promise<void> {
  const txHash = await sendTransactionPromise;
  const txReceipt = await client.waitForTransactionReceipt({ hash: txHash });

  if (txReceipt.status !== "success") {
    throw new Error(`Expected successful transaction receipt, got status "${txReceipt.status}" for hash ${txHash}`);
  }
}
