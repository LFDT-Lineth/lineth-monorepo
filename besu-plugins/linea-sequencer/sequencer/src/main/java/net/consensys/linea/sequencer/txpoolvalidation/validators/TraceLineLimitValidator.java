/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txpoolvalidation.validators;

import java.util.Optional;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.sequencer.txselection.InvalidTransactionByLineCountCache;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;

/**
 * Validator that checks if a transaction is already known to go over the trace line count limit.
 * This validator uses a shared cache populated by the transaction selection process to avoid
 * reprocessing transactions that are already known to exceed line count limits.
 */
@Slf4j
public class TraceLineLimitValidator implements PluginTransactionPoolValidator {

  private final InvalidTransactionByLineCountCache invalidTransactionByLineCountCache;

  public TraceLineLimitValidator(
      final InvalidTransactionByLineCountCache invalidTransactionByLineCountCache) {
    this.invalidTransactionByLineCountCache = invalidTransactionByLineCountCache;
  }

  @Override
  public Optional<String> validateTransaction(
      final Transaction transaction, final boolean isLocal, final boolean hasPriority) {

    if (invalidTransactionByLineCountCache.contains(transaction.getHash())) {
      final String reason =
          String.format(
              "Transaction %s was already identified to go over line count limit",
              transaction.getHash());

      log.trace(reason);

      return Optional.of(reason);
    }

    return Optional.empty();
  }
}
