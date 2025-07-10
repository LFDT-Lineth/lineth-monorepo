import {
  Abi,
  Account,
  Address,
  BlockNumber,
  BlockTag,
  Chain,
  Client,
  ContractEventName,
  GetContractEventsParameters,
  Hash,
  Hex,
  Transport,
} from "viem";
import { getContractEvents } from "viem/actions";

type EventLogBase = {
  blockNumber: number;
  logIndex: number;
  contractAddress: string;
  transactionHash: string;
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

export async function getMessageSentEvents<
  chain extends Chain | undefined,
  account extends Account | undefined,
  strict extends boolean | undefined = undefined,
  fromBlock extends BlockNumber | BlockTag | undefined = undefined,
  toBlock extends BlockNumber | BlockTag | undefined = undefined,
>(
  client: Client<Transport, chain, account>,
  parameters: GetMessageSentEventsParameters<
    [
      {
        anonymous: false;
        inputs: [
          { indexed: true; internalType: "address"; name: "_from"; type: "address" },
          { indexed: true; internalType: "address"; name: "_to"; type: "address" },
          { indexed: false; internalType: "uint256"; name: "_fee"; type: "uint256" },
          { indexed: false; internalType: "uint256"; name: "_value"; type: "uint256" },
          { indexed: false; internalType: "uint256"; name: "_nonce"; type: "uint256" },
          { indexed: false; internalType: "bytes"; name: "_calldata"; type: "bytes" },
          { indexed: true; internalType: "bytes32"; name: "_messageHash"; type: "bytes32" },
        ];
        name: "MessageSent";
        type: "event";
      },
    ],
    "MessageSent",
    strict,
    fromBlock,
    toBlock
  >,
) {
  const events = await getContractEvents(client, {
    address: parameters.address,
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
    args: parameters.args,
    fromBlock: parameters.fromBlock ?? "earliest",
    toBlock: parameters.toBlock ?? "latest",
  });

  return events
    .filter((event) => event.removed === false)
    .map((event) => ({
      messageSender: event.args._from!,
      destination: event.args._to!,
      fee: event.args._fee!,
      value: event.args._value!,
      messageNonce: event.args._nonce!,
      calldata: event.args._calldata!,
      messageHash: event.args._messageHash!,
      blockNumber: event.blockNumber,
      logIndex: event.logIndex,
      contractAddress: event.address,
      transactionHash: event.transactionHash,
    }));
}
