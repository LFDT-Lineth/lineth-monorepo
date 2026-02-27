/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.utils;

import com.google.common.cache.Cache;
import com.google.common.cache.CacheBuilder;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import linea.blob.BlobCompressor;
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Transaction;

/**
 * A caching wrapper around the transaction compressor that caches compressed sizes based on
 * transaction hash. This allows compression results to be reused across different parts of the
 * system, improving performance by avoiding redundant compression operations.
 */
@Slf4j
public class CachingTransactionCompressor implements TransactionCompressor {
  private static final long DEFAULT_CACHE_SIZE = 10000;
  private final BlobCompressor blobCompressor;
  private final Cache<Hash, Integer> compressedSizeCache;

  public CachingTransactionCompressor(final long cacheSize, final BlobCompressor blobCompressor) {
    this.blobCompressor = blobCompressor;
    compressedSizeCache =
        CacheBuilder.newBuilder()
            .maximumSize(cacheSize)
            .expireAfterAccess(30, TimeUnit.MINUTES)
            .build();
  }

  public CachingTransactionCompressor(final BlobCompressor blobCompressor) {
    this(DEFAULT_CACHE_SIZE, blobCompressor);
  }

  private int calculateCompressedSize(final Transaction transaction) {
    final byte[] encoded = TxEncodingUtils.encodeForCompressor(transaction);
    return blobCompressor.compressedSize(encoded);
  }

  /**
   * Get the compressed size of a transaction. If the compressed size has been calculated before for
   * this transaction (identified by its hash), it will be retrieved from the cache. Otherwise, it
   * will be calculated and cached for future use.
   *
   * @param transaction the transaction for which to get the compressed size
   * @return the compressed size of the transaction
   */
  @Override
  public int getCompressedSize(final Transaction transaction) {
    try {
      return compressedSizeCache.get(
          transaction.getHash(), () -> calculateCompressedSize(transaction));
    } catch (ExecutionException e) {
      log.atWarn()
          .setMessage(
              "Failed to calculate compressed size for transaction {}, calculating directly")
          .addArgument(transaction::getHash)
          .setCause(e)
          .log();
      return calculateCompressedSize(transaction);
    }
  }
}
