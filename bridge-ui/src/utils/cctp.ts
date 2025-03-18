import MessageTransmitterV2 from "@/abis/MessageTransmitterV2.json";
import { CCTPMessageReceivedEvent, CctpAttestationMessageStatus, TransactionStatus } from "@/types";
import { GetPublicClientReturnType } from "@wagmi/core";
import { getAddress } from "viem";
import { eventCCTPMessageReceived } from "./events";

// Contract for sending CCTP message, appears constant for each chain
export const CCTP_TOKEN_MESSENGER = getAddress("0x8FE6B999Dc680CcFDD5Bf7EB0974218be2542DAA");
// TODO - Find optimal value
export const CCTP_TRANSFER_MAX_FEE = 500n;
// 1000 Fast transfer, 2000 Standard transfer
export const CCTP_MIN_FINALITY_THRESHOLD = 1000;
// Contract for receiving CCTP message, appears constant for each chain
export const CCTP_MESSAGE_TRANSMITTER = getAddress("0xE737e5cEBEEBa77EFE34D4aa090756590b1CE275"); // Appears constant for each chain

export const isCCTPNonceUsed = async (client: GetPublicClientReturnType, nonce: string): Promise<boolean> => {
  const resp = await client?.readContract({
    address: CCTP_MESSAGE_TRANSMITTER,
    abi: MessageTransmitterV2.abi,
    functionName: "usedNonces",
    args: [nonce],
  });

  return resp === 1n;
};

export const getCCTPTransactionStatus = (
  cctpMessageStatus: CctpAttestationMessageStatus,
  isNonceUsed: boolean,
): TransactionStatus => {
  if (cctpMessageStatus === "pending_confirmations") return TransactionStatus.PENDING;
  if (!isNonceUsed) return TransactionStatus.READY_TO_CLAIM;
  return TransactionStatus.COMPLETED;
};

export const getCCTPClaimTx = async (
  client: GetPublicClientReturnType,
  status: CctpAttestationMessageStatus,
  isNonceUsed: boolean,
  nonce: `0x${string}`,
): Promise<string | undefined> => {
  if (!client) return undefined;
  if (!isNonceUsed) return undefined;

  const messageReceivedEvents = <CCTPMessageReceivedEvent[]>await client.getLogs({
    event: eventCCTPMessageReceived,
    // TODO - Find more efficient `fromBlock` param than 'earliest'
    fromBlock: "earliest",
    toBlock: "latest",
    address: CCTP_MESSAGE_TRANSMITTER,
    args: {
      nonce: nonce,
    },
  });

  if (messageReceivedEvents.length === 0) return undefined;
  return messageReceivedEvents[0].transactionHash;
};
