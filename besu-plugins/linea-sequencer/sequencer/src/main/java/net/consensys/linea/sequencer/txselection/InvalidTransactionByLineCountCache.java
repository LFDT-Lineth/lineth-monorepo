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
import java.util.LinkedHashSet;
import java.util.Set;
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.datatypes.Hash;

/**
 * Cache for tracking transaction hashes that are known to exceed trace line count limits. This
 * cache is shared between transaction selection and transaction pool validation to avoid
 * reprocessing transactions that are already known to be invalid.
 */
@Slf4j
public class InvalidTransactionByLineCountCache {
  private final Set<Hash> cache = new LinkedHashSet<>();
  private final int maxSize;

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
      return cache.contains(transactionHash);
    } catch (ConcurrentModificationException e) {
      log.atTrace()
          .setMessage("ConcurrentModificationException during cache read, returning false")
          .log();
      return false;
    }
  }

  /**
   * Add a transaction hash to the cache, removing oldest entries if necessary to maintain size
   * limit.
   *
   * @param transactionHash the transaction hash to remember as invalid
   */
  public void remember(final Hash transactionHash) {
    try {
      internalRemember(transactionHash);
    } catch (ConcurrentModificationException e) {
      log.atTrace()
          .setMessage("ConcurrentModificationException during cache write, retrying")
          .log();
      // Retry the operation once
      try {
        internalRemember(transactionHash);
      } catch (ConcurrentModificationException retryException) {
        log.atDebug()
            .setMessage("Failed to add to cache after retry due to concurrent modification")
            .log();
      }
    }
  }

  private void internalRemember(final Hash transactionHash) throws ConcurrentModificationException {
    while (cache.size() >= maxSize) {
      final var it = cache.iterator();
      if (it.hasNext()) {
        it.next();
        it.remove();
      }
    }
    cache.add(transactionHash);
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

  /**
   * Get the maximum size of the cache.
   *
   * @return maximum cache size
   */
  public int getMaxSize() {
    return maxSize;
  }

  /** Clear all entries from the cache. This method is primarily intended for testing. */
  @VisibleForTesting
  public void clear() {
    cache.clear();
  }
}
