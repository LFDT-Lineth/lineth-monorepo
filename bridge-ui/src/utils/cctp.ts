import MessageTransmitterV2 from "@/abis/MessageTransmitterV2.json";
import { CctpAttestationMessage, Chain, TransactionStatus, CctpAttestationMessageStatus } from "@/types";
import { GetPublicClientReturnType } from "@wagmi/core";
import { fetchCctpAttestationByTxHash, reattestCctpV2PreFinalityMessage } from "@/services/cctp";
import { getPublicClient } from "@wagmi/core";
import { config as wagmiConfig } from "@/lib/wagmi";

// Current fast transfer route fee for mainnet
export const CCTP_TRANSFER_MAX_FEE_FALLBACK = 100n;
// 1000 Fast transfer, 2000 Standard transfer
export const CCTP_MIN_FINALITY_THRESHOLD = 1000;
// https://developers.circle.com/stablecoins/message-format, add 2 for '0x' prefix
const CCTP_V2_MESSAGE_HEADER_LENGTH = 298;
const CCTP_V2_EXPIRATION_BLOCK_OFFSET = 2 + 344 * 2;
const CCTP_V2_EXPIRATION_BLOCK_LENGTH = 64;

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

const getCctpMessageExpiryBlock = (message: string): bigint | undefined => {
  // See CCTPV2 message format at https://developers.circle.com/stablecoins/message-format
  const expiryInHex = message.substring(
    CCTP_V2_EXPIRATION_BLOCK_OFFSET,
    CCTP_V2_EXPIRATION_BLOCK_OFFSET + CCTP_V2_EXPIRATION_BLOCK_LENGTH,
  );
  const expiryInInt = parseInt(expiryInHex, 16);
  if (Number.isNaN(expiryInInt)) return undefined;
  // Return bigint because this is also returned by Viem client.getBlockNumber()
  return BigInt(expiryInInt);
};

export const getCctpTransactionStatus = async (
  toChain: Chain,
  cctpAttestationMessage: CctpAttestationMessage,
  nonce: string,
): Promise<TransactionStatus> => {
  const toChainClient = getPublicClient(wagmiConfig, {
    chainId: toChain.id,
  });
  if (!toChainClient) return TransactionStatus.PENDING;
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
  if (!attestationApiResp) return;
  const message = attestationApiResp.messages[0];
  if (!message) return;
  return message;
};
