import { Account, Address, BaseError, Chain, Client, Hex, Transport } from "viem";
import { getContractEvents } from "viem/actions";
import { getContractsAddressesByChainId } from "@consensys/linea-sdk-core";

export type GetMessageByMessageHashParameters = {
  messageHash: Hex;
  // Defaults to the message service address for the chain
  messageServiceAddress?: Address;
};

export type GetMessageByMessageHashReturnType = {
  from: Hex;
  to: Hex;
  fee: bigint;
  value: bigint;
  nonce: bigint;
  calldata: Hex;
  messageHash: Hex;
  transactionHash: Hex;
  blockNumber: bigint;
};

/**
 * Returns the details of a message by its hash.
 *
 * @returns The details of a message. {@link GetMessageByMessageHashReturnType}
 * @param client - Client to use
 * @param args - {@link GetMessageByMessageHashParameters}
 *
 * @example
 * import { createPublicClient, http } from 'viem'
 * import { linea } from 'viem/chains'
 * import { getMessageByMessageHash } from '@consensys/linea-sdk-viem'
 *
 * const client = createPublicClient({
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const message = await getMessageByMessageHash(client, {
 *   messageHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
 * });
 */
export async function getMessageByMessageHash<chain extends Chain | undefined, account extends Account | undefined>(
  client: Client<Transport, chain, account>,
  parameters: GetMessageByMessageHashParameters,
): Promise<GetMessageByMessageHashReturnType> {
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

  return {
    from: event.args._from!,
    to: event.args._to!,
    fee: event.args._fee!,
    value: event.args._value!,
    nonce: event.args._nonce!,
    calldata: event.args._calldata!,
    messageHash: event.args._messageHash!,
    transactionHash: event.transactionHash,
    blockNumber: event.blockNumber,
  };
}
