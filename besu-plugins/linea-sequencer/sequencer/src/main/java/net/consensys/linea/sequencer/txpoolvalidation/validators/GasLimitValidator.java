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
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;

/**
 * Validator that checks if the gas limit is below the configured max amount. This means that max
 * gas limit of a transaction could be less than the block gas limit.
 */
@Slf4j
@RequiredArgsConstructor
public class GasLimitValidator implements PluginTransactionPoolValidator {
  final int maxTxGasLimit;

  @Override
  public Optional<String> validateTransaction(
      final Transaction transaction, final boolean isLocal, final boolean hasPriority) {
    if (transaction.getGasLimit() > maxTxGasLimit) {
      final String errMsg =
          "Gas limit of transaction is greater than the allowed max of " + maxTxGasLimit;
      log.debug(errMsg);
      return Optional.of(errMsg);
    }
    return Optional.empty();
  }
}
