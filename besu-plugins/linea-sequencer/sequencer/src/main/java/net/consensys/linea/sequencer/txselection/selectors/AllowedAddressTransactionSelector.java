/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_FROM;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_TO;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;

import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

/**
 * Transaction selector that rejects transactions based on sender or recipient address validation.
 * Checks against a deny list and returns different results for sender vs recipient filtering.
 */
@Slf4j
@RequiredArgsConstructor
public class AllowedAddressTransactionSelector implements PluginTransactionSelector {

  private final AtomicReference<Set<Address>> denied;

  @Override
  public TransactionSelectionResult evaluateTransactionPreProcessing(
      final TransactionEvaluationContext evaluationContext) {
    final Transaction transaction = evaluationContext.getPendingTransaction().getTransaction();
    final Set<Address> denyList = denied.get();

    if (denyList.contains(transaction.getSender())) {
      log.atTrace()
          .setMessage("action=reject_filtered_address_from txHash={} sender={}")
          .addArgument(transaction::getHash)
          .addArgument(transaction::getSender)
          .log();
      return TX_FILTERED_ADDRESS_FROM;
    }

    if (transaction.getTo().isPresent() && denyList.contains(transaction.getTo().get())) {
      log.atTrace()
          .setMessage("action=reject_filtered_address_to txHash={} to={}")
          .addArgument(transaction::getHash)
          .addArgument(() -> transaction.getTo().get())
          .log();
      return TX_FILTERED_ADDRESS_TO;
    }

    return SELECTED;
  }

  @Override
  public TransactionSelectionResult evaluateTransactionPostProcessing(
      final TransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult) {
    return SELECTED;
  }
}
