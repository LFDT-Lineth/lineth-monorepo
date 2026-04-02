import { GoNativeCompressor } from "@consensys/linea-native-libs";

import { LineaGasFees } from "../../core/clients/blockchain/IGasProvider";
import { IL2MessageServiceClient } from "../../core/clients/blockchain/linea/IL2MessageServiceClient";
import { MessageProps } from "../../core/entities/Message";
import { ITransactionSigner } from "../../core/services/ITransactionSigner";
import { IL2ClaimTransactionSizeCalculator } from "../../core/services/processors/IL2ClaimTransactionSizeCalculator";
import { Address } from "../../core/types";

export class L2ClaimTransactionSizeCalculator implements IL2ClaimTransactionSizeCalculator {
  private compressor: GoNativeCompressor;

  /**
   * Constructs a new instance of the `L2ClaimTransactionSizeCalculator`.
   *
   * @param {IL2MessageServiceClient} l2MessageServiceClient - Used to encode the claim calldata and retrieve the contract address.
   * @param {ITransactionSigner} transactionSigner - Used to sign and serialize a dummy transaction for size measurement.
   */
  constructor(
    private readonly l2MessageServiceClient: IL2MessageServiceClient,
    private readonly transactionSigner: ITransactionSigner,
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
    message: MessageProps & { feeRecipient?: Address },
    fees: LineaGasFees,
  ): Promise<number> {
    const transactionData = this.l2MessageServiceClient.encodeClaimMessageTransactionData(message);
    const destinationAddress = this.l2MessageServiceClient.getContractAddress();

    const rlpEncodedTxInBytes = await this.transactionSigner.signAndSerialize(
      { to: destinationAddress, value: 0n, data: transactionData },
      fees,
    );

    return this.compressor.getCompressedTxSize(rlpEncodedTxInBytes);
  }
}
