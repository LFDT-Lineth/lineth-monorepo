import { BaseError, Client, Hex, SendTransactionParameters, TransactionReceipt } from "viem";
import { estimateGas, EstimateGasParameters, EstimateGasReturnType } from "viem/linea";
import { err, ok, Result } from "neverthrow";
import {
  sendRawTransaction as viemSendRawTransaction,
  waitForTransactionReceipt,
  sendTransaction as viemSendTransaction,
} from "viem/actions";

export async function estimateTransactionGas(
  client: Client,
  params: EstimateGasParameters,
): Promise<Result<EstimateGasReturnType, BaseError>> {
  try {
    const response = await estimateGas(client, params);
    return ok(response);
  } catch (error) {
    if (error instanceof BaseError) {
      const decodedError = (error.walk((err) => "data" in (err as BaseError)) || error.walk()) as BaseError;
      return err(decodedError);
    }
    return err(error as BaseError);
  }
}

export async function sendRawTransaction(
  client: Client,
  serializedTransaction: Hex,
): Promise<Result<TransactionReceipt, BaseError>> {
  try {
    const txHash = await viemSendRawTransaction(client, { serializedTransaction });
    const receipt = await waitForTransactionReceipt(client, { hash: txHash });
    return ok(receipt);
  } catch (error) {
    if (error instanceof BaseError) {
      const decodedError = (error.walk((err) => "data" in (err as BaseError)) || error.walk()) as BaseError;
      return err(decodedError as BaseError);
    }
    return err(error as BaseError);
  }
}

export async function sendTransaction(
  client: Client,
  params: SendTransactionParameters,
): Promise<Result<TransactionReceipt, BaseError>> {
  try {
    const txHash = await viemSendTransaction(client, params);
    const receipt = await waitForTransactionReceipt(client, { hash: txHash });
    return ok(receipt);
  } catch (error) {
    if (error instanceof BaseError) {
      const decodedError = (error.walk((err) => "data" in (err as BaseError)) || error.walk()) as BaseError;
      return err(decodedError as BaseError);
    }
    return err(error as BaseError);
  }
}
