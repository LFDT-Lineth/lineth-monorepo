import { err, ok, Result } from "neverthrow";
import { Address, BaseError, Client, encodeFunctionData, Hex } from "viem";
import { readContract } from "viem/actions";

import { SUBMIT_INVOICE_ABI } from "./abi.js";

export async function getLastInvoiceDate(client: Client, contractAddress: Address): Promise<Result<bigint, BaseError>> {
  try {
    const lastInvoiceDate = await readContract(client, {
      address: contractAddress,
      abi: [
        {
          inputs: [],
          name: "lastInvoiceDate",
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
      functionName: "lastInvoiceDate",
    });
    return ok(lastInvoiceDate);
  } catch (error) {
    if (error instanceof BaseError) {
      const decodedError = error.walk();
      return err(decodedError as BaseError);
    }
    return err(error as BaseError);
  }
}

export function computeSubmitInvoiceCalldata(startDate: bigint, endDate: bigint, invoiceAmount: bigint): Hex {
  return encodeFunctionData({
    abi: SUBMIT_INVOICE_ABI,
    functionName: "submitInvoice",
    args: [startDate, endDate, invoiceAmount],
  });
}
