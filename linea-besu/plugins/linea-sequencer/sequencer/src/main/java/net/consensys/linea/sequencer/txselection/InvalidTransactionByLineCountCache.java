/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection;

import com.google.common.annotations.VisibleForTesting;
import java.util.ConcurrentModificationException;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Optional;
import lombok.Getter;
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.datatypes.Hash;

/**
 * Cache for tracking transaction hashes that are known to exceed trace line count limits. This
 * cache is shared between transaction selection and transaction pool validation to avoid
 * reprocessing transactions that are already known to be invalid. Maps transaction hashes to the
 * name of the module that caused the overflow.
 */
@Slf4j
public class InvalidTransactionByLineCountCache {
  private final Map<Hash, String> cache = new LinkedHashMap<>();

  @Getter private final int maxSize;

  public InvalidTransactionByLineCountCache(final int maxSize) {
    this.maxSize = maxSize;
  }

  /**
   * Check if a transaction hash is in the cache.
   *
   * @param transactionHash the transaction hash to check
   * @return true if the transaction is cached as invalid
   */
  public boolean contains(final Hash transactionHash) {
    try {
      return cache.containsKey(transactionHash);
    } catch (ConcurrentModificationException e) {
      log.atTrace()
          .setMessage("ConcurrentModificationException during cache read, returning false")
          .log();
      return false;
    }
  }

  /**
   * Get the overflowing module name for a cached transaction.
   *
   * @param transactionHash the transaction hash to look up
   * @return Optional containing the module name if cached, empty otherwise
   */
  public Optional<String> getOverflowingModule(final Hash transactionHash) {
    try {
      return Optional.ofNullable(cache.get(transactionHash));
    } catch (ConcurrentModificationException e) {
      log.atTrace()
          .setMessage("ConcurrentModificationException during cache read, returning empty")
          .log();
      return Optional.empty();
    }
  }

  /**
   * Add a transaction hash to the cache with the overflowing module name, removing oldest entries
   * if necessary to maintain size limit.
   *
   * @param transactionHash the transaction hash to remember as invalid
   * @param moduleName the name of the module that caused the overflow
   */
  public void remember(final Hash transactionHash, final String moduleName) {
    try {
      internalRemember(transactionHash, moduleName);
    } catch (ConcurrentModificationException e) {
      log.atTrace()
          .setMessage("ConcurrentModificationException during cache write, retrying")
          .log();
      // Retry the operation once
      try {
        internalRemember(transactionHash, moduleName);
      } catch (ConcurrentModificationException retryException) {
        log.atDebug()
            .setMessage("Failed to add to cache after retry due to concurrent modification")
            .log();
      }
    }
  }

  private void internalRemember(final Hash transactionHash, final String moduleName)
      throws ConcurrentModificationException {
    while (cache.size() >= maxSize) {
      final var it = cache.keySet().iterator();
      if (it.hasNext()) {
        it.next();
        it.remove();
      }
    }
    cache.put(transactionHash, moduleName);
    log.atTrace()
        .setMessage("invalidTransactionByLineCountCache size={}")
        .addArgument(cache::size)
        .log();
  }

  /**
   * Get the current size of the cache.
   *
   * @return current cache size
   */
  public int size() {
    return cache.size();
  }

  /** Clear all entries from the cache. This method is primarily intended for testing. */
  @VisibleForTesting
  public void clear() {
    cache.clear();
  }
}
