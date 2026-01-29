/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.forced;

/** Represents the outcome of a forced transaction inclusion attempt. */
public enum ForcedTransactionInclusionResult {
  /** Transaction was successfully included in the block. */
  Included,

  /** Transaction failed due to nonce mismatch. */
  BadNonce,

  /** Transaction failed due to insufficient balance. */
  BadBalance,

  /** Transaction failed due to precompile call failure. */
  BadPrecompile,

  /** Transaction failed due to exceeding log limits. */
  TooManyLogs,

  /** Transaction failed because sender address is on deny list. */
  FilteredAddressFrom,

  /** Transaction failed because recipient address is on deny list. */
  FilteredAddressTo,

  /** Transaction was rejected by Phylax filtering. */
  Phylax,

  /**
   * Transaction failed for an unrecognized reason. This is a transient status - the transaction
   * will be retried in the next block rather than being finalized with this status.
   */
  Other;

  /**
   * Returns true if this result is transient and the transaction should be retried.
   *
   * @return true if the transaction should be retried
   */
  public boolean shouldRetry() {
    return this == Other;
  }
}
