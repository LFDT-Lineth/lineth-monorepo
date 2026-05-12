/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.sequencer.txvalidation;

import java.util.Optional;
import net.consensys.linea.config.LineaTransactionValidatorConfiguration;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.TransactionType;

/**
 * Shared validation logic for transaction type checks (blob, delegate code). Used by both the
 * block-level validator plugin and the pool-level validator.
 */
public class TransactionTypeValidation {

  public enum Error {
    BLOB_TX_NOT_ALLOWED,
    DELEGATE_CODE_TX_NOT_ALLOWED;

    @Override
    public String toString() {
      return "TransactionTypeValidation - " + name();
    }
  }

  /**
   * Validates whether the given transaction type is allowed by the current configuration.
   *
   * @param transaction the transaction to validate
   * @param config the configuration containing enabled/disabled flags
   * @return an optional error message if the transaction type is not allowed, or empty if valid
   */
  public static Optional<String> validate(
      final Transaction transaction, final LineaTransactionValidatorConfiguration config) {
    if (transaction.getType() == TransactionType.BLOB && !config.blobTxEnabled()) {
      return Optional.of(Error.BLOB_TX_NOT_ALLOWED.toString());
    } else if (transaction.getType() == TransactionType.DELEGATE_CODE
        && !config.delegateCodeTxEnabled()) {
      return Optional.of(Error.DELEGATE_CODE_TX_NOT_ALLOWED.toString());
    }
    return Optional.empty();
  }
}
