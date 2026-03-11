import { getPublicClient } from "@wagmi/core";
import { Address, encodeFunctionData, padHex, zeroHash } from "viem";

import { config } from "@/config";
import { USDC_SYMBOL } from "@/constants/tokens";
import { BridgeProvider, ChainLayer, ClaimType } from "@/types";
import { ceilDiv, isUndefined, isUndefinedOrEmptyString } from "@/utils/misc";
import { isCctp } from "@/utils/tokens";

import { MESSAGE_TRANSMITTER_V2_ABI, TOKEN_MESSENGER_V2_ABI } from "./abis";
import { CCTP_MAX_FINALITY_THRESHOLD, CCTP_MIN_FINALITY_THRESHOLD } from "./constants";
import { CctpMessageReceivedAbiEvent } from "./events";
import { fetchCctpBridgeEvents } from "./history/fetchCctpBridgeEvents";
import { isCctpV2BridgeMessage } from "./message";
import { getCctpFee } from "./service";

import type {
  AdapterMode,
  BridgeAdapter,
  ClaimParams,
  DepositParams,
  EstimatedTime,
  FeeParams,
  HistoryParams,
  ReceivedAmountParams,
  TransactionRequest,
} from "../types";

const CCTP_LOGO = `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/cctp.svg`;

const CCTP_MODES = [
  { id: "STANDARD", label: "CCTP Standard", description: "No fees. Slower settlement", logoSrc: CCTP_LOGO },
  { id: "FAST", label: "CCTP Fast", description: "0.14% fee. Near-instant settlement", logoSrc: CCTP_LOGO },
] as const satisfies readonly AdapterMode[];

export type CctpModeId = (typeof CCTP_MODES)[number]["id"];

export const cctpAdapter: BridgeAdapter = {
  id: "cctp",
  name: "Circle CCTP",
  provider: BridgeProvider.CCTP,
  logoSrc: CCTP_LOGO,
  modes: CCTP_MODES,
  defaultMode: "STANDARD",

  isEnabled() {
    return config.isCctpEnabled;
  },

  matchesToken(token) {
    return token.symbol === USDC_SYMBOL;
  },

  canHandle(token, fromChain) {
    return isCctp(token) && !!fromChain.cctpTokenMessengerV2Address;
  },

  getEstimatedTime(fromChainLayer: ChainLayer, mode: CctpModeId | undefined): EstimatedTime {
    const isFromL1 = fromChainLayer === ChainLayer.L1;

    if (mode === "FAST") {
      return isFromL1 ? { min: 20, max: 20, unit: "second" } : { min: 8, max: 8, unit: "second" };
    }
    return isFromL1 ? { min: 13, max: 19, unit: "minute" } : { min: 2, max: 12, unit: "hour" };
  },

  buildDepositTx({
    token,
    amount,
    recipient,
    fromChain,
    toChain,
    fees,
    options,
  }: DepositParams): TransactionRequest | undefined {
    if (!recipient || fees.protocolFee === null) {
      return;
    }

    const mode = options?.selectedMode ?? "STANDARD";

    return {
      to: fromChain.cctpTokenMessengerV2Address,
      data: encodeFunctionData({
        abi: TOKEN_MESSENGER_V2_ABI,
        functionName: "depositForBurn",
        args: [
          amount,
          toChain.cctpDomain,
          padHex(recipient),
          token[fromChain.layer],
          zeroHash,
          fees.protocolFee,
          mode === "FAST" ? CCTP_MAX_FINALITY_THRESHOLD : CCTP_MIN_FINALITY_THRESHOLD,
        ],
      }),
      value: 0n,
      chainId: fromChain.id,
    };
  },

  buildClaimTx({ message, toChain }: ClaimParams): TransactionRequest | undefined {
    if (
      !isCctpV2BridgeMessage(message) ||
      isUndefinedOrEmptyString(message.attestation) ||
      isUndefinedOrEmptyString(message.message)
    ) {
      return;
    }

    return {
      to: toChain.cctpMessageTransmitterV2Address,
      data: encodeFunctionData({
        abi: MESSAGE_TRANSMITTER_V2_ABI,
        functionName: "receiveMessage",
        args: [message.message as `0x${string}`, message.attestation as `0x${string}`],
      }),
      value: 0n,
      chainId: toChain.id,
    };
  },

  computeReceivedAmount({ amount, fees }: ReceivedAmountParams): bigint {
    return fees.protocolFee ? amount - fees.protocolFee : amount;
  },

  getApprovalTarget(_token, fromChain): Address {
    return fromChain.cctpTokenMessengerV2Address;
  },

  async fetchHistory({ historyStoreActions, address, fromChain, toChain, tokens, wagmiConfig }: HistoryParams) {
    return fetchCctpBridgeEvents(historyStoreActions, address, fromChain, toChain, tokens, wagmiConfig);
  },

  async getFees({ amount, fromChain, toChain, options }: FeeParams) {
    const fees = await getCctpFee(fromChain.cctpDomain, toChain.cctpDomain, fromChain.testnet);
    const mode = (options?.selectedMode ?? "STANDARD") as CctpModeId;
    const threshold = mode === "FAST" ? CCTP_MAX_FINALITY_THRESHOLD : CCTP_MIN_FINALITY_THRESHOLD;
    const minimumFee = fees.find((fee) => fee.finalityThreshold === threshold)?.minimumFee;

    if (isUndefined(minimumFee)) {
      return { protocolFee: null, bridgingFee: 0n, claimType: ClaimType.MANUAL };
    }

    const protocolFee = ceilDiv(amount * BigInt(minimumFee), 10_000n);
    return { protocolFee, bridgingFee: 0n, claimType: ClaimType.MANUAL };
  },

  async getClaimingTxHash(message, toChain, wagmiConfig) {
    if (!isCctpV2BridgeMessage(message) || isUndefinedOrEmptyString(message.nonce)) return;
    const client = getPublicClient(wagmiConfig, { chainId: toChain.id });
    if (!client) return;
    const events = await client.getLogs({
      event: CctpMessageReceivedAbiEvent,
      fromBlock: "earliest",
      toBlock: "latest",
      address: toChain.cctpMessageTransmitterV2Address,
      args: { nonce: message.nonce },
    });
    return events[0]?.transactionHash;
  },

  getFallbackGasLimit() {
    return 112_409n;
  },
};
