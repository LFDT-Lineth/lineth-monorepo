import { TransactionReceipt } from "viem";
import { parseEventLogs } from "viem";

import { MessageEvent } from "../../types";

const MESSAGE_SENT_ABI = [
  {
    anonymous: false,
    inputs: [
      {
        indexed: true,
        internalType: "address",
        name: "_from",
        type: "address",
      },
      {
        indexed: true,
        internalType: "address",
        name: "_to",
        type: "address",
      },
      {
        indexed: false,
        internalType: "uint256",
        name: "_fee",
        type: "uint256",
      },
      {
        indexed: false,
        internalType: "uint256",
        name: "_value",
        type: "uint256",
      },
      {
        indexed: false,
        internalType: "uint256",
        name: "_nonce",
        type: "uint256",
      },
      {
        indexed: false,
        internalType: "bytes",
        name: "_calldata",
        type: "bytes",
      },
      {
        indexed: true,
        internalType: "bytes32",
        name: "_messageHash",
        type: "bytes32",
      },
    ],
    name: "MessageSent",
    type: "event",
  },
] as const;

/**
 * Extracts MessageSent events from a list of receipts.
 */
export function getMessageSentEventFromLogs(receipts: TransactionReceipt[]): MessageEvent[] {
  const logs = receipts.flatMap((r) => r.logs);

  const parsedLogs = parseEventLogs({
    abi: MESSAGE_SENT_ABI,
    eventName: "MessageSent",
    logs,
    strict: true,
  });

  return parsedLogs.map((log) => ({
    from: log.args._from,
    to: log.args._to,
    fee: log.args._fee,
    value: log.args._value,
    messageNumber: log.args._nonce,
    calldata: log.args._calldata,
    messageHash: log.args._messageHash,
    blockNumber: log.blockNumber,
  }));
}
