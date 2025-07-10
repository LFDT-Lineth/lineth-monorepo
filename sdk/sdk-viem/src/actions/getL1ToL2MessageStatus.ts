import { Account, Address, Chain, Client, Hex, Transport } from "viem";
import { readContract } from "viem/actions";
import { formatMessageStatus, getContractsAddressesByChainId, OnChainMessageStatus } from "@consensys/linea-sdk-core";

export type GetL1ToL2MessageStatusReturnType = OnChainMessageStatus;

export type GetL1ToL2MessageStatusParameters = {
  messageHash: Hex;
  // Defaults to the message service address for the L2 chain
  l2MessageServiceAddress?: Address;
};

/**
 * Returns the status of an L1 to L2 message on Linea.
 *
 * @returns The status of the L1 to L2 message. {@link GetL1ToL2MessageStatusReturnType}
 * @param client - Client to use
 * @param args - {@link GetL1ToL2MessageStatusParameters}
 *
 * @example
 * import { createPublicClient, http } from 'viem'
 * import { linea } from 'viem/chains'
 * import { getL1ToL2MessageStatus } from '@consensys/linea-sdk-viem'
 *
 * const client = createPublicClient({
 *   chain: linea,
 *   transport: http(),
 * });
 *
 * const messageStatus = await getL1ToL2MessageStatus(client, {
 *   messageHash: '0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
 * });
 */
export async function getL1ToL2MessageStatus<chain extends Chain | undefined, account extends Account | undefined>(
  client: Client<Transport, chain, account>,
  parameters: GetL1ToL2MessageStatusParameters,
): Promise<GetL1ToL2MessageStatusReturnType> {
  const { messageHash, l2MessageServiceAddress } = parameters;

  if (!client.chain) {
    throw new Error("Client chain is required to get L1 to L2 message status.");
  }

  const l2MessageService = l2MessageServiceAddress ?? getContractsAddressesByChainId(client.chain.id).messageService;

  const status = await readContract(client, {
    address: l2MessageService,
    abi: [
      {
        inputs: [{ internalType: "bytes32", name: "messageHash", type: "bytes32" }],
        name: "inboxL1L2MessageStatus",
        outputs: [{ internalType: "uint256", name: "messageStatus", type: "uint256" }],
        stateMutability: "view",
        type: "function",
      },
    ],
    functionName: "inboxL1L2MessageStatus",
    args: [messageHash],
  });

  return formatMessageStatus(status);
}
