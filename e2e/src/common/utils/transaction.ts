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
