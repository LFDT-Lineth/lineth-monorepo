import MessageTransmitterV2 from "@/abis/MessageTransmitterV2.json" assert { type: "json" };
import { CctpAttestationMessage, CctpAttestationMessageStatus, CCTPMode, Chain, TransactionStatus } from "@/types";
import { getPublicClient, GetPublicClientReturnType } from "@wagmi/core";
import { fetchCctpAttestationByTxHash, reattestCctpV2PreFinalityMessage } from "@/services/cctp";
import {
  CCTP_MAX_FINALITY_THRESHOLD,
  CCTP_V2_EXPIRATION_BLOCK_LENGTH,
  CCTP_V2_EXPIRATION_BLOCK_OFFSET,
  CCTP_V2_MESSAGE_HEADER_LENGTH,
} from "@/constants";
import { isUndefined } from "@/utils/utils";
import { Config } from "wagmi";

const isCctpNonceUsed = async (
  client: GetPublicClientReturnType,
  nonce: string,
  cctpMessageTransmitterV2Address: `0x${string}`,
): Promise<boolean> => {
  const resp = await client?.readContract({
    address: cctpMessageTransmitterV2Address,
    abi: MessageTransmitterV2.abi,
    functionName: "usedNonces",
    args: [nonce],
  });

  return resp === 1n;
};

export const getCctpMessageExpiryBlock = (message: string): bigint | undefined => {
  // See CCTPV2 message format at https://developers.circle.com/stablecoins/message-format
  const expiryInHex = message.substring(
    CCTP_V2_EXPIRATION_BLOCK_OFFSET,
    CCTP_V2_EXPIRATION_BLOCK_OFFSET + CCTP_V2_EXPIRATION_BLOCK_LENGTH,
  );
  // Should be 32-bytes
  if (expiryInHex.length !== 64) return undefined;
  const expiryInInt = parseInt(expiryInHex, 16);
  if (Number.isNaN(expiryInInt)) return undefined;
  // Return bigint because this is also returned by Viem client.getBlockNumber()
  return BigInt(expiryInInt);
};

export const getCctpTransactionStatus = async (
  toChain: Chain,
  cctpAttestationMessage: CctpAttestationMessage,
  nonce: string,
  wagmiConfig: Config,
): Promise<TransactionStatus> => {
  const toChainClient = getPublicClient(wagmiConfig, {
    chainId: toChain.id,
  });
  if (isUndefined(toChainClient)) return TransactionStatus.PENDING;
  // Attestation/message not yet available
  if (cctpAttestationMessage.message.length < CCTP_V2_MESSAGE_HEADER_LENGTH) return TransactionStatus.PENDING;
  const isNonceUsed = await isCctpNonceUsed(toChainClient, nonce, toChain.cctpMessageTransmitterV2Address);
  if (isNonceUsed) return TransactionStatus.COMPLETED;
  const messageExpiryBlock = getCctpMessageExpiryBlock(cctpAttestationMessage.message);
  if (messageExpiryBlock === undefined) return TransactionStatus.PENDING;
  // Message has no expiry
  if (messageExpiryBlock === 0n)
    return cctpAttestationMessage.status === CctpAttestationMessageStatus.PENDING_CONFIRMATIONS
      ? TransactionStatus.PENDING
      : TransactionStatus.READY_TO_CLAIM;

  // Message not expired
  const currentToBlock = await toChainClient.getBlockNumber();
  if (currentToBlock < messageExpiryBlock)
    return cctpAttestationMessage.status === CctpAttestationMessageStatus.PENDING_CONFIRMATIONS
      ? TransactionStatus.PENDING
      : TransactionStatus.READY_TO_CLAIM;

  // Message has expired, must reattest
  await reattestCctpV2PreFinalityMessage(nonce, toChain.testnet);

  /**
   * We will not re-query to get a new message/attestation set here:
   *
   * 1.) There is a concrete possibility that the new message status will be 'pending_confirmations', which will be a regression from the old message status of 'completed'
   * - To avoid this edge case, we will simply deem the transaction status as 'TransactionStatus.PENDING'
   * - TransactionStatus.PENDING means the user will not be able to claim, hence they have no need for a valid message/attestation set for this Transaction
   *
   * 2.) We avoid a CCTP API call that has a concrete possibility of returning a result consistent with TransactionStatus.PENDING anyway
   * - Even in the case that the new message status is 'complete', the only cost to the user is a page refresh
   */
  return TransactionStatus.PENDING;
};

export const getCctpMessageByTxHash = async (
  transactionHash: string,
  fromChainCctpDomain: number,
  isTestnet: boolean,
): Promise<CctpAttestationMessage | undefined> => {
  const attestationApiResp = await fetchCctpAttestationByTxHash(fromChainCctpDomain, transactionHash, isTestnet);
  if (isUndefined(attestationApiResp)) return;
  const message = attestationApiResp.messages[0];
  if (isUndefined(message)) return;
  return message;
};

export const getCctpModeFromFinalityThreshold = (minFinalityThreshold: number): CCTPMode => {
  if (minFinalityThreshold === CCTP_MAX_FINALITY_THRESHOLD) {
    return CCTPMode.FAST;
  }
  return CCTPMode.STANDARD;
};
