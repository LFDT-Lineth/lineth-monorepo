import {
  Client,
  SignTransactionParameters,
  Hash,
  Hex,
  Address,
  Chain,
  Account,
  Transport,
  keccak256,
  SendTransactionParameters,
  TransactionReceipt,
  encodeFunctionData,
} from "viem";
import { sendTransaction, signTransaction, waitForTransactionReceipt } from "viem/actions";

import { SendMessageArgs } from "../types";

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

export async function sendMessage<chain extends Chain | undefined, account extends Account>(
  client: Client<Transport, chain, account>,
  params: {
    contractAddress: Address;
    args: SendMessageArgs;
  } & SendTransactionParameters,
): Promise<TransactionReceipt> {
  const { contractAddress, args, ...overrides } = params;
  const txhash = await sendTransaction(client, {
    to: contractAddress,
    value: overrides?.value || 0n,
    data: encodeFunctionData({
      abi: [
        {
          inputs: [
            { internalType: "address", name: "_to", type: "address" },
            { internalType: "uint256", name: "_fee", type: "uint256" },
            { internalType: "bytes", name: "_calldata", type: "bytes" },
          ],
          name: "sendMessage",
          outputs: [],
          stateMutability: "payable",
          type: "function",
        },
      ] as const,
      functionName: "sendMessage",
      args: [args.to, args.fee, args.calldata],
    }),
    ...overrides,
  });

  const receipt = await waitForTransactionReceipt(client, { hash: txhash });

  if (!receipt) {
    throw new Error("Transaction receipt is undefined");
  }
  return receipt;
}
