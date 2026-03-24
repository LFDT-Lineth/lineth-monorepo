/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.utils;

import kotlin.time.Instant;
import org.hyperledger.besu.datatypes.Transaction;

/**
 * Interface for compressing transactions and caching the results. Implementations should provide
 * efficient compression of transaction data, potentially with caching to avoid redundant
 * compression operations.
 */
public interface TransactionCompressor {
  /**
   * Get the compressed size of a transaction. Implementations may cache the result based on the
   * transaction hash to improve performance.
   *
   * @param transaction the transaction for which to get the compressed size
   * @param blockTimestamp the timestamp of the block which is expected to include this transaction
   * @return the compressed size of the transaction
   */
  int getCompressedSize(Transaction transaction, Instant blockTimestamp);

  default int getCompressedSize(Transaction transaction) {
    return getCompressedSize(
        transaction, Instant.Companion.fromEpochMilliseconds(System.currentTimeMillis()));
  }
}
