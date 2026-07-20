import {
  Abi,
  Account,
  Address,
  BlockNumber,
  BlockTag,
  Chain,
  Client,
  ContractEventName,
  GetContractEventsErrorType,
  GetContractEventsParameters,
  Hash,
  Hex,
  Transport,
} from "viem";
import { getContractEvents } from "viem/actions";

import { MESSAGE_SENT_EVENT_ABI } from "../abis";

type EventLogBase = {
  blockNumber: bigint | null;
  logIndex: number | null;
  contractAddress: Address;
  transactionHash: Hash | null;
};

type MessageSent = {
  messageSender: Address;
  destination: Address;
  fee: bigint;
  value: bigint;
  messageNonce: bigint;
  calldata: Hex;
  messageHash: Hash;
} & EventLogBase;

export type GetMessageSentEventsReturnType = MessageSent[];

export type GetMessageSentEventsParameters<
  abi extends Abi | readonly unknown[] = Abi,
  eventName extends ContractEventName<abi> | undefined = ContractEventName<abi> | undefined,
  strict extends boolean | undefined = undefined,
  fromBlock extends BlockNumber | BlockTag | undefined = undefined,
  toBlock extends BlockNumber | BlockTag | undefined = undefined,
> = Pick<
  GetContractEventsParameters<abi, eventName, strict, fromBlock, toBlock>,
  "args" | "fromBlock" | "toBlock" | "address"
>;

export type GetMessageSentEventsErrorType = GetContractEventsErrorType;

export async function getMessageSentEvents<
  chain extends Chain | undefined,
  account extends Account | undefined,
  strict extends boolean | undefined = undefined,
  fromBlock extends BlockNumber | BlockTag | undefined = undefined,
  toBlock extends BlockNumber | BlockTag | undefined = undefined,
>(
  client: Client<Transport, chain, account>,
  parameters: GetMessageSentEventsParameters<typeof MESSAGE_SENT_EVENT_ABI, "MessageSent", strict, fromBlock, toBlock>,
) {
  const events = await getContractEvents(client, {
    address: parameters.address,
    abi: MESSAGE_SENT_EVENT_ABI,
    eventName: "MessageSent",
    args: parameters.args,
    fromBlock: parameters.fromBlock ?? "earliest",
    toBlock: parameters.toBlock ?? "latest",
    strict: true,
  });

  return events
    .filter((event) => event.removed === false)
    .map((event) => ({
      messageSender: event.args._from,
      destination: event.args._to,
      fee: event.args._fee,
      value: event.args._value,
      messageNonce: event.args._nonce,
      calldata: event.args._calldata,
      messageHash: event.args._messageHash,
      blockNumber: event.blockNumber,
      logIndex: event.logIndex,
      contractAddress: event.address,
      transactionHash: event.transactionHash,
    }));
}
