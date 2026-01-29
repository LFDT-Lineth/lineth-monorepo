/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.rpc.methods;

import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.locks.Lock;
import java.util.concurrent.locks.ReentrantLock;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.sequencer.forced.ForcedTransaction;
import net.consensys.linea.sequencer.forced.ForcedTransactionPoolService;
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.parameters.JsonRpcParameter;
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.response.RpcErrorType;
import org.hyperledger.besu.ethereum.api.util.DomainObjectDecodeUtils;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.services.exception.PluginRpcEndpointException;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;
import org.hyperledger.besu.plugin.services.rpc.RpcMethodError;

/**
 * RPC method for submitting forced transactions. These transactions have highest priority in block
 * building and are guaranteed to be tried in order.
 *
 * <p>Request format:
 *
 * <pre>
 * {
 *   "method": "linea_sendForcedRawTransaction",
 *   "params": [
 *     {"forcedTransactionNumber": 6, "transaction": "0x...", "deadlineBlockNumber": "0xfce"},
 *     {"forcedTransactionNumber": 7, "transaction": "0x...", "deadlineBlockNumber": "0xfcf"}
 *   ]
 * }
 * </pre>
 *
 * <p>Response format (all successful):
 *
 * <pre>
 * {
 *   "result": [
 *     {"forcedTransactionNumber": 6, "hash": "0xTX_HASH_1"},
 *     {"forcedTransactionNumber": 7, "hash": "0xTX_HASH_2"}
 *   ]
 * }
 * </pre>
 *
 * <p>Response format (partial failure - e.g., 3rd transaction fails validation):
 *
 * <pre>
 * {
 *   "result": [
 *     {"forcedTransactionNumber": 6, "hash": "0xTX_HASH_1"},
 *     {"forcedTransactionNumber": 7, "hash": "0xTX_HASH_2"},
 *     {"forcedTransactionNumber": 8, "error": "Error message"}
 *   ]
 * }
 * </pre>
 */
@Slf4j
public class LineaSendForcedRawTransaction {
  private static final AtomicInteger LOG_SEQUENCE = new AtomicInteger();

  private final JsonRpcParameter parameterParser = new JsonRpcParameter();
  private final Lock requestLock = new ReentrantLock();
  private ForcedTransactionPoolService forcedTransactionPoolService;

  public LineaSendForcedRawTransaction init(
      final ForcedTransactionPoolService forcedTransactionPoolService) {
    this.forcedTransactionPoolService = forcedTransactionPoolService;
    return this;
  }

  public String getNamespace() {
    return "linea";
  }

  public String getName() {
    return "sendForcedRawTransaction";
  }

  public List<ForcedTransactionResponse> execute(final PluginRpcRequest request) {
    final int logId = log.isDebugEnabled() ? LOG_SEQUENCE.incrementAndGet() : -1;

    if (!requestLock.tryLock()) {
      throw new PluginRpcEndpointException(new RequestBusyError());
    }
    try {
      final ForcedTransactionParam[] params = parseRequest(logId, request.getParams());

      if (params == null || params.length == 0) {
        throw new IllegalArgumentException("At least one forced transaction is required");
      }

      validateUniqueForcedTransactionNumbers(params);

      log.atDebug()
          .setMessage("action=send_forced_raw_tx_received logId={} count={}")
          .addArgument(logId)
          .addArgument(params.length)
          .log();

      final List<ForcedTransactionResponse> responses = new ArrayList<>(params.length);
      final List<ForcedTransaction> forcedTransactions = new ArrayList<>(params.length);

      for (int i = 0; i < params.length; i++) {
        final ForcedTransactionParam param = params[i];

        if (param.forcedTransactionNumber == null) {
          responses.add(
              ForcedTransactionResponse.error(
                  0L, "forcedTransactionNumber is required at index " + i));
          break;
        }

        if (param.transaction == null || param.transaction.isEmpty()) {
          responses.add(
              ForcedTransactionResponse.error(
                  param.forcedTransactionNumber, "Empty transaction data"));
          break;
        }

        final Transaction tx;
        try {
          tx = decodeTransaction(param.transaction);
        } catch (final Exception e) {
          responses.add(
              ForcedTransactionResponse.error(
                  param.forcedTransactionNumber,
                  "Failed to decode transaction: " + e.getMessage()));
          break;
        }

        final long deadlineBlockNumber;
        try {
          deadlineBlockNumber = parseDeadlineBlockNumber(param.deadlineBlockNumber);
        } catch (final Exception e) {
          responses.add(
              ForcedTransactionResponse.error(param.forcedTransactionNumber, e.getMessage()));
          break;
        }

        forcedTransactions.add(
            new ForcedTransaction(
                param.forcedTransactionNumber, tx.getHash(), tx, deadlineBlockNumber));
        responses.add(
            ForcedTransactionResponse.success(
                param.forcedTransactionNumber, tx.getHash().toHexString()));

        log.atDebug()
            .setMessage(
                "action=parse_forced_tx logId={} forcedTxNumber={} txHash={} deadlineBlockNumber={}")
            .addArgument(logId)
            .addArgument(param.forcedTransactionNumber)
            .addArgument(tx.getHash()::toHexString)
            .addArgument(deadlineBlockNumber)
            .log();
      }

      if (!forcedTransactions.isEmpty()) {
        forcedTransactionPoolService.addForcedTransactions(forcedTransactions);
      }

      final boolean hasError = responses.stream().anyMatch(r -> r.error != null);
      if (hasError) {
        log.atWarn()
            .setMessage(
                "action=send_forced_raw_tx_partial logId={} successCount={} totalReceived={}")
            .addArgument(logId)
            .addArgument(forcedTransactions.size())
            .addArgument(params.length)
            .log();
      } else {
        log.atInfo()
            .setMessage("action=send_forced_raw_tx_success logId={} count={}")
            .addArgument(logId)
            .addArgument(responses::size)
            .log();
      }

      return responses;

    } catch (final IllegalArgumentException e) {
      log.atWarn()
          .setMessage("action=send_forced_raw_tx_invalid logId={} error={}")
          .addArgument(logId)
          .addArgument(e::getMessage)
          .log();
      throw new PluginRpcEndpointException(new SendForcedRawTransactionError(e.getMessage()));
    } catch (final Exception e) {
      log.atError()
          .setMessage("action=send_forced_raw_tx_error logId={} error={}")
          .addArgument(logId)
          .addArgument(e::getMessage)
          .setCause(e)
          .log();
      throw new PluginRpcEndpointException(
          RpcErrorType.PLUGIN_INTERNAL_ERROR, "Internal error: " + e.getMessage());
    } finally {
      requestLock.unlock();
    }
  }

  private ForcedTransactionParam[] parseRequest(final int logId, final Object[] params) {
    try {
      return parameterParser.required(params, 0, ForcedTransactionParam[].class);
    } catch (final Exception e) {
      log.atError()
          .setMessage("action=parse_request_failed logId={} error={}")
          .addArgument(logId)
          .addArgument(e::getMessage)
          .setCause(e)
          .log();
      throw new IllegalArgumentException("Malformed request parameters: " + e.getMessage());
    }
  }

  private Transaction decodeTransaction(final String rawTx) {
    return DomainObjectDecodeUtils.decodeRawTransaction(rawTx);
  }

  private long parseDeadlineBlockNumber(final String deadlineBlockNumberHex) {
    if (deadlineBlockNumberHex == null || deadlineBlockNumberHex.isEmpty()) {
      throw new IllegalArgumentException("Deadline block number is required");
    }
    try {
      return Long.decode(deadlineBlockNumberHex);
    } catch (final NumberFormatException e) {
      throw new IllegalArgumentException(
          "Invalid deadlineBlockNumber format: " + deadlineBlockNumberHex);
    }
  }

  private void validateUniqueForcedTransactionNumbers(final ForcedTransactionParam[] params) {
    final Set<Long> seenNumbers = new HashSet<>();
    for (final ForcedTransactionParam param : params) {
      if (param.forcedTransactionNumber != null
          && !seenNumbers.add(param.forcedTransactionNumber)) {
        throw new IllegalArgumentException(
            "Duplicate forcedTransactionNumber: " + param.forcedTransactionNumber);
      }
    }
  }

  /** Parameter class for forced transaction submission. */
  public static class ForcedTransactionParam {
    @JsonProperty("forcedTransactionNumber")
    public Long forcedTransactionNumber;

    @JsonProperty("transaction")
    public String transaction;

    @JsonProperty("deadlineBlockNumber")
    public String deadlineBlockNumber;

    @JsonCreator
    public ForcedTransactionParam(
        @JsonProperty("forcedTransactionNumber") final Long forcedTransactionNumber,
        @JsonProperty("transaction") final String transaction,
        @JsonProperty("deadlineBlockNumber") final String deadlineBlockNumber) {
      this.forcedTransactionNumber = forcedTransactionNumber;
      this.transaction = transaction;
      this.deadlineBlockNumber = deadlineBlockNumber;
    }
  }

  /** Response class for forced transaction submission. */
  @JsonInclude(JsonInclude.Include.NON_NULL)
  public static class ForcedTransactionResponse {
    @JsonProperty("forcedTransactionNumber")
    public final long forcedTransactionNumber;

    @JsonProperty("hash")
    public final String hash;

    @JsonProperty("error")
    public final String error;

    private ForcedTransactionResponse(
        final long forcedTransactionNumber, final String hash, final String error) {
      this.forcedTransactionNumber = forcedTransactionNumber;
      this.hash = hash;
      this.error = error;
    }

    public static ForcedTransactionResponse success(
        final long forcedTransactionNumber, final String hash) {
      return new ForcedTransactionResponse(forcedTransactionNumber, hash, null);
    }

    public static ForcedTransactionResponse error(
        final long forcedTransactionNumber, final String error) {
      return new ForcedTransactionResponse(forcedTransactionNumber, null, error);
    }
  }

  static class SendForcedRawTransactionError implements RpcMethodError {
    private final String errMessage;

    SendForcedRawTransactionError(final String errMessage) {
      this.errMessage = errMessage;
    }

    @Override
    public int getCode() {
      return INVALID_PARAMS_ERROR_CODE;
    }

    @Override
    public String getMessage() {
      return errMessage;
    }
  }

  static class RequestBusyError implements RpcMethodError {
    private static final int SERVER_BUSY_ERROR_CODE = -32000;

    @Override
    public int getCode() {
      return SERVER_BUSY_ERROR_CODE;
    }

    @Override
    public String getMessage() {
      return "Another request is already being processed";
    }
  }
}
