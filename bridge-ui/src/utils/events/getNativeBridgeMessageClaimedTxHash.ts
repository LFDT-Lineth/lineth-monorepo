import { PublicClient } from "viem";

import { MessageClaimedABIEvent } from "@/types";

export const getNativeBridgeMessageClaimedTxHash = async (
  chainClient: PublicClient,
  messageServiceAddress: `0x${string}`,
  messageHash: `0x${string}`,
): Promise<string> => {
  const messageClaimedEvents = await chainClient.getLogs({
    event: MessageClaimedABIEvent,
    fromBlock: "earliest",
    toBlock: "latest",
    address: messageServiceAddress,
    args: {
      _messageHash: messageHash,
    },
  });
  if (messageClaimedEvents.length === 0) return "";
  return messageClaimedEvents[0].transactionHash;
};
