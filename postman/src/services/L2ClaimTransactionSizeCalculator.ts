import {
  ContractTransactionResponse,
  ErrorDescription,
  ethers,
  Overrides,
  Signer,
  Transaction,
  TransactionReceipt,
  TransactionResponse,
} from "ethers";
import { GoNativeCompressor } from "@consensys/linea-native-libs";
import { serialize } from "@consensys/linea-sdk";
import { BaseError } from "../core/errors";
import { MessageProps } from "../core/entities/Message";
import { IL2MessageServiceClient } from "../core/clients/blockchain/linea/IL2MessageServiceClient";
import { LineaGasFees } from "../core/clients/blockchain/IGasProvider";
import { IL2ClaimTransactionSizeCalculator } from "../core/services/processors/IL2ClaimTransactionSizeCalculator";

export class L2ClaimTransactionSizeCalculator implements IL2ClaimTransactionSizeCalculator {
  private compressor: GoNativeCompressor;

  /**
   * Constructs a new instance of the `L2ClaimTransactionSizeCalculator`.
   *
   * @param {IL2MessageServiceClient} l2MessageServiceClient - An instance of a class implementing the `IL2MessageServiceClient` interface, used to interact with the L2 message service.
   */
  constructor(
    private readonly l2MessageServiceClient: IL2MessageServiceClient<
      Overrides,
      TransactionReceipt,
      TransactionResponse,
      ContractTransactionResponse,
      Signer,
      ErrorDescription
    >,
  ) {
    this.compressor = new GoNativeCompressor(800_000);
  }

  /**
   * Calculates the transaction size for a given message.
   *
   * @param {MessageProps & { feeRecipient?: string }} message - The message properties including an optional fee recipient.
   * @param {LineaGasFees} fees - The transaction gas fees.
   * @returns {Promise<number>} A promise that resolves to the calculated transaction size.
   * @throws {BaseError} If there is an error during the transaction size calculation.
   */
  public async calculateTransactionSize(
    message: MessageProps & { feeRecipient?: string },
    fees: LineaGasFees,
  ): Promise<number> {
    try {
      const transactionData = this.l2MessageServiceClient.encodeClaimMessageTransactionData(message);
      const signer = this.l2MessageServiceClient.getSigner();
      const destinationAddress = this.l2MessageServiceClient.getContractAddress();
      const { gasLimit, maxFeePerGas, maxPriorityFeePerGas } = fees;

      if (!signer) {
        throw new BaseError("Signer is undefined.");
      }

      const transaction = Transaction.from({
        to: destinationAddress,
        value: 0n,
        data: transactionData,
        maxPriorityFeePerGas,
        maxFeePerGas,
        gasLimit,
        type: 2,
      });

      const signedTx = await signer.signTransaction(transaction);
      const rlpEncodedTx = ethers.encodeRlp(signedTx);
      const rlpEncodedTxInBytes = ethers.getBytes(rlpEncodedTx);

      return this.compressor.getCompressedTxSize(rlpEncodedTxInBytes);
    } catch (error) {
      throw new BaseError(`Transaction size calculation error: ${serialize(error)}`);
    }
  }
}
