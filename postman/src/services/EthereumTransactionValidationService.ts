import {
  ContractTransactionResponse,
  ErrorDescription,
  Overrides,
  TransactionReceipt,
  TransactionRequest,
  TransactionResponse,
} from "ethers";

import { ILineaRollupClient } from "../core/clients/blockchain/ethereum/ILineaRollupClient";
import { IEthereumGasProvider } from "../core/clients/blockchain/IGasProvider";
import { PROFIT_MARGIN_MULTIPLIER } from "../core/constants";
import { Message } from "../core/entities/Message";
import {
  ITransactionValidationService,
  TransactionValidationServiceConfig,
} from "../core/services/ITransactionValidationService";
import { IPostmanLogger } from "../utils/IPostmanLogger";

export class EthereumTransactionValidationService implements ITransactionValidationService {
  /**
   * Constructs a new instance of the `EthereumTransactionValidationService`.
   *
   * @param {ILineaRollupClient} lineaRollupClient - An instance of a class implementing the `ILineaRollupClient` interface, used to interact with the Linea Rollup client.
   * @param {IEthereumGasProvider} gasProvider - An instance of a class implementing the `IEthereumGasProvider` interface, used to fetch gas fee estimates.
   * @param {TransactionValidationServiceConfig} config - Configuration settings for the transaction validation service, including profit margin and maximum gas limit.
   */
  constructor(
    private readonly lineaRollupClient: ILineaRollupClient<
      Overrides,
      TransactionReceipt,
      TransactionResponse,
      ContractTransactionResponse,
      ErrorDescription
    >,
    private readonly gasProvider: IEthereumGasProvider<TransactionRequest>,
    private readonly config: TransactionValidationServiceConfig,
    private readonly logger: IPostmanLogger,
  ) {}

  /**
   * Evaluates a transaction to determine its feasibility based on various factors such as gas estimation, profit margin, and rate limits.
   *
   * @param {Message} message - The message object to evaluate.
   * @param {string} [feeRecipient] - The optional fee recipient address.
   * @param {string} [claimViaAddress] - The optional destination address to claim via.
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
    claimViaAddress?: string,
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
    const [gasLimit, { maxPriorityFeePerGas, maxFeePerGas }] = await Promise.all([
      this.lineaRollupClient.estimateClaimGas(
        {
          ...message,
          feeRecipient,
          messageBlockNumber: message.sentBlockNumber,
        },
        { claimViaAddress },
      ),
      this.gasProvider.getGasFees(),
    ]);

    this.logger.debug(
      `Estimated gas fees for message claiming. messageHash=${message.messageHash} gasLimit=${gasLimit} maxPriorityFeePerGas=${maxPriorityFeePerGas} maxFeePerGas=${maxFeePerGas}`,
    );

    const threshold = this.calculateGasEstimationThreshold(message.fee, gasLimit);
    const estimatedGasLimit = this.getGasLimit(gasLimit);
    const isUnderPriced = this.isUnderPriced(gasLimit, message.fee, maxFeePerGas);
    const hasZeroFee = this.hasZeroFee(message);
    const isRateLimitExceeded = await this.isRateLimitExceeded(message.fee, message.value);
    const isForSponsorship = this.isForSponsorship(gasLimit, hasZeroFee, isUnderPriced);

    this.logger.debug(
      `Transaction evaluation results. messageHash=${message.messageHash} hasZeroFee=${hasZeroFee} isUnderPriced=${isUnderPriced} isRateLimitExceeded=${isRateLimitExceeded} isForSponsorship=${isForSponsorship} estimatedGasLimit=${estimatedGasLimit} threshold=${threshold}`,
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
   * Determines if the transaction is underpriced based on the gas limit, message fee, and maximum fee per gas.
   *
   * @param {bigint} gasLimit - The gas limit for the transaction.
   * @param {bigint} messageFee - The fee associated with the message.
   * @param {bigint} maxFeePerGas - The maximum fee per gas for the transaction.
   * @returns {boolean} `true` if the transaction is underpriced, `false` otherwise.
   */
  private isUnderPriced(gasLimit: bigint, messageFee: bigint, maxFeePerGas: bigint): boolean {
    const actualCost =
      gasLimit * maxFeePerGas * BigInt(Math.floor(this.config.profitMargin * PROFIT_MARGIN_MULTIPLIER));
    const maxFee = messageFee * BigInt(PROFIT_MARGIN_MULTIPLIER);
    return actualCost > maxFee;
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

  /**
   * Determines if the rate limit has been exceeded based on the message fee and value.
   *
   * @param {bigint} messageFee - The fee associated with the message.
   * @param {bigint} messageValue - The value associated with the message.
   * @returns {Promise<boolean>} A promise that resolves to `true` if the rate limit has been exceeded, `false` otherwise.
   */
  private async isRateLimitExceeded(messageFee: bigint, messageValue: bigint): Promise<boolean> {
    return this.lineaRollupClient.isRateLimitExceeded(messageFee, messageValue);
  }

  /**
   * Determines if the claim transaction is for sponsorship
   *
   * @param {bigint} gasLimit - The gas limit for the transaction.
   * @param {boolean} hasZeroFee - `true` if the message has zero fee, `false` otherwise.
   * @param {boolean} isUnderPriced - `true` if the transaction is underpriced, `false` otherwise.
   * @returns {boolean} `true` if the message is for sponsoring, `false` otherwise.
   */
  private isForSponsorship(gasLimit: bigint, hasZeroFee: boolean, isUnderPriced: boolean): boolean {
    if (!this.config.isPostmanSponsorshipEnabled) return false;
    if (gasLimit > this.config.maxPostmanSponsorGasLimit) return false;
    if (hasZeroFee) return true;
    if (isUnderPriced) return true;
    // The message would be claimed regardless of sponsorship settings
    return false;
  }
}
