import { err, ok, Result } from "neverthrow";
import { Address, BaseError, Client, encodeFunctionData, Hex } from "viem";
import { readContract, simulateContract } from "viem/actions";
import { BURN_AND_BRIDGE_ABI, QUOTE_EXACT_INPUT_SINGLE_ABI, SWAP_ABI } from "./abi.js";

export async function getInvoiceArrears(client: Client, contractAddress: Address): Promise<Result<bigint, BaseError>> {
  try {
    const invoiceArrears = await readContract(client, {
      address: contractAddress,
      abi: [
        {
          inputs: [],
          name: "invoiceArrears",
          outputs: [
            {
              internalType: "uint256",
              name: "",
              type: "uint256",
            },
          ],
          stateMutability: "view",
          type: "function",
        },
      ],
      functionName: "invoiceArrears",
    });
    return ok(invoiceArrears);
  } catch (error) {
    if (error instanceof BaseError) {
      const decodedError = error.walk();
      return err(decodedError as BaseError);
    }
    return err(error as BaseError);
  }
}

export async function getMinimumFee(client: Client, contractAddress: Address): Promise<Result<bigint, BaseError>> {
  try {
    const minimumFeeInWei = await readContract(client, {
      address: contractAddress,
      abi: [
        {
          inputs: [],
          name: "minimumFeeInWei",
          outputs: [
            {
              internalType: "uint256",
              name: "",
              type: "uint256",
            },
          ],
          stateMutability: "view",
          type: "function",
        },
      ],
      functionName: "minimumFeeInWei",
    });
    return ok(minimumFeeInWei);
  } catch (error) {
    if (error instanceof BaseError) {
      const decodedError = error.walk();
      return err(decodedError as BaseError);
    }
    return err(error as BaseError);
  }
}

export async function getQuote(
  client: Client,
  contractAddress: Address,
  params: {
    tokenIn: Address;
    tokenOut: Address;
    amountIn: bigint;
    tickSpacing: number;
    sqrtPriceLimitX96: bigint;
  },
): Promise<Result<readonly [bigint, bigint, number, bigint], BaseError>> {
  try {
    const { tokenIn, tokenOut, amountIn, tickSpacing, sqrtPriceLimitX96 } = params;
    const { result } = await simulateContract(client, {
      address: contractAddress,
      abi: QUOTE_EXACT_INPUT_SINGLE_ABI,
      functionName: "quoteExactInputSingle",
      args: [
        {
          tokenIn,
          tokenOut,
          amountIn,
          tickSpacing,
          sqrtPriceLimitX96,
        },
      ],
    });
    return ok(result);
  } catch (error) {
    if (error instanceof BaseError) {
      const decodedError = error.walk();
      return err(decodedError as BaseError);
    }
    return err(error as BaseError);
  }
}

export const computeSwapCalldata = (minLineaOut: bigint, deadline: bigint): Hex => {
  return encodeFunctionData({
    abi: SWAP_ABI,
    functionName: "swap",
    args: [minLineaOut, deadline],
  });
};

export function computeBurnAndBridgeCalldata(swapData: Hex): Hex {
  return encodeFunctionData({
    abi: BURN_AND_BRIDGE_ABI,
    functionName: "burnAndBridge",
    args: [swapData],
  });
}
