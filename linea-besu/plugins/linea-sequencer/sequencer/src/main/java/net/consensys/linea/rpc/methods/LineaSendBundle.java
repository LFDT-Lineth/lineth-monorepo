/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.rpc.methods;

import com.github.benmanes.caffeine.cache.Cache;
import com.github.benmanes.caffeine.cache.Caffeine;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.atomic.AtomicInteger;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bundles.BundleParameter;
import net.consensys.linea.bundles.BundlePoolService;
import net.consensys.linea.bundles.LineaLimitedBundlePool;
import net.consensys.linea.bundles.TransactionBundle;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.parameters.JsonRpcParameter;
import org.hyperledger.besu.ethereum.api.util.DomainObjectDecodeUtils;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.exception.PluginRpcEndpointException;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;
import org.hyperledger.besu.plugin.services.rpc.RpcMethodError;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;

@Slf4j
@RequiredArgsConstructor
public class LineaSendBundle {
  private static final AtomicInteger LOG_SEQUENCE = new AtomicInteger();
  private static final int MAX_TRACKED_SEEN_REQUESTS = 1_000;
  private final JsonRpcParameter parameterParser = new JsonRpcParameter();
  private final Cache<BundleParameter, Instant> recentlySeenRequestsCache =
      Caffeine.newBuilder()
          .maximumSize(MAX_TRACKED_SEEN_REQUESTS)
          .expireAfterAccess(Duration.ofMinutes(1))
          .build();
  private final BlockchainService blockchainService;
  private BundlePoolService bundlePool;
  private PluginTransactionPoolValidator txPoolValidator;

  public LineaSendBundle init(
      final BundlePoolService bundlePoolService,
      final PluginTransactionPoolValidator txPoolValidator) {
    this.bundlePool = bundlePoolService;
    this.txPoolValidator = txPoolValidator;
    return this;
  }

  public String getNamespace() {
    return "linea";
  }

  public String getName() {
    return "sendBundle";
  }

  public BundleResponse execute(final PluginRpcRequest request) {
    // sequence id for correlating error messages in logs:
    final int logId = log.isDebugEnabled() ? LOG_SEQUENCE.incrementAndGet() : -1;

    try {
      final BundleParameter bundleParams = parseRequest(logId, request.getParams());

      validateParameters(bundleParams);

      final var optBundleUUID = bundleParams.replacementUUID().map(UUID::fromString);

      // use replacement UUID hashed if present, otherwise the hash of the transactions themselves
      final var optBundleHash =
          optBundleUUID
              .map(LineaLimitedBundlePool::UUIDToHash)
              .or(
                  () ->
                      bundleParams.txs().stream()
                          .map(Bytes::fromHexString)
                          .reduce(Bytes::concatenate)
                          .map(Hash::hash));

      return optBundleHash
          .map(
              bundleHash -> {
                final List<Transaction> txs =
                    bundleParams.txs().stream()
                        .map(DomainObjectDecodeUtils::decodeRawTransaction)
                        .toList();

                validateTransactions(txs);

                bundlePool.putOrReplace(
                    bundleHash,
                    new TransactionBundle(
                        bundleHash,
                        txs,
                        bundleParams.blockNumber(),
                        bundleParams.minTimestamp(),
                        bundleParams.maxTimestamp(),
                        bundleParams.revertingTxHashes(),
                        optBundleUUID,
                        false));
                return new BundleResponse(bundleHash.getBytes().toHexString());
              })
          .orElseThrow(
              () ->
                  // otherwise boom.
                  new RuntimeException("Malformed bundle, no bundle transactions present"));

    } catch (final Exception e) {
      throw new PluginRpcEndpointException(new LineaSendBundleError(e.getMessage()));
    }
  }

  private void validateParameters(final BundleParameter bundleParams) {
    // synchronized to avoid that 2 parallel requests with the same parameters
    // will be both processed
    synchronized (recentlySeenRequestsCache) {
      final var alreadySeenAt = recentlySeenRequestsCache.getIfPresent(bundleParams);
      if (alreadySeenAt != null) {
        throw new IllegalArgumentException(
            "request already seen " + Duration.between(alreadySeenAt, Instant.now()) + " ago");
      }
      recentlySeenRequestsCache.put(bundleParams, Instant.now());
    }

    final var chainHeadBlockNumber = blockchainService.getChainHeadHeader().getNumber();
    if (bundleParams.blockNumber() <= chainHeadBlockNumber) {
      throw new IllegalArgumentException(
          "Bundle block number "
              + bundleParams.blockNumber()
              + " is not greater than current chain head block number "
              + chainHeadBlockNumber);
    }

    bundleParams
        .maxTimestamp()
        .ifPresent(
            maxTimestamp -> {
              final var now = Instant.now().getEpochSecond();
              if (maxTimestamp < now) {
                throw new IllegalArgumentException(
                    "Bundle max timestamp "
                        + maxTimestamp
                        + " is in the past, current timestamp is "
                        + now);
              }
            });
  }

  private BundleParameter parseRequest(final int logId, final Object[] params) {
    try {
      BundleParameter param = parameterParser.required(params, 0, BundleParameter.class);
      return param;
    } catch (Exception e) {
      log.atError()
          .setMessage("[{}] Failed to parse linea_sendBundle request")
          .addArgument(logId)
          .setCause(e)
          .log();
      throw new RuntimeException("Malformed linea_sendBundle json param");
    }
  }

  private void validateTransactions(final List<Transaction> txs) {
    for (final Transaction tx : txs) {
      final var maybeInvalidReason = txPoolValidator.validateTransaction(tx, true, false);
      if (maybeInvalidReason.isPresent()) {
        throw new IllegalArgumentException(
            "Invalid transaction in bundle: hash "
                + tx.getHash()
                + ", reason: "
                + maybeInvalidReason.get());
      }
    }
  }

  public record BundleResponse(String bundleHash) {}

  static class LineaSendBundleError implements RpcMethodError {

    final String errMessage;

    LineaSendBundleError(String errMessage) {
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
}
