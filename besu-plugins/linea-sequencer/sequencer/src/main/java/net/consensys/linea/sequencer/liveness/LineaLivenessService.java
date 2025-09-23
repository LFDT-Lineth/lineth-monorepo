/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.liveness;

import static net.consensys.linea.metrics.LineaMetricCategory.SEQUENCER_LIVENESS;

import java.io.IOException;
import java.util.*;
import java.util.concurrent.atomic.AtomicLong;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bundles.TransactionBundle;
import net.consensys.linea.config.LineaLivenessServiceConfiguration;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.plugin.services.MetricsSystem;
import org.hyperledger.besu.plugin.services.RpcEndpointService;
import org.hyperledger.besu.plugin.services.metrics.Counter;
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

    // Initialize metrics if enabled
    if (this.lineaLivenessServiceConfiguration.enabled()
        && this.metricCategoryRegistry.isMetricCategoryEnabled(SEQUENCER_LIVENESS)) {
      // Register metric category
      uptimeTransactionsCounter =
          this.metricsSystem.createCounter(
              SEQUENCER_LIVENESS,
              "uptime_transaction_success",
              "Number of sequencer uptime transaction submission successes");

      transactionFailureCounter =
          this.metricsSystem.createCounter(
              SEQUENCER_LIVENESS,
              "uptime_transaction_failure",
              "Number of sequencer uptime transaction submission failures");

      // Labeled gauge for better aggregation across instances
      final var labelledUptimeGauge =
          this.metricsSystem.createLabelledSuppliedGauge(
              SEQUENCER_LIVENESS,
              "uptime_transaction",
              "Number of succeeded sequencer uptime transactions by status",
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

  /**
   * Checks if it needs to update the LineaSequencerUptimeFeed contract.
   *
   * @param currentTimestamp the current epoch timestamp in second
   * @param lastBlockTimestamp the current head block timestamp in second
   * @param targetBlockNumber the target block number to include the bundle transaction to update
   *     the LineaSequencerUptimeFeed contract
   * @return boolean
   */
  private boolean shouldBuildLivenessBundle(
      long currentTimestamp, long lastBlockTimestamp, long targetBlockNumber) {
    boolean shouldBuild = false;

    // The last seen block timestamp before the latest block timestamp (i.e. lastBlockTimestamp)
    long cachedLastBlockTimestamp = this.lastBlockTimestamp.get();
    // The last block timestamp regarded as late
    long cachedLastDownBlockTimestamp = this.lastDownBlockTimestamp.get();
    // The last successfully reported block timestamp regarded as late
    long cachedLastReportedDownBlockTimestamp = this.lastReportedDownBlockTimestamp.get();

    // We only check to skip if it was earlier block timestamp than last seen one, because under
    // PoS we could have last seen block timestamp over and over again
    boolean hasSeenEarlierBlockTimestamp = lastBlockTimestamp < cachedLastBlockTimestamp;
    boolean isLastBlockTimestampFromGenesisBlock = targetBlockNumber <= 1;

    // skip the rest and return false if the earlier block timestamp had been checked or
    // the given lastBlockTimestamp is from genesis block
    if (hasSeenEarlierBlockTimestamp || isLastBlockTimestampFromGenesisBlock) {
      log.atDebug()
          .setMessage(
              "Skip the check as hasSeenEarlierBlockTimestamp={} isLastBlockTimestampFromGenesisBlock={}"
                  + " lastBlockTimestamp={} cachedLastBlockTimestamp={} targetBlockNumber={}")
          .addArgument(hasSeenEarlierBlockTimestamp)
          .addArgument(isLastBlockTimestampFromGenesisBlock)
          .addArgument(lastBlockTimestamp)
          .addArgument(cachedLastBlockTimestamp)
          .addArgument(targetBlockNumber)
          .log();
      return false;
    }

    long pastBlockTimestampToCheck = cachedLastBlockTimestamp;
    long elapsedTimeSincePastBlock = currentTimestamp - pastBlockTimestampToCheck;

    boolean isNotFirstBlockTimestampToCheck = cachedLastBlockTimestamp > 0;
    boolean lastDownBlockTimestampHasBeenReported =
        cachedLastDownBlockTimestamp <= cachedLastReportedDownBlockTimestamp;
    boolean lastSeenBlockTimestampIsNotDownBlockTimestamp =
        cachedLastBlockTimestamp > cachedLastDownBlockTimestamp;
    boolean lastSeenBlockTimestampWasLate =
        elapsedTimeSincePastBlock
            > lineaLivenessServiceConfiguration.maxBlockAgeSeconds().getSeconds();

    // Check if the last seen block timestamp was late, if yes
    // record it was last late block timestamp and return shouldBuild as true
    if (isNotFirstBlockTimestampToCheck
        && lastDownBlockTimestampHasBeenReported
        && lastSeenBlockTimestampIsNotDownBlockTimestamp
        && lastSeenBlockTimestampWasLate) {
      this.lastDownBlockTimestamp.set(cachedLastBlockTimestamp);
      shouldBuild = true;
    } else {
      // check whether the last late block timestamp was reported or not,
      // if reported, uses the given latest block timestamp to calculate elapsed time,
      // otherwise, keeps using the last late block timestamp as the past
      // block timestamp to calculate elapsed time
      pastBlockTimestampToCheck =
          !lastDownBlockTimestampHasBeenReported
              ? cachedLastDownBlockTimestamp
              : lastBlockTimestamp;

      elapsedTimeSincePastBlock = currentTimestamp - pastBlockTimestampToCheck;

      if (elapsedTimeSincePastBlock
          > lineaLivenessServiceConfiguration.maxBlockAgeSeconds().getSeconds()) {
        // only update the last late block timestamp if it was reported
        // otherwise, keeps it as is to ensure it will be reported this round
        if (lastDownBlockTimestampHasBeenReported) {
          this.lastDownBlockTimestamp.set(lastBlockTimestamp);
        }
        shouldBuild = true;
      }
    }

    log.atDebug()
        .setMessage(
            "targetBlockNumber={} lastBlockTimestamp={} cachedLastBlockTimestamp={}"
                + " cachedLastDownBlockTimestamp={} cachedLastReportedDownBlockTimestamp={} lastDownBlockTimestamp={}"
                + " currentTimestamp={} pastBlockTimestampToCheck={} elapsedTimeSincePastBlock={} shouldBuild={}")
        .addArgument(targetBlockNumber)
        .addArgument(lastBlockTimestamp)
        .addArgument(cachedLastBlockTimestamp)
        .addArgument(cachedLastDownBlockTimestamp)
        .addArgument(cachedLastReportedDownBlockTimestamp)
        .addArgument(lastDownBlockTimestamp.get())
        .addArgument(currentTimestamp)
        .addArgument(pastBlockTimestampToCheck)
        .addArgument(elapsedTimeSincePastBlock)
        .addArgument(shouldBuild)
        .log();

    this.lastBlockTimestamp.set(lastBlockTimestamp);

    return shouldBuild;
  }

  /** Checks the timestamp of the last block and reports downtime if necessary. */
  @Override
  public Optional<TransactionBundle> checkBlockTimestampAndBuildBundle(
      long currentTimestamp, long lastBlockTimestamp, long targetBlockNumber) {
    if (!lineaLivenessServiceConfiguration.enabled()) return Optional.empty();

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
      if (this.metricCategoryRegistry.isMetricCategoryEnabled(SEQUENCER_LIVENESS)) {
        uptimeTransactionUpCount.incrementAndGet();
        uptimeTransactionDownCount.incrementAndGet();
        uptimeTransactionsCounter.inc(2);
      }
      lastReportedDownBlockTimestamp.set(blockTimestamp);
    } else {
      if (this.metricCategoryRegistry.isMetricCategoryEnabled(SEQUENCER_LIVENESS)) {
        transactionFailureCounter.inc(2);
      }
    }
  }
}
