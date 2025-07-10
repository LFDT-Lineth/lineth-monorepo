import { Account, Address, BaseError, Chain, Client, GetTransactionReceiptReturnType, Hex, Transport } from "viem";
import { getContractEvents, getTransactionReceipt } from "viem/actions";
import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";

export type GetTransactionReceiptByMessageHashParameters = {
  messageHash: Hex;
  // Defaults to the message service address for the chain
  messageServiceAddress?: Address;
};

export type GetTransactionReceiptByMessageHashReturnType<chain extends Chain | undefined> =
  GetTransactionReceiptReturnType<chain>;

/**
 * Returns the transaction receipt for a message sent by its message hash.
 *
 * @returns The transaction receipt of the message. {@link GetTransactionReceiptByMessageHashReturnType}
 * @param client - Client to use
 * @param args - {@link GetTransactionReceiptByMessageHashParameters}
 *
 * @example
 * import { createPublicClient, http } from 'viem'
 * import { linea } from 'viem/chains'
 * import { getTransactionReceiptByMessageHash } from '@consensys/linea-sdk-viem'
 *
 * const client = createPublicClient({
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const transactionReceipt = await getTransactionReceiptByMessageHash(client, {
 *   transactionHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
 * });
 */
export async function getTransactionReceiptByMessageHash<
  chain extends Chain | undefined,
  account extends Account | undefined,
>(
  client: Client<Transport, chain, account>,
  parameters: GetTransactionReceiptByMessageHashParameters,
): Promise<GetTransactionReceiptByMessageHashReturnType<chain>> {
  const { messageHash, messageServiceAddress } = parameters;

  const chainId = client.chain?.id;

  if (!chainId) {
    throw new BaseError("No chain id found in client");
  }

  const [event] = await getContractEvents(client, {
    address: messageServiceAddress ?? getContractsAddressesByChainId(chainId).messageService,
    abi: [
      {
        anonymous: false,
        inputs: [
          { indexed: true, internalType: "address", name: "_from", type: "address" },
          { indexed: true, internalType: "address", name: "_to", type: "address" },
          { indexed: false, internalType: "uint256", name: "_fee", type: "uint256" },
          { indexed: false, internalType: "uint256", name: "_value", type: "uint256" },
          { indexed: false, internalType: "uint256", name: "_nonce", type: "uint256" },
          { indexed: false, internalType: "bytes", name: "_calldata", type: "bytes" },
          { indexed: true, internalType: "bytes32", name: "_messageHash", type: "bytes32" },
        ],
        name: "MessageSent",
        type: "event",
      },
    ] as const,
    eventName: "MessageSent",
    args: {
      _messageHash: messageHash,
    },
    fromBlock: "earliest",
    toBlock: "latest",
  });

  if (!event) {
    throw new BaseError(`Message with hash ${messageHash} not found.`);
  }

  const receipt = await getTransactionReceipt(client, { hash: event.transactionHash });

  return receipt;
}
