/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.DENIED_LOG_TOPIC;

import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bundles.TransactionBundle;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.log.Log;
import org.hyperledger.besu.evm.log.LogTopic;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

@Slf4j
@RequiredArgsConstructor
public class TransactionEventSelector implements PluginTransactionSelector {
  private final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedEvents;
  private final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedBundleEvents;

  @Override
  public TransactionSelectionResult evaluateTransactionPreProcessing(
      final TransactionEvaluationContext evaluationContext) {
    return TransactionSelectionResult.SELECTED;
  }

  @Override
  public TransactionSelectionResult evaluateTransactionPostProcessing(
      final TransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult) {
    final boolean isBundle =
        evaluationContext.getPendingTransaction() instanceof TransactionBundle.PendingBundleTx;
    final Map<Address, Set<TransactionEventFilter>> deniedEventsByAddress =
        isBundle ? deniedBundleEvents.get() : deniedEvents.get();
    for (Log resultLog : processingResult.getLogs()) {
      Address contractAddress = resultLog.getLogger();
      Set<TransactionEventFilter> deniedEventsForTransaction =
          deniedEventsByAddress.get(contractAddress);
      if (deniedEventsForTransaction != null) {
        List<LogTopic> logTopics =
            resultLog.getTopics().stream()
                .map((logTopic) -> LogTopic.wrap(Bytes32.leftPad(logTopic)))
                .toList();
        for (TransactionEventFilter deniedEvent : deniedEventsForTransaction) {
          if (deniedEvent.matches(contractAddress, logTopics.toArray(new LogTopic[] {}))) {
            log.atDebug()
                .setMessage(
                    "Transaction {} is blocked due to contract address and event logs appearing on SDN or other legally prohibited list")
                .addArgument(
                    () -> evaluationContext.getPendingTransaction().getTransaction().getHash())
                .log();
            return DENIED_LOG_TOPIC;
          }
        }
      }
    }
    return TransactionSelectionResult.SELECTED;
  }
}
