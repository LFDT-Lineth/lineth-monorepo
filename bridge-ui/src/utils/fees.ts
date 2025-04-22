import { zeroAddress, encodeFunctionData, encodeAbiParameters, Address } from "viem";
import { getPublicClient } from "@wagmi/core";
import TokenBridge from "@/abis/TokenBridge.json";
import MessageService from "@/abis/MessageService.json";
import { computeMessageHash, computeMessageStorageSlot } from "./message";
import { Chain, ClaimType, Token } from "@/types";
import { config } from "@/lib/wagmi";
import { isUndefined } from "@/utils";

interface EstimationParams {
  address: Address;
  recipient: Address;
  amount: bigint;
  nextMessageNumber: bigint;
  fromChain: Chain;
  toChain: Chain;
  claimingType: ClaimType;
}

/**
 * Creates a state override object for the gas estimation call.
 */
function createStateOverride(messageServiceAddress: Address, storageSlot: `0x${string}`) {
  return [
    {
      address: messageServiceAddress,
      stateDiff: [
        {
          slot: storageSlot,
          value: "0x0000000000000000000000000000000000000000000000000000000000000001",
        },
      ],
    },
  ];
}

/**
 * Prepares ERC20-specific parameters by checking for a native token.
 */
async function prepareERC20TokenParams(
  originChainPublicClient: ReturnType<typeof getPublicClient>,
  address: Address,
  token: Token,
  fromChain: Chain,
  toChain: Chain,
): Promise<{ tokenAddress: Address; chainId: number; tokenMetadata: string }> {
  const nativeToken = (await originChainPublicClient.readContract({
    account: address,
    address: fromChain.tokenBridgeAddress,
    abi: TokenBridge.abi,
    functionName: "bridgedToNativeToken",
    args: [token[fromChain.layer]],
  })) as Address;

  let tokenAddress = token[fromChain.layer];
  let chainId = fromChain.id;
  let tokenMetadata = encodeAbiParameters(
    [
      { name: "tokenName", type: "string" },
      { name: "tokenSymbol", type: "string" },
      { name: "tokenDecimals", type: "uint8" },
    ],
    [token.name, token.symbol, token.decimals],
  );

  if (nativeToken !== zeroAddress) {
    tokenAddress = nativeToken;
    chainId = toChain.id;
    tokenMetadata = "0x";
  }

  return { tokenAddress, chainId, tokenMetadata };
}

/**
 * Generic helper to call gas estimation.
 */
async function estimateClaimMessageGasUsed(
  publicClient: ReturnType<typeof getPublicClient>,
  contractAddress: Address,
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  args: any[],
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  stateOverride: any,
  account: Address,
  value: bigint = 0n,
): Promise<bigint> {
  return publicClient.estimateContractGas({
    abi: MessageService.abi,
    functionName: "claimMessage",
    address: contractAddress,
    args,
    value,
    account,
    stateOverride,
  });
}

/**
 * Estimates the gas fee for bridging an ERC20 token.
 */
export async function estimateERC20BridgingGasUsed({
  address,
  recipient,
  amount,
  nextMessageNumber,
  fromChain,
  toChain,
  token,
  claimingType,
}: EstimationParams & { token: Token }): Promise<bigint> {
  if (claimingType === ClaimType.MANUAL) return 0n;

  const destinationChainPublicClient = getPublicClient(config, {
    chainId: toChain.id,
  });
  if (isUndefined(destinationChainPublicClient)) return 0n;

  const originChainPublicClient = getPublicClient(config, {
    chainId: fromChain.id,
  });
  if (isUndefined(originChainPublicClient)) return 0n;

  const { tokenAddress, chainId, tokenMetadata } = await prepareERC20TokenParams(
    originChainPublicClient,
    address,
    token,
    fromChain,
    toChain,
  );

  const encodedData = encodeFunctionData({
    abi: TokenBridge.abi,
    functionName: "completeBridging",
    args: [tokenAddress, amount, recipient, chainId, tokenMetadata],
  });

  const messageHash = computeMessageHash(
    fromChain.tokenBridgeAddress,
    toChain.tokenBridgeAddress,
    0n,
    0n,
    nextMessageNumber,
    encodedData,
  );

  const storageSlot = computeMessageStorageSlot(messageHash);
  const stateOverride = createStateOverride(toChain.messageServiceAddress, storageSlot);

  const argsArray = [
    fromChain.tokenBridgeAddress,
    toChain.tokenBridgeAddress,
    0n,
    0n,
    zeroAddress,
    encodedData,
    nextMessageNumber,
  ];

  return estimateClaimMessageGasUsed(
    destinationChainPublicClient,
    toChain.messageServiceAddress,
    argsArray,
    stateOverride,
    address,
  );
}

/**
 * Estimates the gas fee for bridging ETH.
 */
export async function estimateEthBridgingGasUsed({
  address,
  recipient,
  amount,
  nextMessageNumber,
  toChain,
  claimingType,
}: EstimationParams): Promise<bigint> {
  if (claimingType === ClaimType.MANUAL) return 0n;

  const destinationChainPublicClient = getPublicClient(config, {
    chainId: toChain.id,
  });
  if (isUndefined(destinationChainPublicClient)) return 0n;

  const messageHash = computeMessageHash(address, recipient, 0n, amount, nextMessageNumber, "0x");

  const storageSlot = computeMessageStorageSlot(messageHash);
  const stateOverride = createStateOverride(toChain.messageServiceAddress, storageSlot);

  const argsArray = [address, recipient, 0n, amount, zeroAddress, "0x", nextMessageNumber];

  return estimateClaimMessageGasUsed(
    destinationChainPublicClient,
    toChain.messageServiceAddress,
    argsArray,
    stateOverride,
    address,
  );
}
