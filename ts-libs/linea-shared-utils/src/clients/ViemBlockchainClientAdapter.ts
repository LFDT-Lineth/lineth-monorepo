import {
  Hex,
  createPublicClient,
  http,
  PublicClient,
  TransactionReceipt,
  Address,
  TransactionSerializableEIP1559,
  serializeTransaction,
  parseSignature,
  Chain,
  withTimeout,
  TimeoutError,
  BaseError,
  EstimateGasExecutionError,
  RawContractError,
  ContractFunctionRevertedError,
  RpcRequestError,
  Abi,
  decodeErrorResult,
} from "viem";
import { sendRawTransaction, waitForTransactionReceipt } from "viem/actions";

import { IBlockchainClient } from "../core/client/IBlockchainClient";
import { IContractSignerClient } from "../core/client/IContractSignerClient";
import { MAX_BPS } from "../core/constants/maths";
import { IEstimateGasErrorReporter } from "../core/services/IEstimateGasErrorReporter";
import { ILogger } from "../logging/ILogger";

/**
 * Adapter that wraps viem's PublicClient to provide blockchain interaction functionality.
 * Implements transaction sending with retry logic, gas fee estimation, and connection pooling.
 * Uses a single PublicClient instance for better connection pooling and memory efficiency.
 */
export class ViemBlockchainClientAdapter implements IBlockchainClient<PublicClient, TransactionReceipt> {
  blockchainClient: PublicClient;

  /**
   * Creates a new ViemBlockchainClientAdapter instance.
   *
   * @param {ILogger} logger - The logger instance for logging blockchain operations.
   * @param {string} rpcUrl - The RPC URL for the blockchain network.
   * @param {Chain} chain - The blockchain chain configuration.
   * @param {IContractSignerClient} contractSignerClient - The client for signing transactions.
   * @param {IEstimateGasErrorReporter} [errorReporter] - Optional error reporter for tracking estimateGas errors via metrics.
   * @param {number} [sendTransactionsMaxRetries=3] - Maximum number of retry attempts for sending transactions (must be at least 1).
   * @param {bigint} [gasRetryBumpBps=1000n] - Gas price bump in basis points per retry (e.g., 1000n = +10% per retry).
   * @param {number} [sendTransactionAttemptTimeoutMs=300000] - Timeout in milliseconds for each transaction attempt (default: 5 minutes).
   * @param {bigint} [gasLimitBufferBps=1500n] - Gas limit buffer in basis points applied to estimated gas (e.g., 1500n = +15% buffer). This prevents transactions from being included in blocks but failing to complete execution due to running out of gas, which can leave contract state partially updated.
   * @throws {Error} If sendTransactionsMaxRetries is less than 1.
   */
  constructor(
    private readonly logger: ILogger,
    rpcUrl: string,
    chain: Chain,
    private readonly contractSignerClient: IContractSignerClient,
    private readonly errorReporter?: IEstimateGasErrorReporter,
    private readonly sendTransactionsMaxRetries = 3,
    private readonly gasRetryBumpBps: bigint = 1000n, // +10% per retry
    private readonly sendTransactionAttemptTimeoutMs = 300_000, // 5m
    private readonly gasLimitBufferBps: bigint = 1500n, // +15% buffer to prevent partial execution
  ) {
    if (sendTransactionsMaxRetries < 1) {
      throw new Error("sendTransactionsMaxRetries must be at least 1");
    }
    // Aim re-use single blockchain client for
    // i.) Better connection pooling
    // ii.) Memory efficient
    // iii.) Single point of configuration
    this.blockchainClient = createPublicClient({
      chain: {
        ...chain,
        id: chain.id, // Explicitly set chainId to prevent redundant eth_chainId validation calls
      },
      transport: http(rpcUrl, {
        batch: true,
        // TODO - How does this interact with our custom retry logic in sendSignedTransaction?
        // Hypothesis - Default Viem timeout of 10s will kick in first. It should still retry because we are using the native Viem Timeout error.
        retryCount: 3,
        onFetchRequest: async (request) => {
          const cloned = request.clone(); // clone before reading body
          try {
            const bodyText = await cloned.text();
            this.logger.debug("onFetchRequest", {
              method: request.method,
              url: request.url,
              body: bodyText,
            });
          } catch (err) {
            this.logger.warn("Failed to read request body", { err });
          }
        },
        onFetchResponse: async (resp) => {
          const cloned = resp.clone(); // clone before reading body
          try {
            const bodyText = await cloned.text();
            this.logger.debug("onFetchResponse", {
              status: resp.status,
              statusText: resp.statusText,
              body: bodyText,
            });
          } catch (err) {
            this.logger.warn("Failed to read response body", { err });
          }
        },
      }),
      batch: {
        // Not sure if this will help or not, need to experiment in testnet
        multicall: true,
      },
    });
  }

  /**
   * Gets the underlying viem PublicClient instance.
   *
   * @returns {PublicClient} The viem PublicClient instance used for blockchain interactions.
   */
  getBlockchainClient(): PublicClient {
    return this.blockchainClient;
  }

  /**
   * Gets the Ethereum address of the signer associated with this blockchain client.
   *
   * @returns {Address} The Ethereum address of the signer.
   */
  getSignerAddress(): Address {
    return this.contractSignerClient.getAddress();
  }

  /**
   * Gets the chain ID of the connected blockchain network.
   *
   * @returns {Promise<number>} The chain ID of the blockchain network.
   */
  async getChainId(): Promise<number> {
    return await this.blockchainClient.getChainId();
  }

  /**
   * Gets the balance of an Ethereum address in wei.
   *
   * @param {Address} address - The Ethereum address to query the balance for.
   * @returns {Promise<bigint>} The balance in wei.
   */
  async getBalance(address: Address): Promise<bigint> {
    return await this.blockchainClient.getBalance({
      address,
    });
  }

  /**
   * Estimates the current gas fees for EIP-1559 transactions.
   *
   * @returns {Promise<{ maxFeePerGas: bigint; maxPriorityFeePerGas: bigint }>} An object containing the estimated maxFeePerGas and maxPriorityFeePerGas values.
   */
  async estimateGasFees(): Promise<{ maxFeePerGas: bigint; maxPriorityFeePerGas: bigint }> {
    const { maxFeePerGas, maxPriorityFeePerGas } = await this.blockchainClient.estimateFeesPerGas();
    return { maxFeePerGas, maxPriorityFeePerGas };
  }

  /**
   * Gets the transaction receipt for a given transaction hash.
   *
   * @param {Hex} txHash - The transaction hash to query.
   * @returns {Promise<TransactionReceipt | undefined>} The transaction receipt if found, undefined otherwise.
   */
  async getTxReceipt(txHash: Hex): Promise<TransactionReceipt | undefined> {
    try {
      return await this.blockchainClient.getTransactionReceipt({ hash: txHash });
    } catch (error) {
      this.logger.warn("getTxReceipt - failed to get transaction receipt", { txHash, error });
      return undefined;
    }
  }

  /**
   * Attempts to send a signed transaction with retry-on-timeout semantics.
   * On each retry, bumps gas fees by `gasRetryBumpBps` basis points.
   * Uses a single nonce for all retry attempts to prevent nonce conflicts.
   * Does not retry on contract revert errors or non-timeout errors, only on timeout.
   *
   * @param {Address} contractAddress - The address of the contract to interact with.
   * @param {Hex} calldata - The encoded function call data.
   * @param {bigint} [value=0n] - The amount of ether to send with the transaction (default: 0).
   * @param {Abi} [abi] - Optional ABI for error decoding in estimateGas failures.
   * @returns {Promise<TransactionReceipt>} The transaction receipt if successful.
   * @throws {ContractFunctionRevertedError} If the contract call reverts (not retried).
   * @throws {Error} If retry attempts are exhausted or a non-timeout error occurs.
   */
  async sendSignedTransaction(
    contractAddress: Address,
    calldata: Hex,
    value: bigint = 0n,
    abi?: Abi,
  ): Promise<TransactionReceipt> {
    this.logger.debug("sendSignedTransaction started");
    let gasMultiplierBps = MAX_BPS; // Start at 100%
    let maxFeePerGas = 0n;
    let maxPriorityFeePerGas = 0n;
    let lastError: unknown;

    // Use a single nonce for all retries.
    const [nonce, gasLimit, chainId] = await Promise.all([
      this.blockchainClient.getTransactionCount({ address: this.contractSignerClient.getAddress() }),
      this._estimateGasWithErrorHandling(contractAddress, calldata, value, abi),
      this.getChainId(),
    ]);

    for (let attempt = 1; attempt <= this.sendTransactionsMaxRetries; attempt += 1) {
      // Try to send tx with a timeout
      try {
        // Get new fee estimate each time
        const { maxFeePerGas: maxFeePerGasCandidate, maxPriorityFeePerGas: maxPriorityFeePerGasCandidate } =
          await this.estimateGasFees();

        // Use the highest fee estimate retrieved
        maxFeePerGas = maxFeePerGasCandidate > maxFeePerGas ? maxFeePerGasCandidate : maxFeePerGas;
        maxPriorityFeePerGas =
          maxPriorityFeePerGasCandidate > maxPriorityFeePerGas ? maxPriorityFeePerGasCandidate : maxPriorityFeePerGas;

        const receipt = await withTimeout(
          () =>
            this._sendSignedEIP1559Transaction(
              contractAddress,
              calldata,
              value,
              nonce,
              gasLimit,
              chainId,
              maxFeePerGas,
              maxPriorityFeePerGas,
              gasMultiplierBps,
            ),
          {
            timeout: this.sendTransactionAttemptTimeoutMs,
            signal: false, // don’t try to abort, just reject
            errorInstance: new TimeoutError({
              body: { message: "sendSignedTransaction attempt timed out" },
              url: "local:sendSignedTransaction",
            }),
          },
        );
        this.logger.debug(`sendSignedTransaction succeeded`, { receipt });
        return receipt;
      } catch (error) {
        if (error instanceof BaseError && !this._shouldRetryViemSendRawTransactionError(error)) {
          const decodedError = error.walk();
          this.logger.error("sendSignedTransaction failed and will not be retried", { decodedError });
          throw decodedError;
        }
        if (attempt >= this.sendTransactionsMaxRetries) {
          this.logger.error(
            `sendSignedTransaction retry attempts exhausted sendTransactionsMaxRetries=${this.sendTransactionsMaxRetries}`,
            { error },
          );
          throw error;
        }

        this.logger.warn(
          `sendSignedTransaction retry attempt failed attempt=${attempt} sendTransactionsMaxRetries=${this.sendTransactionsMaxRetries}`,
          { error },
        );
        lastError = error;
        // Compound gas for next retry
        gasMultiplierBps = (gasMultiplierBps * (MAX_BPS + this.gasRetryBumpBps)) / MAX_BPS;
      }
    }

    // Unreachable but required to simplify TypeScript return type
    throw lastError;
  }

  /**
   * Internal method that sends a signed EIP-1559 transaction to the blockchain.
   * Signs the transaction using the contract signer client, serializes it, and broadcasts it.
   * Waits for the transaction receipt before returning.
   *
   * @param {Address} contractAddress - The address of the contract to interact with.
   * @param {Hex} calldata - The encoded function call data.
   * @param {bigint} value - The amount of ether to send with the transaction.
   * @param {number} nonce - The transaction nonce.
   * @param {bigint} gasLimit - The estimated gas limit for the transaction.
   * @param {number} chainId - The chain ID of the blockchain network.
   * @param {bigint} maxFeePerGas - The maximum fee per gas (EIP-1559).
   * @param {bigint} maxPriorityFeePerGas - The maximum priority fee per gas (EIP-1559).
   * @param {bigint} gasMultiplierBps - Gas multiplier in basis points to apply to gas values.
   * @returns {Promise<TransactionReceipt>} The transaction receipt after the transaction is mined.
   */
  private async _sendSignedEIP1559Transaction(
    contractAddress: Address,
    calldata: Hex,
    value: bigint,
    nonce: number,
    gasLimit: bigint,
    chainId: number,
    maxFeePerGas: bigint,
    maxPriorityFeePerGas: bigint,
    gasMultiplierBps: bigint,
  ): Promise<TransactionReceipt> {
    const tx: TransactionSerializableEIP1559 = {
      to: contractAddress,
      type: "eip1559",
      data: calldata,
      chainId: chainId,
      gas: (gasLimit * gasMultiplierBps) / MAX_BPS,
      maxFeePerGas: (maxFeePerGas * gasMultiplierBps) / MAX_BPS,
      maxPriorityFeePerGas: (maxPriorityFeePerGas * gasMultiplierBps) / MAX_BPS,
      nonce,
      value,
    };
    this.logger.debug("_sendSignedTransaction tx for signing", { tx });
    const signature = await this.contractSignerClient.sign(tx);
    const serializedTransaction = serializeTransaction(tx, parseSignature(signature));

    this.logger.debug(
      `_sendSignedTransaction - sending raw transaction serializedTransaction=${serializedTransaction}`,
    );
    const txHash = await sendRawTransaction(this.blockchainClient, { serializedTransaction });
    const receipt = await waitForTransactionReceipt(this.blockchainClient, { hash: txHash });
    return receipt;
  }

  /**
   * Wrapper for estimateGas that handles error parsing and logging.
   * Extracts enhanced error details from EstimateGasExecutionError and logs them before re-throwing.
   * Applies a configurable gas limit buffer to prevent partial execution due to running out of gas.
   *
   * @param {Address} contractAddress - The address of the contract to interact with.
   * @param {Hex} calldata - The encoded function call data.
   * @param {bigint} value - The amount of ether to send with the transaction.
   * @param {Abi} [abi] - Optional ABI for error decoding.
   * @returns {Promise<bigint>} The estimated gas limit with buffer applied.
   * @throws {Error} Re-throws the original error after logging enhanced details.
   */
  private async _estimateGasWithErrorHandling(
    contractAddress: Address,
    calldata: Hex,
    value: bigint,
    abi?: Abi,
  ): Promise<bigint> {
    try {
      const estimateGasParams: {
        account: Address;
        to: Address;
        data: Hex;
        value: bigint;
        abi?: Abi;
      } = {
        account: this.contractSignerClient.getAddress(),
        to: contractAddress,
        data: calldata,
        value,
      };
      if (abi) {
        estimateGasParams.abi = abi;
      }
      const estimatedGas = await this.blockchainClient.estimateGas(estimateGasParams);
      const bufferedGas = (estimatedGas * (MAX_BPS + this.gasLimitBufferBps)) / MAX_BPS;
      this.logger.debug("Gas estimation with buffer applied", {
        originalEstimatedGas: estimatedGas.toString(),
        bufferedGas: bufferedGas.toString(),
        gasLimitBufferBps: this.gasLimitBufferBps.toString(),
        contractAddress,
      });
      return bufferedGas;
    } catch (error) {
      const parsedError = this._parseEstimateGasError(error, abi);
      if (parsedError) {
        this.logger.error("estimateGas failed with enhanced error details", {
          errorType: parsedError.errorType,
          rawRevertData: parsedError.rawRevertData,
          decodedError: parsedError.decodedError,
          rpcErrorData: parsedError.rpcErrorData,
          originalMessage: parsedError.originalMessage,
          contractAddress,
          calldata,
          value: value.toString(),
        });

        // Report error for metrics tracking if reporter is available
        if (this.errorReporter && parsedError.rawRevertData) {
          const errorName = parsedError.decodedError?.errorName;
          this.errorReporter.recordContractError(contractAddress, parsedError.rawRevertData, errorName);
        }
      } else {
        this.logger.error("estimateGas failed", {
          error,
          contractAddress,
          calldata,
          value: value.toString(),
        });
      }
      throw error;
    }
  }

  /**
   * Parses an estimateGas error to extract revert data, decoded error information, and RPC error data.
   * Uses error.walk() to traverse the error chain and extract information from various error types.
   * If automatic decoding fails but raw revert data and ABI are available, attempts manual decoding.
   *
   * @param {unknown} error - The error to parse.
   * @param {Abi} [abi] - Optional ABI for manual error decoding when automatic decoding fails.
   * @returns {object | undefined} An object containing parsed error information, or undefined if the error is not an EstimateGasExecutionError.
   */
  private _parseEstimateGasError(
    error: unknown,
    abi?: Abi,
  ):
    | {
        errorType: string;
        rawRevertData?: Hex;
        decodedError?: {
          raw: Hex;
          errorName?: string;
          args?: unknown;
          reason?: string;
        };
        rpcErrorData?: Hex | unknown;
        originalMessage: string;
      }
    | undefined {
    if (!(error instanceof EstimateGasExecutionError)) {
      return undefined;
    }

    const errorType = error.name || "EstimateGasExecutionError";
    const originalMessage = error.message;

    // Extract raw revert data from RawContractError
    const rawError = error.walk() as RawContractError | null;
    let rawRevertData: Hex | undefined;
    if (rawError) {
      rawRevertData =
        typeof rawError.data === "object" && rawError.data !== null && "data" in rawError.data
          ? (rawError.data as { data: Hex }).data
          : typeof rawError.data === "string"
            ? (rawError.data as Hex)
            : undefined;
    }

    // Extract decoded error information from ContractFunctionRevertedError
    const contractError = error.walk(
      (e) => e instanceof ContractFunctionRevertedError,
    ) as ContractFunctionRevertedError | null;
    let decodedError:
      | {
          raw: Hex;
          errorName?: string;
          args?: unknown;
          reason?: string;
        }
      | undefined;
    if (contractError && contractError.raw) {
      decodedError = {
        raw: contractError.raw,
        errorName: contractError.data?.errorName,
        args: contractError.data?.args,
        reason: contractError.reason,
      };
    }

    // If error wasn't decoded but we have ABI and raw data, try manual decoding
    if (!decodedError?.errorName && rawRevertData && abi) {
      try {
        const decoded = decodeErrorResult({
          abi,
          data: rawRevertData,
        });
        decodedError = {
          raw: rawRevertData,
          errorName: decoded.errorName,
          args: decoded.args,
          reason: undefined,
        };
      } catch (decodeErr) {
        // Manual decoding failed, keep decodedError as undefined or existing value
      }
    }

    // Extract RPC error data from RpcRequestError
    const rpcError = error.walk((e) => e instanceof RpcRequestError) as RpcRequestError | null;
    let rpcErrorData: Hex | unknown | undefined;
    if (rpcError) {
      rpcErrorData =
        typeof rpcError.data === "object" && rpcError.data !== null && "data" in rpcError.data
          ? (rpcError.data as { data: Hex }).data
          : rpcError.data;
    }

    return {
      errorType,
      rawRevertData,
      decodedError,
      rpcErrorData,
      originalMessage,
    };
  }

  /**
   * Determines whether a viem sendRawTransaction error should be retried.
   * In general we retry server-side errors, and exclude client-side errors from retries.
   *
   * Uses error.walk() to traverse the error chain and find errors with specific properties
   * (code, status) or error types. This is more robust than checking only the root error.
   *
   * @param {BaseError} error - The error to evaluate for retry eligibility.
   * @returns {boolean} True if the error should be retried, false otherwise.
   */
  private _shouldRetryViemSendRawTransactionError(error: BaseError): boolean {
    // We don't want to retry our own timeout
    // But Viem internal retry should be ok - TODO test for conflict here
    if (error instanceof TimeoutError) {
      return false;
    }

    // Check RPC error codes using walk() to find error with code property in the error chain
    const errorWithCode = error.walk((err) => typeof (err as { code?: unknown }).code === "number");
    if (errorWithCode) {
      const code = (errorWithCode as unknown as { code: number }).code;

      // Explicitly retry these RPC error codes
      // -32603 InternalRpcError
      // -32005 LimitExceededRpcError
      // -32002 ResourceUnavailableRpcError
      // -1 Unknown error
      // 4900 ProviderDisconnectedError
      // 4901 ChainDisconnectedError
      if (code === -32603 || code === -32005 || code === -32002 || code === -1 || code === 4900 || code === 4901) {
        return true;
      }

      // Explicitly do NOT retry these RPC error codes
      if (
        code === -32700 || // ParseRpcError
        code === -32600 || // InvalidRequestRpcError
        code === -32601 || // MethodNotFoundRpcError
        code === -32602 || // InvalidParamsRpcError
        code === -32000 || // InvalidInputRpcError
        code === -32001 || // ResourceNotFoundRpcError
        code === -32003 || // TransactionRejectedRpcError
        code === -32004 || // MethodNotSupportedRpcError
        code === -32006 || // JsonRpcVersionUnsupportedError
        code === -32015 || // VM execution error (Solidity revert)
        code === 4001 || // UserRejectedRequestError
        code === 5000 || // UserRejectedRequestError (CAIP-25)
        code === 4100 || // UnauthorizedProviderError
        code === 4200 || // UnsupportedProviderMethodError
        code === 4902 || // SwitchChainError
        (code >= 5700 && code <= 5760) // Various capability errors (5700-5760)
      ) {
        return false;
      }

      // For errors with codes not in explicit retry/non-retry lists above, default to retry
      // This follows the guide's default behavior: retry unknown errors
      return true;
    }

    // Check HTTP status codes using walk() to find error with status property in the error chain
    const errorWithStatus = error.walk((err) => typeof (err as { status?: unknown }).status === "number");
    if (errorWithStatus) {
      const status = (errorWithStatus as unknown as { status: number }).status;
      // Retry on these HTTP status codes
      if ([408, 429, 500, 502, 503, 504].includes(status)) {
        return true;
      }
      // Do not retry on other HTTP status codes
      return false;
    }

    // Check specific error types using walk() to find errors by name in the error chain
    // Retry WebSocketRequestError and UnknownRpcError
    if (
      error.walk((err) => (err as { name?: string }).name === "WebSocketRequestError") ||
      error.walk((err) => (err as { name?: string }).name === "UnknownRpcError")
    ) {
      return true;
    }

    // Default behavior - retry unknown errors
    return true;
  }
}
