/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.liveness;

import java.io.IOException;
import java.util.*;
import java.util.concurrent.atomic.AtomicLong;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bundles.TransactionBundle;
import net.consensys.linea.config.LineaLivenessServiceConfiguration;
import net.consensys.linea.metrics.LineaMetricCategory;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.services.MetricsSystem;
import org.hyperledger.besu.plugin.services.RpcEndpointService;
import org.hyperledger.besu.plugin.services.metrics.Counter;
import org.hyperledger.besu.plugin.services.metrics.MetricCategory;
import org.hyperledger.besu.plugin.services.metrics.MetricCategoryRegistry;
import org.hyperledger.besu.plugin.services.rpc.RpcResponseType;

/**
 * The LineaLivenessService is monitoring the blockchain and sending transactions to update the
 * LineaSequencerUptimeFeed contract when the sequencer is down/up.
 *
 * <p>This service works by checking the timestamp of the last block and comparing it to the current
 * time. If the last block is older than a configurable threshold, it sends two transactions: 1. A
 * 'down' transaction with the timestamp of the last block 2. An 'up' transaction with the current
 * timestamp
 *
 * <p>These transactions help protocols like Aave to better handle liquidations during sequencer
 * downtime.
 */
@Slf4j
public class LineaLivenessService implements LivenessService {
  private static final MetricCategory SEQUENCER_LIVENESS_CATEGORY =
      LineaMetricCategory.SEQUENCER_LIVENESS;

  private Counter uptimeTransactionsCounter;
  private Counter transactionFailureCounter;
  private final AtomicLong lastBlockTimestamp = new AtomicLong(0);
  private final AtomicLong lastDownBlockTimestamp = new AtomicLong(0);
  private final AtomicLong lastReportedDownBlockTimestamp = new AtomicLong(0);
  private final AtomicLong uptimeTransactionDownCount = new AtomicLong(0);
  private final AtomicLong uptimeTransactionUpCount = new AtomicLong(0);

  private final LineaLivenessServiceConfiguration lineaLivenessServiceConfiguration;
  private final RpcEndpointService rpcEndpointService;
  private final LivenessTxBuilder livenessTxBuilder;
  private final MetricCategoryRegistry metricCategoryRegistry;
  private final MetricsSystem metricsSystem;

  public LineaLivenessService(
      final LineaLivenessServiceConfiguration lineaLivenessServiceConfiguration,
      final RpcEndpointService rpcEndpointService,
      final LivenessTxBuilder livenessTxBuilder,
      final MetricCategoryRegistry metricCategoryRegistry,
      final MetricsSystem metricsSystem) {
    this.lineaLivenessServiceConfiguration = lineaLivenessServiceConfiguration;
    this.rpcEndpointService = rpcEndpointService;
    this.livenessTxBuilder = livenessTxBuilder;
    this.metricCategoryRegistry = metricCategoryRegistry;
    this.metricsSystem = metricsSystem;

    this.metricCategoryRegistry.addMetricCategory(SEQUENCER_LIVENESS_CATEGORY);

    // Initialize metrics if enabled
    if (this.lineaLivenessServiceConfiguration.enabled()
        && this.metricCategoryRegistry.isMetricCategoryEnabled(SEQUENCER_LIVENESS_CATEGORY)) {
      // Register metric category
      uptimeTransactionsCounter =
          this.metricsSystem.createCounter(
              SEQUENCER_LIVENESS_CATEGORY,
              "uptime_transactions",
              "Number of sequencer uptime transactions sent");

      transactionFailureCounter =
          this.metricsSystem.createCounter(
              SEQUENCER_LIVENESS_CATEGORY,
              "transaction_failures",
              "Number of transaction submission failures after");

      // Labeled gauge for better aggregation across instances
      final var labelledUptimeGauge =
          this.metricsSystem.createLabelledSuppliedGauge(
              SEQUENCER_LIVENESS_CATEGORY,
              "uptime_transactions",
              "Total number of sequencer uptime transactions sent by status",
              "status");
      labelledUptimeGauge.labels(uptimeTransactionDownCount::doubleValue, "down");
      labelledUptimeGauge.labels(uptimeTransactionUpCount::doubleValue, "up");
    }
  }

  private TransactionBundle buildTransactionBundle(
      long lastBlockTimestamp,
      long currentTimestamp,
      long targetBlockNumber,
      long minTimestamp,
      long maxTimestamp) {
    try {
      log.info(
          "Building bundle for sequencer uptime transactions: lastBlockTimestamp={}, currentTimestamp={}",
          lastBlockTimestamp,
          currentTimestamp);

      // Get nonce for the signer address
      String signerAddress = lineaLivenessServiceConfiguration.signerAddress();

      final var resp =
          rpcEndpointService.call(
              "eth_getTransactionCount", new Object[] {signerAddress, "latest"});

      if (!resp.getType().equals(RpcResponseType.SUCCESS)) {
        throw new IOException("Unable to query sender nonce");
      }

      final Long nonce = Long.decode((String) resp.getResult());

      log.debug("Retrieved valid nonce: {} for address: {}", nonce, signerAddress);

      // First transaction: mark as down with last block timestamp
      Transaction downTimeTransaction =
          livenessTxBuilder.buildUptimeTransaction(false, lastBlockTimestamp, nonce);

      // Second transaction: mark as up with current timestamp
      Transaction upTimeTransaction =
          livenessTxBuilder.buildUptimeTransaction(true, currentTimestamp, nonce + 1);

      List<Transaction> transactions = List.of(downTimeTransaction, upTimeTransaction);

      Hash bundleHash = Hash.hash(Bytes32.random());

      return new TransactionBundle(
          bundleHash,
          transactions,
          targetBlockNumber,
          Optional.of(minTimestamp),
          Optional.of(maxTimestamp),
          Optional.empty(),
          Optional.of(UUID.randomUUID()),
          true);
    } catch (IOException e) {
      log.error("Error building bundle for sequencer uptime transactions", e);
      throw new RuntimeException("Error building bundle for sequencer uptime transactions");
    }
  }

  private boolean shouldBuildLivenessBundle(
      long currentTimestamp, long lastBlockTimestamp, long targetBlockNumber) {
    boolean shouldBuild = false;

    long cachedLastBlockTimestamp = this.lastBlockTimestamp.get();
    long cachedLastDownBlockTimestamp = this.lastDownBlockTimestamp.get();
    long cachedLastReportedDownBlockTimestamp = this.lastReportedDownBlockTimestamp.get();

    long adjustedLastBlockTimestamp = cachedLastBlockTimestamp;
    long elapsedTimeSinceLastBlock = currentTimestamp - adjustedLastBlockTimestamp;

    // check if the cached lastBlockTimestamp was not recorded as lastDownBlockTimestamp
    // and if its elapsed time was longer than maxBlockAgeSeconds
    if (cachedLastBlockTimestamp > 0
        && cachedLastDownBlockTimestamp <= cachedLastReportedDownBlockTimestamp
        && cachedLastBlockTimestamp > cachedLastDownBlockTimestamp
        && elapsedTimeSinceLastBlock
            > lineaLivenessServiceConfiguration.maxBlockAgeSeconds().getSeconds()) {
      this.lastDownBlockTimestamp.set(cachedLastBlockTimestamp);
      shouldBuild = true;
    } else {
      // check whether cachedLastDownBlockTimestamp was reported or not,
      // if no, uses the given lastBlockTimestamp to calculate elapsed time,
      // otherwise, keeps using cachedLastDownBlockTimestamp
      adjustedLastBlockTimestamp =
          cachedLastDownBlockTimestamp > cachedLastReportedDownBlockTimestamp
              ? cachedLastDownBlockTimestamp
              : lastBlockTimestamp;

      elapsedTimeSinceLastBlock = currentTimestamp - adjustedLastBlockTimestamp;

      if (elapsedTimeSinceLastBlock
          > lineaLivenessServiceConfiguration.maxBlockAgeSeconds().getSeconds()) {
        // only update lastDownBlockTimestamp if the last late block was reported
        // should only happen for first lastBlockTimestamp check
        if (cachedLastDownBlockTimestamp <= cachedLastReportedDownBlockTimestamp) {
          this.lastDownBlockTimestamp.set(lastBlockTimestamp);
        }
        shouldBuild = true;
      }
    }

    log.debug(
        " targetBlockNumber={} lastBlockTimestamp={} cachedLastBlockTimestamp={}"
            + " cachedLastDownBlockTimestamp={} cachedLastReportedDownBlockTimestamp={} lastDownBlockTimestamp={}"
            + " currentTimestamp={} adjustedLastBlockTimestamp={} elapsedTimeSinceLastBlock={} shouldBuild={}",
        targetBlockNumber,
        lastBlockTimestamp,
        cachedLastBlockTimestamp,
        cachedLastDownBlockTimestamp,
        cachedLastReportedDownBlockTimestamp,
        lastDownBlockTimestamp.get(),
        currentTimestamp,
        adjustedLastBlockTimestamp,
        elapsedTimeSinceLastBlock,
        shouldBuild);

    this.lastBlockTimestamp.set(lastBlockTimestamp);

    return shouldBuild;
  }

  /** Checks the timestamp of the last block and reports downtime if necessary. */
  @Override
  public Optional<TransactionBundle> checkBlockTimestampAndBuildBundle(
      long currentTimestamp, long lastBlockTimestamp, long targetBlockNumber) {
    if (!lineaLivenessServiceConfiguration.enabled()) return Optional.empty();

    // skip if the same block timestamp had been checked or lastBlockTimestamp is from genesis block
    if (this.lastBlockTimestamp.get() == lastBlockTimestamp || targetBlockNumber <= 1) {
      return Optional.empty();
    }

    try {
      if (shouldBuildLivenessBundle(currentTimestamp, lastBlockTimestamp, targetBlockNumber)) {
        log.info(
            "Need to send liveness bundled txs at targetBlockNumber={} as last block elapsed time is greater than {}",
            targetBlockNumber,
            lineaLivenessServiceConfiguration.maxBlockAgeSeconds().getSeconds());
        return Optional.of(
            buildTransactionBundle(
                this.lastDownBlockTimestamp.get(),
                currentTimestamp,
                targetBlockNumber,
                currentTimestamp - 1,
                currentTimestamp
                    + lineaLivenessServiceConfiguration
                        .bundleMaxTimestampSurplusSecond()
                        .getSeconds()));
      } else {
        return Optional.empty();
      }
    } catch (RuntimeException e) {
      log.error("Unexpected error in checkBlockTimestampAndBuildBundle", e);
      return Optional.empty();
    }
  }

  @Override
  public void updateUptimeMetrics(boolean isSucceeded, long blockTimestamp) {
    if (isSucceeded) {
      if (this.metricCategoryRegistry.isMetricCategoryEnabled(SEQUENCER_LIVENESS_CATEGORY)) {
        uptimeTransactionUpCount.incrementAndGet();
        uptimeTransactionDownCount.incrementAndGet();
        uptimeTransactionsCounter.inc(2);
      }
      lastReportedDownBlockTimestamp.set(blockTimestamp);
    } else if (transactionFailureCounter != null) {
      if (this.metricCategoryRegistry.isMetricCategoryEnabled(SEQUENCER_LIVENESS_CATEGORY)) {
        transactionFailureCounter.inc(2);
      }
    }
  }
}
