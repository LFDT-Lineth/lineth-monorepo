/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_AUTHORIZATION;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_FROM;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_TO;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;

import java.util.List;
import java.util.Optional;
import java.util.Set;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.CodeDelegation;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

/**
 * Transaction selector that rejects transactions based on sender, recipient, or EIP-7702
 * authorization list address validation. Checks against a deny list and returns different results
 * for sender, recipient, and authorization list filtering.
 */
@Slf4j
@RequiredArgsConstructor
public class AllowedAddressTransactionSelector implements PluginTransactionSelector {

  private final Set<Address> denied;

  @Override
  public TransactionSelectionResult evaluateTransactionPreProcessing(
      final TransactionEvaluationContext evaluationContext) {
    final Transaction transaction = evaluationContext.getPendingTransaction().getTransaction();

    if (denied.contains(transaction.getSender())) {
      log.atInfo()
          .setMessage("action=reject_filtered_address_from txHash={} sender={}")
          .addArgument(transaction::getHash)
          .addArgument(transaction::getSender)
          .log();
      return TX_FILTERED_ADDRESS_FROM;
    }

    if (transaction.getTo().isPresent() && denied.contains(transaction.getTo().get())) {
      log.atInfo()
          .setMessage("action=reject_filtered_address_to txHash={} to={}")
          .addArgument(transaction::getHash)
          .addArgument(() -> transaction.getTo().get())
          .log();
      return TX_FILTERED_ADDRESS_TO;
    }

    final Optional<List<CodeDelegation>> codeDelegationList = transaction.getCodeDelegationList();
    if (codeDelegationList.isPresent()) {
      for (final CodeDelegation delegation : codeDelegationList.get()) {
        final Optional<Address> maybeAuthority = delegation.authorizer();
        if (maybeAuthority.isEmpty()) {
          log.atInfo()
              .setMessage("action=skip_unrecoverable_authority delegationAddress={}")
              .addArgument(delegation::address)
              .log();
        } else if (denied.contains(maybeAuthority.get())) {
          log.atInfo()
              .setMessage("action=reject_filtered_address_authorization txHash={} authority={}")
              .addArgument(transaction::getHash)
              .addArgument(maybeAuthority::get)
              .log();
          return TX_FILTERED_ADDRESS_AUTHORIZATION;
        }

        if (denied.contains(delegation.address())) {
          log.atInfo()
              .setMessage(
                  "action=reject_filtered_address_authorization txHash={} delegationAddress={}")
              .addArgument(transaction::getHash)
              .addArgument(delegation::address)
              .log();
          return TX_FILTERED_ADDRESS_AUTHORIZATION;
        }
      }
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
