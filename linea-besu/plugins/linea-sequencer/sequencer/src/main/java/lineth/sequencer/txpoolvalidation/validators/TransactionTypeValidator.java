/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.sequencer.txpoolvalidation.validators;

import java.util.Optional;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import lineth.config.LineaTransactionValidatorConfiguration;
import lineth.sequencer.txvalidation.TransactionTypeValidation;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;

/**
 * Pool-level validator that rejects unsupported transaction types (blob, delegate code) based on
 * configuration. This provides explicit RPC/P2P-level enforcement of transaction type rules.
 */
@Slf4j
@RequiredArgsConstructor
public class TransactionTypeValidator implements PluginTransactionPoolValidator {
  final LineaTransactionValidatorConfiguration config;

  @Override
  public Optional<String> validateTransaction(
      final Transaction transaction, final boolean isLocal, final boolean hasPriority) {
    return TransactionTypeValidation.validate(transaction, config);
  }
}
