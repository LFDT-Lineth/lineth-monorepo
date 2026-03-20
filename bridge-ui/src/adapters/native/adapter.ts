import { getMessageProof } from "@consensys/linea-sdk-viem";
import { estimateFeesPerGas, getPublicClient, readContract } from "@wagmi/core";
import { Address, Client, Hex, encodeFunctionData } from "viem";

import { config } from "@/config";
import { BridgeProvider, ChainLayer, ClaimType } from "@/types";
import { isUndefinedOrEmptyString, isZero } from "@/utils/misc";
import { isEth } from "@/utils/tokens";

import { MESSAGE_SERVICE_ABI, MINIMUM_FEE_ABI, TOKEN_BRIDGE_ABI } from "./abis";
import { buildStEthClaimMessages, encodeL1ClaimData, encodeL2ClaimData, isValidClaimMessage } from "./claim";
import { MessageClaimedABIEvent } from "./events";
import { estimateERC20BridgingGasUsed, estimateEthBridgingGasUsed } from "./fees";
import { fetchERC20BridgeEvents } from "./history/fetchERC20BridgeEvents";
import { fetchETHBridgeEvents } from "./history/fetchETHBridgeEvents";
import { fetchMessageServiceBalance, isL2ToL1EthWithYieldProvider } from "./liquidity";
import { isNativeBridgeMessage } from "./message";

import type {
  BridgeAdapter,
  ClaimParams,
  DepositParams,
  EstimatedTime,
  FeeParams,
  HistoryParams,
  ReceivedAmountParams,
  TransactionRequest,
} from "../types";

const MAX_POSTMAN_SPONSOR_GAS_LIMIT = 250000n;
const NATIVE_LOGO = `${process.env.NEXT_PUBLIC_BASE_PATH}/images/logo/linea-rounded.svg`;

export const nativeAdapter: BridgeAdapter = {
  id: "native",
  name: "Linea Native Bridge",
  provider: BridgeProvider.NATIVE,
  logoSrc: NATIVE_LOGO,
  hasAdvancedSettings: true,

  isEnabled() {
    return true;
  },

  matchesToken() {
    return false;
  },

  canHandle(token) {
    return token.bridgeProvider === BridgeProvider.NATIVE;
  },

  getEstimatedTime(fromChainLayer: ChainLayer): EstimatedTime {
    return fromChainLayer === ChainLayer.L1 ? { min: 20, max: 20, unit: "minute" } : { min: 2, max: 12, unit: "hour" };
  },

  buildDepositTx({ token, amount, recipient, fromChain, fees }: DepositParams): TransactionRequest | undefined {
    const { bridgingFee, claimType } = fees;

    if (
      !recipient ||
      (isZero(bridgingFee) && fromChain.layer === ChainLayer.L2) ||
      (isZero(bridgingFee) && claimType === ClaimType.AUTO_PAID)
    ) {
      return;
    }

    if (isEth(token)) {
      if (amount === 0n) return;
      return {
        to: fromChain.messageServiceAddress,
        data: encodeFunctionData({
          abi: MESSAGE_SERVICE_ABI,
          functionName: "sendMessage",
          args: [recipient, bridgingFee, "0x"],
        }),
        value: amount + bridgingFee,
        chainId: fromChain.id,
      };
    }

    return {
      to: fromChain.tokenBridgeAddress,
      data: encodeFunctionData({
        abi: TOKEN_BRIDGE_ABI,
        functionName: "bridgeToken",
        args: [token[fromChain.layer], amount, recipient],
      }),
      value: bridgingFee,
      chainId: fromChain.id,
    };
  },

  async buildClaimTx({ message, fromChain, toChain, options, wagmiConfig }: ClaimParams) {
    if (!isNativeBridgeMessage(message)) return;

    let claimMessage = message;

    if (toChain.layer === ChainLayer.L1 && !message.proof) {
      if (isUndefinedOrEmptyString(message.messageHash)) return;

      const originLayerClient = getPublicClient(wagmiConfig, { chainId: fromChain.id });
      const destinationLayerClient = getPublicClient(wagmiConfig, { chainId: toChain.id });

      const proof = await getMessageProof(destinationLayerClient as Client, {
        messageHash: message.messageHash as Hex,
        l2Client: originLayerClient as Client,
        ...(config.e2eTestMode
          ? {
              lineaRollupAddress: config.chains[toChain.id].messageServiceAddress as Address,
              l2MessageServiceAddress: config.chains[fromChain.id].messageServiceAddress as Address,
            }
          : {}),
      });

      claimMessage = { ...message, proof };
    }

    if (!isValidClaimMessage(claimMessage, toChain)) return;

    const data =
      toChain.layer === ChainLayer.L1
        ? encodeL1ClaimData(claimMessage, toChain, options)
        : encodeL2ClaimData(claimMessage);
    if (!data) return;

    return { to: toChain.messageServiceAddress, data, value: 0n, chainId: toChain.id };
  },

  async getDepositWarnings({ token, fromChain, toChain, amount, wagmiConfig }) {
    if (!isL2ToL1EthWithYieldProvider(token, fromChain, toChain) || amount <= 0n) return undefined;

    try {
      const balance = await fetchMessageServiceBalance(wagmiConfig, toChain);
      if (amount > balance) {
        return [
          {
            text: "The bridge is currently congested.",
            // TODO: uncomment this once Yield boost doc is ready.
            // link: { url: "https://docs.linea.build/network/overview/yield-boost", label: "Learn more." },
          },
        ];
      }
    } catch {
      // RPC failure — skip warning
    }
    return undefined;
  },

  async getClaimContext({ transaction, connectedAddress, wagmiConfig }) {
    const { fromChain, toChain, token, message } = transaction;
    if (!isNativeBridgeMessage(message) || !isL2ToL1EthWithYieldProvider(token, fromChain, toChain)) return undefined;

    let isLowLiquidity = false;
    try {
      if (message.amountSent > 0n) {
        const balance = await fetchMessageServiceBalance(wagmiConfig, toChain);
        isLowLiquidity = message.amountSent > balance;
      }
    } catch {
      // RPC failure — default to false so standard claim proceeds
    }

    if (!isLowLiquidity) return undefined;

    const isRecipient = !!connectedAddress && message.to.toLowerCase() === connectedAddress.toLowerCase();

    return {
      label: "Claim stETH",
      errorMessage:
        "stETH claiming is currently unavailable. Please wait until stETH or ETH claiming becomes available.",
      messages: buildStEthClaimMessages(isRecipient),
      ...(isRecipient ? { claimOptions: { useAlternativeClaim: true } } : {}),
    };
  },

  computeReceivedAmount({ amount, token, fromChainLayer, fees }: ReceivedAmountParams): bigint {
    if (!isEth(token)) return amount;
    if (fromChainLayer === ChainLayer.L1 && fees.claimType === ClaimType.MANUAL) return amount;
    return amount - fees.bridgingFee;
  },

  getApprovalTarget(token, fromChain): Address | undefined {
    if (isEth(token)) return undefined;
    return fromChain.tokenBridgeAddress;
  },

  async fetchHistory({ historyStoreActions, address, fromChain, toChain, tokens, wagmiConfig }: HistoryParams) {
    const [ethEvents, erc20Events] = await Promise.all([
      fetchETHBridgeEvents(historyStoreActions, address, fromChain, toChain, tokens, wagmiConfig),
      fetchERC20BridgeEvents(historyStoreActions, address, fromChain, toChain, tokens, wagmiConfig),
    ]);
    return [...ethEvents, ...erc20Events];
  },

  async getClaimingTxHash(message, toChain, wagmiConfig) {
    if (!isNativeBridgeMessage(message) || isUndefinedOrEmptyString(message.messageHash)) return;
    const client = getPublicClient(wagmiConfig, { chainId: toChain.id });
    if (!client) return;
    const events = await client.getLogs({
      event: MessageClaimedABIEvent,
      fromBlock: "earliest",
      toBlock: "latest",
      address: toChain.messageServiceAddress,
      args: { _messageHash: message.messageHash as `0x${string}` },
    });
    return events[0]?.transactionHash;
  },

  getFallbackGasLimit(token) {
    if (isEth(token)) return undefined;
    return 133_000n;
  },

  async getFees({ amount, token, fromChain, toChain, address, recipient, wagmiConfig, options }: FeeParams) {
    if (fromChain.layer === ChainLayer.L2) {
      const minimumFee = await readContract(wagmiConfig, {
        address: fromChain.messageServiceAddress,
        abi: MINIMUM_FEE_ABI,
        functionName: "minimumFeeInWei",
        chainId: fromChain.id,
      });
      return { protocolFee: null, bridgingFee: minimumFee, claimType: ClaimType.MANUAL };
    }

    if (options?.manualClaim || amount === 0n) {
      return {
        protocolFee: null,
        bridgingFee: 0n,
        claimType: options?.manualClaim ? ClaimType.MANUAL : ClaimType.AUTO_SPONSORED,
      };
    }

    const nextMessageNumber = await readContract(wagmiConfig, {
      address: fromChain.messageServiceAddress,
      abi: MESSAGE_SERVICE_ABI,
      functionName: "nextMessageNumber",
      chainId: fromChain.id,
    });

    const estimateFn = isEth(token) ? estimateEthBridgingGasUsed : estimateERC20BridgingGasUsed;
    const gasUsed = await estimateFn({
      address,
      recipient,
      amount,
      token,
      fromChain,
      toChain,
      claimingType: ClaimType.AUTO_PAID,
      nextMessageNumber,
      wagmiConfig,
    });

    const gasWithSurplus = gasUsed + fromChain.gasLimitSurplus;
    if (gasWithSurplus < MAX_POSTMAN_SPONSOR_GAS_LIMIT) {
      return { protocolFee: null, bridgingFee: 0n, claimType: ClaimType.AUTO_SPONSORED };
    }

    const feeData = await estimateFeesPerGas(wagmiConfig, { chainId: toChain.id, type: "eip1559" });
    const bridgingFee = feeData.maxFeePerGas * gasWithSurplus * fromChain.profitMargin;

    return { protocolFee: null, bridgingFee, claimType: ClaimType.AUTO_PAID };
  },
};
