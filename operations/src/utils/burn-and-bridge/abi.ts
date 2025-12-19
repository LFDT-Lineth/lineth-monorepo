export const BURN_AND_BRIDGE_ABI = [
  {
    inputs: [
      {
        internalType: "bytes",
        name: "_swapData",
        type: "bytes",
      },
    ],
    name: "burnAndBridge",
    outputs: [],
    stateMutability: "nonpayable",
    type: "function",
  },
] as const;

export const SWAP_ABI = [
  {
    inputs: [
      {
        internalType: "uint256",
        name: "_minLineaOut",
        type: "uint256",
      },
      {
        internalType: "uint256",
        name: "_deadline",
        type: "uint256",
      },
    ],
    name: "swap",
    outputs: [
      {
        internalType: "uint256",
        name: "amountOut",
        type: "uint256",
      },
    ],
    stateMutability: "payable",
    type: "function",
  },
] as const;

export const QUOTE_EXACT_INPUT_SINGLE_ABI = [
  {
    inputs: [
      {
        components: [
          { internalType: "address", name: "tokenIn", type: "address" },
          { internalType: "address", name: "tokenOut", type: "address" },
          { internalType: "uint256", name: "amountIn", type: "uint256" },
          { internalType: "int24", name: "tickSpacing", type: "int24" },
          {
            internalType: "uint160",
            name: "sqrtPriceLimitX96",
            type: "uint160",
          },
        ],
        internalType: "struct IQuoterV2.QuoteExactInputSingleParams",
        name: "params",
        type: "tuple",
      },
    ],
    name: "quoteExactInputSingle",
    outputs: [
      { internalType: "uint256", name: "amountOut", type: "uint256" },
      {
        internalType: "uint160",
        name: "sqrtPriceX96After",
        type: "uint160",
      },
      {
        internalType: "uint32",
        name: "initializedTicksCrossed",
        type: "uint32",
      },
      { internalType: "uint256", name: "gasEstimate", type: "uint256" },
    ],
    stateMutability: "nonpayable",
    type: "function",
  },
] as const;

export const ETH_BURNT_SWAPPED_AND_BRIDGED_EVENT_ABI = [
  {
    anonymous: false,
    inputs: [
      {
        indexed: false,
        internalType: "uint256",
        name: "ethBurnt",
        type: "uint256",
      },
      {
        indexed: false,
        internalType: "uint256",
        name: "lineaTokensBridged",
        type: "uint256",
      },
    ],
    name: "EthBurntSwappedAndBridged",
    type: "event",
  },
] as const;

export const ARREARS_PAID_EVENT_ABI = [
  {
    anonymous: false,
    inputs: [
      {
        indexed: false,
        internalType: "uint256",
        name: "amount",
        type: "uint256",
      },
      {
        indexed: false,
        internalType: "uint256",
        name: "remainingArrears",
        type: "uint256",
      },
    ],
    name: "ArrearsPaid",
    type: "event",
  },
] as const;
