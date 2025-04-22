import {
  Block,
  ContractTransactionResponse,
  ErrorDescription,
  JsonRpcProvider,
  Overrides,
  Signer,
  TransactionReceipt,
  TransactionRequest,
  TransactionResponse,
} from "ethers";
import { BaseError } from "../core/errors";
import { Message } from "../core/entities/Message";
import {
  ITransactionValidationService,
  TransactionValidationServiceConfig,
} from "../core/services/ITransactionValidationService";
import { MINIMUM_MARGIN, PROFIT_MARGIN_MULTIPLIER } from "../core/constants";
import { IL2MessageServiceClient } from "../core/clients/blockchain/linea/IL2MessageServiceClient";
import { ILineaProvider } from "../core/clients/blockchain/linea/ILineaProvider";

export class LineaTransactionValidationService implements ITransactionValidationService {
  /**
   * Constructs a new instance of the `LineaTransactionValidationService`.
   *
   * @param {TransactionValidationServiceConfig} config - Configuration settings for the transaction validation service, including profit margin and maximum gas limit.
   * @param {ILineaProvider} provider - An instance of a class implementing the `ILineaProvider` interface, used to interact with the blockchain.
   * @param {IL2MessageServiceClient} l2MessageServiceClient - An instance of a class implementing the `IL2MessageServiceClient` interface, used to interact with the L2 message service.
   */
  constructor(
    private readonly config: TransactionValidationServiceConfig,
    private readonly provider: ILineaProvider<
      TransactionReceipt,
      Block,
      TransactionRequest,
      TransactionResponse,
      JsonRpcProvider
    >,
    private readonly l2MessageServiceClient: IL2MessageServiceClient<
      Overrides,
      TransactionReceipt,
      TransactionResponse,
      ContractTransactionResponse,
      Signer,
      ErrorDescription
    >,
  ) {}

  /**
   * Evaluates a transaction to determine its feasibility based on various factors such as gas estimation, profit margin, and rate limits.
   *
   * @param {Message} message - The message object to evaluate.
   * @param {string} [feeRecipient] - The optional fee recipient address.
   * @returns {Promise<{
   *   hasZeroFee: boolean;
   *   isUnderPriced: boolean;
   *   isRateLimitExceeded: boolean;
   *   isForSponsorship: boolean;
   *   estimatedGasLimit: bigint | null;
   *   threshold: number;
   *   maxPriorityFeePerGas: bigint;
   *   maxFeePerGas: bigint;
   * }>} A promise that resolves to an object containing the evaluation results.
   */
  public async evaluateTransaction(
    message: Message,
    feeRecipient?: string,
  ): Promise<{
    hasZeroFee: boolean;
    isUnderPriced: boolean;
    isRateLimitExceeded: boolean;
    isForSponsorship: boolean;
    estimatedGasLimit: bigint | null;
    threshold: number;
    maxPriorityFeePerGas: bigint;
    maxFeePerGas: bigint;
  }> {
    const { gasLimit, maxPriorityFeePerGas, maxFeePerGas } = await this.l2MessageServiceClient.estimateClaimGasFees({
      ...message,
      feeRecipient: feeRecipient,
    });

    const threshold = this.calculateGasEstimationThreshold(message.fee, gasLimit);
    const estimatedGasLimit = this.getGasLimit(gasLimit);
    const isUnderPriced = await this.isUnderPriced(gasLimit, message.fee, message.compressedTransactionSize!);
    const hasZeroFee = this.hasZeroFee(message);
    const isRateLimitExceeded = await this.isRateLimitExceeded(message.fee, message.value);
    const isForSponsorship = this.isForSponsorship(
      gasLimit,
      this.config.isPostmanSponsorshipEnabled,
      this.config.maxPostmanSponsorGasLimit,
    );

    return {
      hasZeroFee,
      isUnderPriced,
      isRateLimitExceeded,
      isForSponsorship,
      estimatedGasLimit,
      threshold,
      maxPriorityFeePerGas,
      maxFeePerGas,
    };
  }

  /**
   * Determines if the message has zero fee.
   *
   * @param {Message} message - The message object to check.
   * @returns {boolean} `true` if the message has zero fee, `false` otherwise.
   */
  private hasZeroFee(message: Message): boolean {
    return message.hasZeroFee() && this.config.profitMargin !== 0;
  }

  /**
   * Determines if the transaction is underpriced based on the gas limit, message fee, and maximum fee per gas.
   *
   * @param {bigint} gasLimit - The gas limit for the transaction.
   * @param {bigint} messageFee - The fee associated with the message.
   * @param {bigint} maxFeePerGas - The maximum fee per gas for the transaction.
   * @returns {boolean} `true` if the transaction is underpriced, `false` otherwise.
   */
  private async isUnderPriced(
    gasLimit: bigint,
    messageFee: bigint,
    messageCompressedTransactionSize: number,
  ): Promise<boolean> {
    const extraData = await this.provider.getBlockExtraData("latest");

    if (!extraData) {
      throw new BaseError("No extra data.");
    }

    const priorityFee =
      (BigInt(MINIMUM_MARGIN * 10) *
        ((BigInt(extraData.variableCost) * BigInt(messageCompressedTransactionSize)) / gasLimit +
          BigInt(extraData.fixedCost))) /
      10n;

    const actualCost = priorityFee * gasLimit * BigInt(Math.floor(this.config.profitMargin * PROFIT_MARGIN_MULTIPLIER));

    const maxFee = messageFee * BigInt(PROFIT_MARGIN_MULTIPLIER);
    return maxFee < actualCost;
  }

  /**
   * Determines if the rate limit has been exceeded based on the message fee and value.
   *
   * @param {bigint} messageFee - The fee associated with the message.
   * @param {bigint} messageValue - The value associated with the message.
   * @returns {Promise<boolean>} A promise that resolves to `true` if the rate limit has been exceeded, `false` otherwise.
   */
  private async isRateLimitExceeded(messageFee: bigint, messageValue: bigint): Promise<boolean> {
    return this.l2MessageServiceClient.isRateLimitExceeded(messageFee, messageValue);
  }

  /**
   * Determines if the claim transaction is for sponsorship
   *
   * @param {bigint} gasLimit - The gas limit for the transaction.
   * @param {boolean} isPostmanSponsorshipEnabled - `true` if Postman sponsorship is enabled, `false` otherwise
   * @param {bigint} maxPostmanSponsorGasLimit - Maximum gas limit for sponsored Postman claim transactions
   * @returns {boolean} `true` if the message is for sponsorsing, `false` otherwise.
   */
  private isForSponsorship(
    gasLimit: bigint,
    isPostmanSponsorshipEnabled: boolean,
    maxPostmanSponsorGasLimit: bigint,
  ): boolean {
    if (!isPostmanSponsorshipEnabled) return false;
    return gasLimit < maxPostmanSponsorGasLimit;
  }

  /**
   * Calculates the gas estimation threshold based on the message fee and gas limit.
   *
   * @param {bigint} messageFee - The fee associated with the message.
   * @param {bigint} gasLimit - The gas limit for the transaction.
   * @returns {number} The calculated gas estimation threshold.
   */
  private calculateGasEstimationThreshold(messageFee: bigint, gasLimit: bigint): number {
    return parseFloat((messageFee / gasLimit).toString());
  }

  /**
   * Determines the gas limit for the transaction, ensuring it does not exceed the maximum allowed gas limit.
   *
   * @param {bigint} gasLimit - The gas limit for the transaction.
   * @returns {bigint | null} The gas limit if it is within the allowed range, `null` otherwise.
   */
  private getGasLimit(gasLimit: bigint): bigint | null {
    return gasLimit <= this.config.maxClaimGasLimit ? gasLimit : null;
  }
}
