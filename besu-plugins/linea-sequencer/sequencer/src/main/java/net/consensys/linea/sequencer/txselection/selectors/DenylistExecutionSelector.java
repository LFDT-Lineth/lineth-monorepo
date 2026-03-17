/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_CALLED;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;

import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import lombok.extern.slf4j.Slf4j;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

/**
 * Transaction selector that rejects transactions which called a denied address during EVM
 * execution. Works in post-processing by checking the addresses collected by {@link
 * DenylistOperationTracer} against the denylist.
 */
@Slf4j
public class DenylistExecutionSelector implements PluginTransactionSelector {

  private final AtomicReference<Set<Address>> denied;
  private final DenylistOperationTracer tracer;

  public DenylistExecutionSelector(final AtomicReference<Set<Address>> denied, final DenylistOperationTracer tracer) {
    this.denied = denied;
    this.tracer = tracer;
  }

  @Override
  public TransactionSelectionResult evaluateTransactionPreProcessing(
      final TransactionEvaluationContext evaluationContext) {
    return SELECTED;
  }

  @Override
  public TransactionSelectionResult evaluateTransactionPostProcessing(
      final TransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult) {
    final Set<Address> denyList = denied.get();
    for (final Address calledAddress : tracer.getCalledAddresses()) {
      if (denyList.contains(calledAddress)) {
        log.atInfo()
            .setMessage("action=reject_filtered_address_called txHash={} calledAddress={}")
            .addArgument(
                () -> evaluationContext.getPendingTransaction().getTransaction().getHash())
            .addArgument(calledAddress)
            .log();
        return TX_FILTERED_ADDRESS_CALLED;
      }
    }
    return SELECTED;
  }
}
