/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_MODULE_LINE_COUNT_OVERFLOW;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_MODULE_LINE_COUNT_OVERFLOW_CACHED;

import java.time.Instant;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bl.TransactionProfitabilityCalculator;
import net.consensys.linea.bundles.TransactionBundle;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.config.LineaTransactionSelectorConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.jsonrpc.JsonRpcRequestBuilder;
import net.consensys.linea.metrics.HistogramMetrics;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.txselection.InvalidTransactionByLineCountCache;
import net.consensys.linea.zktracer.LineCountingTracer;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

/** Class for transaction selection using a list of selectors. */
@Slf4j
public class LineaTransactionSelector implements PluginTransactionSelector {

  private TraceLineLimitTransactionSelector traceLineLimitTransactionSelector;
  private final List<PluginTransactionSelector> selectors;
  private final Optional<JsonRpcManager> rejectedTxJsonRpcManager;
  private final Set<String> rejectedTransactionReasonsMap = new HashSet<>();

  public LineaTransactionSelector(
      final SelectorsStateManager selectorsStateManager,
      final BlockchainService blockchainService,
      final LineaTransactionSelectorConfiguration txSelectorConfiguration,
      final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration,
      final LineaProfitabilityConfiguration profitabilityConfiguration,
      final LineaTracerConfiguration tracerConfiguration,
      final Optional<JsonRpcManager> rejectedTxJsonRpcManager,
      final Optional<HistogramMetrics> maybeProfitabilityMetrics,
      final InvalidTransactionByLineCountCache invalidTransactionByLineCountCache,
      final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedEvents,
      final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedBundleEvents,
      final AtomicReference<Set<Address>> deniedAddresses,
      final TransactionProfitabilityCalculator transactionProfitabilityCalculator) {
    this.rejectedTxJsonRpcManager = rejectedTxJsonRpcManager;

    // only report rejected transaction selection result from TraceLineLimitTransactionSelector
    if (rejectedTxJsonRpcManager.isPresent()) {
      rejectedTransactionReasonsMap.add(TX_MODULE_LINE_COUNT_OVERFLOW.toString());
      rejectedTransactionReasonsMap.add(TX_MODULE_LINE_COUNT_OVERFLOW_CACHED.toString());
    }

    selectors =
        createTransactionSelectors(
            selectorsStateManager,
            blockchainService,
            txSelectorConfiguration,
            l1L2BridgeConfiguration,
            profitabilityConfiguration,
            tracerConfiguration,
            maybeProfitabilityMetrics,
            invalidTransactionByLineCountCache,
            deniedEvents,
            deniedBundleEvents,
            deniedAddresses,
            transactionProfitabilityCalculator);
  }

  /**
   * Creates a list of selectors based on Linea configuration.
   *
   * @param selectorsStateManager
   * @param blockchainService Blockchain service.
   * @param txSelectorConfiguration The configuration to use.
   * @param profitabilityConfiguration The profitability configuration.
   * @param tracerConfiguration the tracer config
   * @param maybeProfitabilityMetrics The optional profitability metrics
   * @param deniedEvents The transaction event deny list
   * @param deniedBundleEvents The bundle transaction event deny list
   * @param deniedAddresses The denied addresses set
   * @return A list of selectors.
   */
  private List<PluginTransactionSelector> createTransactionSelectors(
      final SelectorsStateManager selectorsStateManager,
      final BlockchainService blockchainService,
      final LineaTransactionSelectorConfiguration txSelectorConfiguration,
      final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration,
      final LineaProfitabilityConfiguration profitabilityConfiguration,
      final LineaTracerConfiguration tracerConfiguration,
      final Optional<HistogramMetrics> maybeProfitabilityMetrics,
      final InvalidTransactionByLineCountCache invalidTransactionByLineCountCache,
      final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedEvents,
      final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedBundleEvents,
      final AtomicReference<Set<Address>> deniedAddresses,
      final TransactionProfitabilityCalculator transactionProfitabilityCalculator) {

    traceLineLimitTransactionSelector =
        new TraceLineLimitTransactionSelector(
            selectorsStateManager,
            blockchainService,
            l1L2BridgeConfiguration,
            tracerConfiguration,
            invalidTransactionByLineCountCache);

    return List.of(
        new AllowedAddressTransactionSelector(deniedAddresses),
        new MaxBlockCallDataTransactionSelector(
            selectorsStateManager, txSelectorConfiguration.maxBlockCallDataSize()),
        new MaxBlockGasTransactionSelector(
            selectorsStateManager, txSelectorConfiguration.maxGasPerBlock()),
        new ProfitableTransactionSelector(
            blockchainService,
            profitabilityConfiguration,
            maybeProfitabilityMetrics,
            transactionProfitabilityCalculator),
        new BundleConstraintTransactionSelector(),
        new MaxBundleGasPerBlockTransactionSelector(
            selectorsStateManager, txSelectorConfiguration.maxBundleGasPerBlock()),
        traceLineLimitTransactionSelector,
        new TransactionEventSelector(deniedEvents, deniedBundleEvents));
  }

  /**
   * Evaluates a transaction before processing using all selectors. Stops if any selector doesn't
   * select the transaction.
   *
   * @param evaluationContext The current selection context.
   * @return The first non-SELECTED result or SELECTED if all selectors select the transaction.
   */
  @Override
  public TransactionSelectionResult evaluateTransactionPreProcessing(
      final TransactionEvaluationContext evaluationContext) {
    return selectors.stream()
        .map(selector -> selector.evaluateTransactionPreProcessing(evaluationContext))
        .filter(result -> !result.equals(TransactionSelectionResult.SELECTED))
        .findFirst()
        .orElse(TransactionSelectionResult.SELECTED);
  }

  /**
   * Evaluates a transaction considering its processing result. Stops if any selector doesn't select
   * the transaction.
   *
   * @param evaluationContext The current selection context.
   * @param processingResult The result of the transaction processing.
   * @return The first non-SELECTED result or SELECTED if all selectors select the transaction.
   */
  @Override
  public TransactionSelectionResult evaluateTransactionPostProcessing(
      final TransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult) {
    for (var selector : selectors) {
      TransactionSelectionResult result =
          selector.evaluateTransactionPostProcessing(evaluationContext, processingResult);
      if (!result.equals(TransactionSelectionResult.SELECTED)) {
        return result;
      }
    }
    return TransactionSelectionResult.SELECTED;
  }

  /**
   * Notifies all selectors when a transaction is selected.
   *
   * @param evaluationContext The current selection context.
   * @param processingResult The transaction processing result.
   */
  @Override
  public void onTransactionSelected(
      final TransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult) {

    // if pending tx is not from a bundle, then we need to commit now
    if (!(evaluationContext.getPendingTransaction() instanceof TransactionBundle.PendingBundleTx)) {
      getOperationTracer().commitTransactionBundle();
    }

    selectors.forEach(
        selector -> selector.onTransactionSelected(evaluationContext, processingResult));
  }

  /**
   * Notifies all selectors when a transaction is not selected.
   *
   * @param evaluationContext The current selection context.
   * @param transactionSelectionResult The reason for not selecting the transaction.
   */
  @Override
  public void onTransactionNotSelected(
      final TransactionEvaluationContext evaluationContext,
      final TransactionSelectionResult transactionSelectionResult) {

    // if pending tx is not from a bundle, then we need to rollback now
    if (!(evaluationContext.getPendingTransaction() instanceof TransactionBundle.PendingBundleTx)) {
      getOperationTracer().popTransactionBundle();
    }

    selectors.forEach(
        selector ->
            selector.onTransactionNotSelected(evaluationContext, transactionSelectionResult));

    rejectedTxJsonRpcManager.ifPresent(
        jsonRpcManager -> {
          if (transactionSelectionResult.discard()
              && rejectedTransactionReasonsMap.contains(transactionSelectionResult.toString())) {
            jsonRpcManager.submitNewJsonRpcCallAsync(
                JsonRpcRequestBuilder.generateSaveRejectedTxJsonRpc(
                    jsonRpcManager.getNodeType(),
                    evaluationContext.getPendingTransaction().getTransaction(),
                    Instant.now(),
                    Optional.of(evaluationContext.getPendingBlockHeader().getNumber()),
                    transactionSelectionResult.toString(),
                    List.of()));
          }
        });
  }

  /**
   * Returns the operation tracer to be used while processing the transactions for the block.
   *
   * @return the operation tracer
   */
  @Override
  public LineCountingTracer getOperationTracer() {
    return traceLineLimitTransactionSelector.getOperationTracer();
  }
}
