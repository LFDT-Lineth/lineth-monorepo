import { readContract, getPublicClient } from "@wagmi/core";
import { Address, encodeFunctionData } from "viem";

import { config } from "@/config";
import { MUSD_SYMBOL } from "@/constants/tokens";
import { Token, Chain, BridgeTransaction, BridgeProvider, ClaimType } from "@/types";
import { isUndefined } from "@/utils/misc";

import {
  BridgeAdapter,
  FeeParams,
  DepositParams,
  HistoryParams,
  ReceivedAmountParams,
  TransactionRequest,
} from "../types";
import { HUB_PORTAL_ABI } from "./abis";
import { MTokenReceivedAbiEvent, MTokenReceivedLogEvent } from "./events";
import { fetchHyperlaneBridgeEvents } from "./history/fetchHyperlaneBridgeEvents";
import { isHyperlaneBridgeMessage } from "./message";

const HYPERLANE_LOGO = `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/hyperlane.svg`;

export const hyperlaneAdapter: BridgeAdapter = {
  id: "hyperlane",
  name: "Hyperlane",
  provider: BridgeProvider.HYPERLANE,
  logoSrc: HYPERLANE_LOGO,
  bridgingFeeLabel: "Hyperlane fee",

  isEnabled() {
    return config.isHyperlaneEnabled;
  },

  matchesToken(token) {
    return token.symbol === MUSD_SYMBOL;
  },

  canHandle: function (token: Token, fromChain: Chain, toChain: Chain): boolean {
    return (
      token.bridgeProvider === BridgeProvider.HYPERLANE &&
      !isUndefined(fromChain.hyperlanePortalLiteAddress) &&
      !isUndefined(toChain.hyperlanePortalLiteAddress)
    );
  },
  getEstimatedTime() {
    return { min: 3, max: 3, unit: "minute" };
  },
  buildDepositTx: function ({
    fromChain,
    token,
    toChain,
    recipient,
    amount,
    fees,
  }: DepositParams): TransactionRequest | undefined {
    if (!fromChain.hyperlanePortalLiteAddress || !recipient || !fees.bridgingFee) {
      return;
    }

    return {
      to: fromChain.hyperlanePortalLiteAddress,
      data: encodeFunctionData({
        abi: HUB_PORTAL_ABI,
        functionName: "transferMLikeToken",
        args: [amount, token[fromChain.layer], BigInt(toChain.id), token[toChain.layer], recipient, recipient],
      }),
      value: fees.bridgingFee,
      chainId: fromChain.id,
    };
  },
  async getFees({ amount, fromChain, toChain, recipient, wagmiConfig }: FeeParams) {
    const fee = await readContract(wagmiConfig, {
      abi: HUB_PORTAL_ABI,
      address: fromChain.hyperlanePortalLiteAddress as Address,
      functionName: "quoteTransfer",
      chainId: fromChain.id,
      args: [amount, BigInt(toChain.id), recipient],
    });
    return { protocolFee: null, bridgingFee: fee, claimType: ClaimType.AUTO_PAID };
  },
  getApprovalTarget: function (_token: Token, fromChain: Chain): Address | undefined {
    return fromChain.hyperlanePortalLiteAddress;
  },
  computeReceivedAmount({ amount }: ReceivedAmountParams): bigint {
    return amount;
  },
  async getClaimingTxHash(message, toChain, wagmiConfig) {
    if (!isHyperlaneBridgeMessage(message) || isUndefined(toChain.hyperlanePortalLiteAddress)) return;

    const client = getPublicClient(wagmiConfig, { chainId: toChain.id });
    if (!client) return;

    const logs = (await client.getLogs({
      event: MTokenReceivedAbiEvent,
      fromBlock: "earliest",
      toBlock: "latest",
      address: toChain.hyperlanePortalLiteAddress,
      args: {
        sender: message.sender,
        recipient: message.recipient,
      },
    })) as unknown as MTokenReceivedLogEvent[];

    const match = logs.find((log) => log.args.index === message.transferIndex);
    return match?.transactionHash;
  },
  fetchHistory({
    historyStoreActions,
    address,
    fromChain,
    toChain,
    tokens,
    wagmiConfig,
  }: HistoryParams): Promise<BridgeTransaction[]> {
    return fetchHyperlaneBridgeEvents(historyStoreActions, address, fromChain, toChain, tokens, wagmiConfig);
  },
};
