/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package net.consensys.linea.sequencer.txselection.selectors;

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.BLOCK_MODULE_LINE_COUNT_FULL;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_MODULE_LINE_COUNT_OVERFLOW;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_MODULE_LINE_COUNT_OVERFLOW_CACHED;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_MODULE_LINE_INVALID_COUNT;
import static net.consensys.linea.zktracer.Fork.LONDON;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;

import java.math.BigInteger;
import java.util.LinkedHashSet;
import java.util.Map;
import java.util.Set;
import java.util.function.Function;
import java.util.stream.Collectors;

import com.google.common.annotations.VisibleForTesting;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.config.LineaTransactionSelectorConfiguration;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.modulelimit.ModuleLimitsValidationResult;
import net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator;
import net.consensys.linea.zktracer.ZkTracer;
import net.consensys.linea.zktracer.container.module.Module;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.data.BlockBody;
import org.hyperledger.besu.plugin.data.BlockHeader;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.txselection.AbstractStatefulPluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;
import org.slf4j.Marker;
import org.slf4j.MarkerFactory;

/**
 * This class implements TransactionSelector and provides a specific implementation for evaluating
 * transactions based on the number of trace lines per module created by a transaction. It checks if
 * adding a transaction to the block pushes the trace lines for a module over the limit.
 */
@Slf4j
public class TraceLineLimitTransactionSelector
    extends AbstractStatefulPluginTransactionSelector<Map<String, Integer>> {
  private static final Marker BLOCK_LINE_COUNT_MARKER = MarkerFactory.getMarker("BLOCK_LINE_COUNT");
  @VisibleForTesting protected static Set<Hash> overLineCountLimitCache = new LinkedHashSet<>();
  private final ZkTracer zkTracer;
  private final BigInteger chainId;
  private final String limitFilePath;
  private final Map<String, Integer> moduleLimits;
  private final int overLimitCacheSize;
  private final ModuleLineCountValidator moduleLineCountValidator;

  public TraceLineLimitTransactionSelector(
      final SelectorsStateManager stateManager,
      final BigInteger chainId,
      final Map<String, Integer> moduleLimits,
      final LineaTransactionSelectorConfiguration txSelectorConfiguration,
      final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration,
      final LineaTracerConfiguration tracerConfiguration) {
    super(
        stateManager,
        moduleLimits.keySet().stream().collect(Collectors.toMap(Function.identity(), unused -> 0)),
        Map::copyOf);

    this.chainId = chainId;
    this.moduleLimits = moduleLimits;
    this.limitFilePath = tracerConfiguration.moduleLimitsFilePath();
    this.overLimitCacheSize = txSelectorConfiguration.overLinesLimitCacheSize();

    zkTracer = new ZkTracerWithLog(l1L2BridgeConfiguration);
    for (Module m : zkTracer.getHub().getModulesToCount()) {
      if (!moduleLimits.containsKey(m.moduleKey())) {
        throw new IllegalStateException(
            "Limit for module %s not defined in %s".formatted(m.moduleKey(), this.limitFilePath));
      }
    }
    zkTracer.traceStartConflation(1L);
    moduleLineCountValidator = new ModuleLineCountValidator(moduleLimits);
  }

  /**
   * Check if the tx is already known to go over the limit to avoid reprocessing it
   *
   * @param evaluationContext The current selection context.
   * @return transaction selection result
   */
  @Override
  public TransactionSelectionResult evaluateTransactionPreProcessing(
      final TransactionEvaluationContext evaluationContext) {
    if (overLineCountLimitCache.contains(
        evaluationContext.getPendingTransaction().getTransaction().getHash())) {
      log.atTrace()
          .setMessage(
              "Transaction {} was already identified to go over line count limit, dropping it")
          .addArgument(evaluationContext.getPendingTransaction().getTransaction()::getHash)
          .log();
      return TX_MODULE_LINE_COUNT_OVERFLOW_CACHED;
    }
    return SELECTED;
  }

  /**
   * Checking the created trace lines is performed post-processing.
   *
   * @param evaluationContext The current selection context.
   * @param processingResult The result of the transaction processing.
   * @return BLOCK_MODULE_LINE_COUNT_FULL if the trace lines for a module are over the limit for the
   *     block, TX_MODULE_LINE_COUNT_OVERFLOW if the trace lines are over the limit for the single
   *     tx, otherwise SELECTED.
   */
  @Override
  public TransactionSelectionResult evaluateTransactionPostProcessing(
      final TransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult) {

    final var prevCumulatedLineCountMap = getWorkingState();

    // check that we are not exceeding line number for any module
    final var newCumulatedLineCountMap = zkTracer.getModulesLineCount();
    final Transaction transaction = evaluationContext.getPendingTransaction().getTransaction();
    log.atTrace()
        .setMessage("Tx {} line count per module: {}")
        .addArgument(transaction::getHash)
        .addArgument(() -> logTxLineCount(newCumulatedLineCountMap, prevCumulatedLineCountMap))
        .log();

    ModuleLimitsValidationResult result =
        moduleLineCountValidator.validate(newCumulatedLineCountMap, prevCumulatedLineCountMap);

    switch (result.getResult()) {
      case MODULE_NOT_DEFINED:
        log.error("Module {} does not exist in the limits file.", result.getModuleName());
        throw new RuntimeException(
            "Module " + result.getModuleName() + " does not exist in the limits file.");
      case INVALID_LINE_COUNT:
        log.warn(
            "Tx {} line count for module {}={} is invalid, removing from the txpool",
            transaction.getHash(),
            result.getModuleName(),
            result.getModuleLineCount());
        return TX_MODULE_LINE_INVALID_COUNT;
      case TX_MODULE_LINE_COUNT_OVERFLOW:
        log.warn(
            "Tx {} line count for module {}={} is above the limit {}, removing from the txpool",
            transaction.getHash(),
            result.getModuleName(),
            result.getModuleLineCount(),
            result.getModuleLineLimit());
        rememberOverLineCountLimitTransaction(transaction);
        return TX_MODULE_LINE_COUNT_OVERFLOW;
      case BLOCK_MODULE_LINE_COUNT_FULL:
        log.atTrace()
            .setMessage(
                "Cumulated line count for module {}={} is above the limit {}, stopping selection")
            .addArgument(result.getModuleName())
            .addArgument(result.getCumulativeModuleLineCount())
            .addArgument(result.getCumulativeModuleLineLimit())
            .log();
        return BLOCK_MODULE_LINE_COUNT_FULL;
      default:
        break;
    }

    setWorkingState(newCumulatedLineCountMap);

    return SELECTED;
  }

  @Override
  public ZkTracer getOperationTracer() {
    return zkTracer;
  }

  private void rememberOverLineCountLimitTransaction(final Transaction transaction) {
    while (overLineCountLimitCache.size() >= overLimitCacheSize) {
      final var it = overLineCountLimitCache.iterator();
      if (it.hasNext()) {
        it.next();
        it.remove();
      }
    }
    overLineCountLimitCache.add(transaction.getHash());
    log.atTrace()
        .setMessage("overLineCountLimitCache={}")
        .addArgument(overLineCountLimitCache::size)
        .log();
  }

  private String logTxLineCount(
      final Map<String, Integer> currCumulatedLineCount,
      final Map<String, Integer> stateLineLimitMap) {
    return currCumulatedLineCount.entrySet().stream()
        .map(
            e ->
                // tx line count / cumulated line count / line count limit
                e.getKey()
                    + "="
                    + (e.getValue() - stateLineLimitMap.getOrDefault(e.getKey(), 0))
                    + "/"
                    + e.getValue()
                    + "/"
                    + moduleLimits.get(e.getKey()))
        .collect(Collectors.joining(",", "[", "]"));
  }

  private class ZkTracerWithLog extends ZkTracer {
    public ZkTracerWithLog(final LineaL1L2BridgeSharedConfiguration bridgeConfiguration) {
      super(LONDON, bridgeConfiguration, chainId);
    }

    @Override
    public void traceEndBlock(final BlockHeader blockHeader, final BlockBody blockBody) {
      super.traceEndBlock(blockHeader, blockBody);
      log.atDebug()
          .addMarker(BLOCK_LINE_COUNT_MARKER)
          .addKeyValue("blockNumber", blockHeader::getNumber)
          .addKeyValue("blockHash", blockHeader::getBlockHash)
          .addKeyValue(
              "traceCounts",
              () ->
                  getCommitedState().entrySet().stream()
                      .sorted(Map.Entry.comparingByKey())
                      .map(e -> '"' + e.getKey() + "\":" + e.getValue())
                      .collect(Collectors.joining(",")))
          .log();
    }
  }
}
