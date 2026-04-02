import { BlockNumber, BlockTag, type PublicClient } from "viem";
import { getContractEvents } from "viem/actions";

import {
  IL2MessageServiceLogClient,
  MessageSentEventFilters,
} from "../../../../core/clients/blockchain/linea/IL2MessageServiceLogClient";
import { Address, Hash, Hex, MessageSent } from "../../../../core/types";
import { L2MessageServiceAbi } from "../../abis/L2MessageServiceAbi";

type BlockParam = bigint | "latest" | "earliest" | "pending" | "safe" | "finalized";

function toBlockParam(block: BlockNumber | BlockTag | undefined, fallback: BlockParam): BlockParam {
  if (block === undefined) return fallback;
  if (typeof block === "number") return BigInt(block);
  return block as BlockParam;
}

export class ViemL2MessageServiceLogClient implements IL2MessageServiceLogClient {
  constructor(
    private readonly client: PublicClient,
    private readonly contractAddress: Address,
  ) {}

  public async getMessageSentEvents(params: {
    filters?: MessageSentEventFilters;
    fromBlock?: BlockNumber;
    toBlock?: BlockNumber | BlockTag;
    fromBlockLogIndex?: number;
  }): Promise<MessageSent[]> {
    const events = await getContractEvents(this.client, {
      address: this.contractAddress,
      abi: L2MessageServiceAbi,
      eventName: "MessageSent",
      args: {
        _from: params.filters?.from,
        _to: params.filters?.to,
        _messageHash: params.filters?.messageHash,
      },
      fromBlock: toBlockParam(params.fromBlock, "earliest"),
      toBlock: toBlockParam(params.toBlock, "latest"),
    });

    return events
      .filter((e) => !e.removed && e.blockNumber !== null && e.logIndex !== null)
      .filter((e) => {
        if (params.fromBlockLogIndex === undefined) return true;
        const sameBlock = e.blockNumber === params.fromBlock;
        return !sameBlock || e.logIndex! >= params.fromBlockLogIndex;
      })
      .map((e) => {
        const args = e.args as {
          _messageHash: Hash;
          _from: Address;
          _to: Address;
          _fee: bigint;
          _value: bigint;
          _nonce: bigint;
          _calldata: Hex;
        };
        return {
          messageHash: args._messageHash,
          messageSender: args._from,
          destination: args._to,
          fee: args._fee,
          value: args._value,
          messageNonce: args._nonce,
          calldata: args._calldata,
          contractAddress: this.contractAddress,
          blockNumber: Number(e.blockNumber),
          transactionHash: e.transactionHash!,
          logIndex: e.logIndex!,
        };
      });
  }
}
