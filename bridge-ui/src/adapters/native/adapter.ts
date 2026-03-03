import { getMessageProof } from "@consensys/linea-sdk-viem";
import { estimateFeesPerGas, getPublicClient, readContract } from "@wagmi/core";
import { Address, Client, Hex, encodeFunctionData, zeroAddress } from "viem";

import { config } from "@/config";
import { BridgeProvider, ChainLayer, ClaimType } from "@/types";
import { isUndefined, isUndefinedOrEmptyString, isZero } from "@/utils/misc";
import { isEth } from "@/utils/tokens";

import { MESSAGE_SERVICE_ABI, TOKEN_BRIDGE_ABI } from "./abis";
import { MessageClaimedABIEvent } from "./events";
import { estimateERC20BridgingGasUsed, estimateEthBridgingGasUsed } from "./fees";
import { fetchERC20BridgeEvents } from "./history/fetchERC20BridgeEvents";
import { fetchETHBridgeEvents } from "./history/fetchETHBridgeEvents";
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

export const nativeAdapter: BridgeAdapter = {
  id: "native",
  name: "Linea Native Bridge",
  hasAdvancedSettings: true,

  canHandle(token) {
    return token.bridgeProvider === BridgeProvider.NATIVE;
  },

  getEstimatedTime(fromChainLayer: ChainLayer): EstimatedTime {
    const isFromL1 = fromChainLayer === ChainLayer.L1;
    return isFromL1 ? { min: 20, max: 20, unit: "minute" } : { min: 2, max: 12, unit: "hour" };
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

  async prepareClaimMessage({ message, fromChain, toChain, wagmiConfig }) {
    if (
      !isNativeBridgeMessage(message) ||
      toChain.layer !== ChainLayer.L1 ||
      isUndefinedOrEmptyString(message.messageHash)
    ) {
      return;
    }

    const originLayerClient = getPublicClient(wagmiConfig, { chainId: fromChain.id });
    const destinationLayerClient = getPublicClient(wagmiConfig, { chainId: toChain.id });

    message.proof = await getMessageProof(destinationLayerClient as Client, {
      messageHash: message.messageHash as Hex,
      l2Client: originLayerClient as Client,
      ...(config.e2eTestMode
        ? {
            lineaRollupAddress: config.chains[toChain.id].messageServiceAddress as Address,
            l2MessageServiceAddress: config.chains[fromChain.id].messageServiceAddress as Address,
          }
        : {}),
    });
  },

  buildClaimTx({ message, toChain }: ClaimParams): TransactionRequest | undefined {
    if (
      !isNativeBridgeMessage(message) ||
      isUndefinedOrEmptyString(message.from) ||
      isUndefinedOrEmptyString(message.to) ||
      isUndefined(message.fee) ||
      isUndefined(message.value) ||
      isUndefined(message.nonce) ||
      message.nonce === 0n ||
      isUndefinedOrEmptyString(message.calldata) ||
      isUndefinedOrEmptyString(message.messageHash) ||
      (isUndefined(message.proof) && toChain.layer === ChainLayer.L1)
    ) {
      return;
    }

    const data =
      toChain.layer === ChainLayer.L1
        ? encodeFunctionData({
            abi: MESSAGE_SERVICE_ABI,
            functionName: "claimMessageWithProof",
            args: [
              {
                data: message.calldata as `0x{string}`,
                fee: message.fee,
                feeRecipient: zeroAddress,
                from: message.from,
                to: message.to,
                leafIndex: message.proof?.leafIndex as number,
                merkleRoot: message.proof?.root as `0x{string}`,
                messageNumber: message.nonce,
                proof: message.proof?.proof as `0x{string}`[],
                value: message.value,
              },
            ],
          })
        : encodeFunctionData({
            abi: MESSAGE_SERVICE_ABI,
            functionName: "claimMessage",
            args: [
              message.from,
              message.to,
              message.fee,
              message.value,
              zeroAddress,
              message.calldata as `0x{string}`,
              message.nonce,
            ],
          });

    return {
      to: toChain.messageServiceAddress,
      data,
      value: 0n,
      chainId: toChain.id,
    };
  },

  computeReceivedAmount({ amount, token, fromChainLayer, fees }: ReceivedAmountParams): bigint {
    if (!isEth(token)) return amount;
    if (fromChainLayer === ChainLayer.L1 && fees.claimType !== ClaimType.MANUAL) {
      return amount - fees.bridgingFee;
    }
    if (fromChainLayer === ChainLayer.L2) {
      return amount - fees.bridgingFee;
    }
    return amount;
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
        abi: [
          {
            inputs: [],
            name: "minimumFeeInWei",
            outputs: [{ internalType: "uint256", name: "", type: "uint256" }],
            stateMutability: "view",
            type: "function",
          },
        ],
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
