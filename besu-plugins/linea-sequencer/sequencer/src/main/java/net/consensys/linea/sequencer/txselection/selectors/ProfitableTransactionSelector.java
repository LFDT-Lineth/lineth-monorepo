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

import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_UNPROFITABLE;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_UNPROFITABLE_RETRY_LIMIT;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_UNPROFITABLE_UPFRONT;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTED;

import java.util.EnumMap;
import java.util.LinkedHashSet;
import java.util.Locale;
import java.util.Map;
import java.util.Optional;
import java.util.Set;

import com.google.common.annotations.VisibleForTesting;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bl.TransactionProfitabilityCalculator;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.config.LineaTransactionSelectorConfiguration;
import net.consensys.linea.metrics.HistogramMetrics;
import net.consensys.linea.metrics.HistogramMetrics.LabelValue;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.plugin.data.TransactionProcessingResult;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext;

/**
 * This class implements TransactionSelector and provides a specific implementation for evaluating
 * if the transaction is profitable, according to the current config and the min margin defined for
 * this context. Profitability check is done upfront using the gas limit, to avoid processing the
 * transaction at all, and if it passes it is done after the processing this time using the actual
 * gas used by the transaction. This selector keeps a cache of the unprofitable transactions to
 * avoid reprocessing all of them everytime, and only allows for a configurable limited number of
 * unprofitable transaction to be retried on every new block creation.
 */
@Slf4j
public class ProfitableTransactionSelector implements PluginTransactionSelector {
  public enum Phase implements LabelValue {
    PRE_PROCESSING,
    POST_PROCESSING;

    final String value;

    Phase() {
      this.value = name().toLowerCase(Locale.ROOT);
    }

    @Override
    public String value() {
      return value;
    }
  }

  @VisibleForTesting protected static Set<Hash> unprofitableCache = new LinkedHashSet<>();
  protected static Map<Phase, Double> lastBlockMinRatios = new EnumMap<>(Phase.class);
  protected static Map<Phase, Double> lastBlockMaxRatios = new EnumMap<>(Phase.class);

  static {
    resetMinMaxRatios();
  }

  private final LineaTransactionSelectorConfiguration txSelectorConf;
  private final LineaProfitabilityConfiguration profitabilityConf;
  private final TransactionProfitabilityCalculator transactionProfitabilityCalculator;
  private final Optional<HistogramMetrics> maybeProfitabilityMetrics;
  private final Wei baseFee;

  private int unprofitableRetries;

  public ProfitableTransactionSelector(
      final BlockchainService blockchainService,
      final LineaTransactionSelectorConfiguration txSelectorConf,
      final LineaProfitabilityConfiguration profitabilityConf,
      final Optional<HistogramMetrics> maybeProfitabilityMetrics) {
    this.txSelectorConf = txSelectorConf;
    this.profitabilityConf = profitabilityConf;
    this.transactionProfitabilityCalculator =
        new TransactionProfitabilityCalculator(profitabilityConf);
    this.maybeProfitabilityMetrics = maybeProfitabilityMetrics;
    maybeProfitabilityMetrics.ifPresent(
        histogramMetrics -> {
          // temporary solution to update min and max metrics
          // we should do this just after the block is created, but we do not have any API for that
          // so we postponed the update asap the next block creation starts.
          histogramMetrics.setMinMax(
              lastBlockMinRatios.get(Phase.PRE_PROCESSING),
              lastBlockMaxRatios.get(Phase.PRE_PROCESSING),
              Phase.PRE_PROCESSING.value());
          histogramMetrics.setMinMax(
              lastBlockMinRatios.get(Phase.POST_PROCESSING),
              lastBlockMaxRatios.get(Phase.POST_PROCESSING),
              Phase.POST_PROCESSING.value());
          log.atTrace()
              .setMessage("Setting profitability ratio metrics for last block to min={}, max={}")
              .addArgument(lastBlockMinRatios)
              .addArgument(lastBlockMaxRatios)
              .log();
          resetMinMaxRatios();
        });

    this.baseFee =
        blockchainService
            .getNextBlockBaseFee()
            .orElseThrow(() -> new RuntimeException("We only support a base fee market"));
  }

  /**
   * Evaluates a transaction before processing. Checks if it is profitable using its gas limit. If
   * the transaction was found to be unprofitable during a previous block creation process, it is
   * retried, since the gas price market could now make it profitable, but only a configurable
   * amount of these transactions is retried each time, to avoid that they could potentially consume
   * all the time allocated to block creation.
   *
   * @param evaluationContext The current selection context.
   * @return TX_UNPROFITABLE_UPFRONT if the transaction is not profitable upfront,
   *     TX_UNPROFITABLE_RETRY_LIMIT if the transaction was already found to be unprofitable, and
   *     there are no more slot to retry past unprofitable transactions during this block creation
   *     process, otherwise SELECTED.
   */
  @Override
  public TransactionSelectionResult evaluateTransactionPreProcessing(
      final TransactionEvaluationContext evaluationContext) {

    final Wei minGasPrice = evaluationContext.getMinGasPrice();

    if (!evaluationContext.getPendingTransaction().hasPriority()) {
      final Transaction transaction = evaluationContext.getPendingTransaction().getTransaction();
      final long gasLimit = transaction.getGasLimit();

      final var profitablePriorityFeePerGas =
          transactionProfitabilityCalculator.profitablePriorityFeePerGas(
              transaction, profitabilityConf.minMargin(), gasLimit, minGasPrice);

      updateMetric(
          Phase.PRE_PROCESSING, evaluationContext, transaction, profitablePriorityFeePerGas);

      // check the upfront profitability using the gas limit of the tx
      if (!transactionProfitabilityCalculator.isProfitable(
          "PreProcessing",
          profitablePriorityFeePerGas,
          transaction,
          profitabilityConf.minMargin(),
          baseFee,
          evaluationContext.getTransactionGasPrice(),
          gasLimit,
          minGasPrice)) {
        return TX_UNPROFITABLE_UPFRONT;
      }

      if (unprofitableCache.contains(transaction.getHash())) {
        if (unprofitableRetries >= txSelectorConf.unprofitableRetryLimit()) {
          log.atTrace()
              .setMessage("Limit of unprofitable tx retries reached: {}/{}")
              .addArgument(unprofitableRetries)
              .addArgument(txSelectorConf.unprofitableRetryLimit());
          return TX_UNPROFITABLE_RETRY_LIMIT;
        }

        log.atTrace()
            .setMessage("Retrying unprofitable tx. Retry: {}/{}")
            .addArgument(unprofitableRetries)
            .addArgument(txSelectorConf.unprofitableRetryLimit());
        unprofitableCache.remove(transaction.getHash());
        unprofitableRetries++;
      }
    }

    return SELECTED;
  }

  /**
   * Evaluates a transaction post-processing. Checks if it is profitable according to its gas used.
   * If unprofitable, the transaction is added to the unprofitable cache, to be retried in the
   * future, since gas price market fluctuations can make it profitable again.
   *
   * @param evaluationContext The current selection context.
   * @return TX_UNPROFITABLE if the transaction is not profitable after execution, otherwise
   *     SELECTED.
   */
  @Override
  public TransactionSelectionResult evaluateTransactionPostProcessing(
      final TransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult) {

    if (!evaluationContext.getPendingTransaction().hasPriority()) {
      final Transaction transaction = evaluationContext.getPendingTransaction().getTransaction();
      final long gasUsed = processingResult.getEstimateGasUsedByTransaction();

      final var profitablePriorityFeePerGas =
          transactionProfitabilityCalculator.profitablePriorityFeePerGas(
              transaction,
              profitabilityConf.minMargin(),
              gasUsed,
              evaluationContext.getMinGasPrice());

      updateMetric(
          Phase.POST_PROCESSING, evaluationContext, transaction, profitablePriorityFeePerGas);

      if (!transactionProfitabilityCalculator.isProfitable(
          "PostProcessing",
          profitablePriorityFeePerGas,
          transaction,
          profitabilityConf.minMargin(),
          baseFee,
          evaluationContext.getTransactionGasPrice(),
          gasUsed,
          evaluationContext.getMinGasPrice())) {
        rememberUnprofitable(transaction);
        return TX_UNPROFITABLE;
      }
    }
    return SELECTED;
  }

  /**
   * If the transaction has been selected for block inclusion, then we remove it from the
   * unprofitable cache.
   *
   * @param evaluationContext The current selection context
   * @param processingResult The result of processing the selected transaction.
   */
  @Override
  public void onTransactionSelected(
      final TransactionEvaluationContext evaluationContext,
      final TransactionProcessingResult processingResult) {
    unprofitableCache.remove(evaluationContext.getPendingTransaction().getTransaction().getHash());
  }

  /**
   * If the transaction has not been selected and has been discarded from the transaction pool, then
   * we remove it from the unprofitable cache.
   *
   * @param evaluationContext The current selection context
   * @param transactionSelectionResult The transaction selection result
   */
  @Override
  public void onTransactionNotSelected(
      final TransactionEvaluationContext evaluationContext,
      final TransactionSelectionResult transactionSelectionResult) {
    final var txHash = evaluationContext.getPendingTransaction().getTransaction().getHash();
    if (transactionSelectionResult.discard()) {
      unprofitableCache.remove(txHash);
    }
  }

  private void rememberUnprofitable(final Transaction transaction) {
    while (unprofitableCache.size() >= txSelectorConf.unprofitableCacheSize()) {
      final var it = unprofitableCache.iterator();
      if (it.hasNext()) {
        it.next();
        it.remove();
      }
    }
    unprofitableCache.add(transaction.getHash());
    log.atTrace().setMessage("unprofitableCache={}").addArgument(unprofitableCache::size).log();
  }

  private void updateMetric(
      final Phase label,
      final TransactionEvaluationContext evaluationContext,
      final Transaction tx,
      final Wei profitablePriorityFeePerGas) {

    final var effectivePriorityFee = evaluationContext.getTransactionGasPrice().subtract(baseFee);
    final var ratio =
        effectivePriorityFee.getValue().doubleValue()
            / profitablePriorityFeePerGas.getValue().doubleValue();

    maybeProfitabilityMetrics.ifPresent(
        histogramMetrics -> {
          histogramMetrics.track(ratio, label.value());

          if (ratio < lastBlockMinRatios.get(label)) {
            lastBlockMinRatios.put(label, ratio);
          }
          if (ratio > lastBlockMaxRatios.get(label)) {
            lastBlockMaxRatios.put(label, ratio);
          }
        });

    log.atTrace()
        .setMessage(
            "{}: block[{}] tx {} , baseFee {}, effectiveGasPrice {}, ratio (effectivePayingPriorityFee {} / calculatedProfitablePriorityFee {}) {}")
        .addArgument(label.name())
        .addArgument(evaluationContext.getPendingBlockHeader().getNumber())
        .addArgument(tx.getHash())
        .addArgument(baseFee::toHumanReadableString)
        .addArgument(evaluationContext.getTransactionGasPrice()::toHumanReadableString)
        .addArgument(effectivePriorityFee::toHumanReadableString)
        .addArgument(profitablePriorityFeePerGas::toHumanReadableString)
        .addArgument(ratio)
        .log();
  }

  private static void resetMinMaxRatios() {
    lastBlockMinRatios.put(Phase.PRE_PROCESSING, Double.POSITIVE_INFINITY);
    lastBlockMinRatios.put(Phase.POST_PROCESSING, Double.POSITIVE_INFINITY);
    lastBlockMaxRatios.put(Phase.PRE_PROCESSING, Double.NEGATIVE_INFINITY);
    lastBlockMaxRatios.put(Phase.POST_PROCESSING, Double.NEGATIVE_INFINITY);
  }
}
