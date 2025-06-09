/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txpoolvalidation.metrics;

import static net.consensys.linea.metrics.LineaMetricCategory.TX_POOL_PROFITABILITY;

import java.util.stream.Collectors;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bl.TransactionProfitabilityCalculator;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.metrics.HistogramMetrics;
import org.apache.tuweni.units.bigints.UInt256s;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.plugin.services.BesuConfiguration;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.MetricsSystem;
import org.hyperledger.besu.plugin.services.transactionpool.TransactionPoolService;

/**
 * Tracks profitability metrics for transactions in the transaction pool. Specifically monitors the
 * ratio of profitable priority fee to actual priority fee:
 * profitablePriorityFeePerGas/transaction.priorityFeePerGas
 *
 * <p>Provides: - Lowest ratio seen (minimum profitability) - Highest ratio seen (maximum
 * profitability) - Distribution histogram of ratios
 */
@Slf4j
public class TransactionPoolProfitabilityMetrics {
  private final TransactionProfitabilityCalculator profitabilityCalculator;
  private final LineaProfitabilityConfiguration profitabilityConf;
  private final BesuConfiguration besuConfiguration;
  private final TransactionPoolService transactionPoolService;
  private final BlockchainService blockchainService;
  private final HistogramMetrics histogramMetrics;

  public TransactionPoolProfitabilityMetrics(
      final BesuConfiguration besuConfiguration,
      final MetricsSystem metricsSystem,
      final LineaProfitabilityConfiguration profitabilityConf,
      final TransactionPoolService transactionPoolService,
      final BlockchainService blockchainService) {

    this.besuConfiguration = besuConfiguration;
    this.profitabilityConf = profitabilityConf;
    this.profitabilityCalculator = new TransactionProfitabilityCalculator(profitabilityConf);
    this.transactionPoolService = transactionPoolService;
    this.blockchainService = blockchainService;
    this.histogramMetrics =
        new HistogramMetrics(
            metricsSystem,
            TX_POOL_PROFITABILITY,
            "ratio",
            "transaction pool profitability ratio",
            profitabilityConf.profitabilityMetricsBuckets());
  }

  public void update() {
    final long startTime = System.currentTimeMillis();
    final var txPoolContent = transactionPoolService.getPendingTransactions();

    final var ratioStats =
        txPoolContent.parallelStream()
            .map(PendingTransaction::getTransaction)
            .map(
                tx -> {
                  final var ratio = handleTransaction(tx);
                  histogramMetrics.track(ratio);
                  log.trace("Recorded profitability ratio {} for tx {}", ratio, tx.getHash());
                  return ratio;
                })
            .collect(Collectors.summarizingDouble(Double::doubleValue));

    histogramMetrics.setMinMax(ratioStats.getMin(), ratioStats.getMax());

    log.atDebug()
        .setMessage("Transaction pool profitability metrics processed {}txs in {}ms, statistics {}")
        .addArgument(txPoolContent::size)
        .addArgument(() -> System.currentTimeMillis() - startTime)
        .addArgument(ratioStats)
        .log();
  }

  private double handleTransaction(final Transaction transaction) {
    final Wei actualPriorityFeePerGas;
    if (transaction.getMaxPriorityFeePerGas().isEmpty()) {
      actualPriorityFeePerGas =
          Wei.fromQuantity(transaction.getGasPrice().orElseThrow())
              .subtract(blockchainService.getNextBlockBaseFee().orElseThrow());
    } else {
      final Wei maxPriorityFeePerGas =
          Wei.fromQuantity(transaction.getMaxPriorityFeePerGas().get());
      actualPriorityFeePerGas =
          UInt256s.min(
              maxPriorityFeePerGas.add(blockchainService.getNextBlockBaseFee().orElseThrow()),
              Wei.fromQuantity(transaction.getMaxFeePerGas().orElseThrow()));
    }

    final Wei profitablePriorityFeePerGas =
        profitabilityCalculator.profitablePriorityFeePerGas(
            transaction,
            profitabilityConf.txPoolMinMargin(),
            transaction.getGasLimit(),
            besuConfiguration.getMinGasPrice());

    final double ratio =
        actualPriorityFeePerGas.toBigInteger().doubleValue()
            / profitablePriorityFeePerGas.toBigInteger().doubleValue();

    return ratio;
  }
}
