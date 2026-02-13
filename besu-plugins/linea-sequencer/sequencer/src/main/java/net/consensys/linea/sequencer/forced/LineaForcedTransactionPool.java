/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.forced;

import static net.consensys.linea.sequencer.forced.ForcedTransactionInclusionResult.BadBalance;
import static net.consensys.linea.sequencer.forced.ForcedTransactionInclusionResult.BadNonce;
import static net.consensys.linea.sequencer.forced.ForcedTransactionInclusionResult.BadPrecompile;
import static net.consensys.linea.sequencer.forced.ForcedTransactionInclusionResult.FilteredAddressFrom;
import static net.consensys.linea.sequencer.forced.ForcedTransactionInclusionResult.FilteredAddressTo;
import static net.consensys.linea.sequencer.forced.ForcedTransactionInclusionResult.Included;
import static net.consensys.linea.sequencer.forced.ForcedTransactionInclusionResult.Other;
import static net.consensys.linea.sequencer.forced.ForcedTransactionInclusionResult.TooManyLogs;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_FROM;
import static net.consensys.linea.sequencer.txselection.LineaTransactionSelectionResult.TX_FILTERED_ADDRESS_TO;
import static org.hyperledger.besu.plugin.data.AddedBlockContext.EventType.HEAD_ADVANCED;

import com.github.benmanes.caffeine.cache.Cache;
import com.github.benmanes.caffeine.cache.Caffeine;
import java.util.Arrays;
import java.util.Deque;
import java.util.EnumMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentLinkedDeque;
import java.util.concurrent.atomic.AtomicLong;
import java.util.stream.Collectors;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.metrics.LineaMetricCategory;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.ethereum.transaction.TransactionInvalidReason;
import org.hyperledger.besu.plugin.data.AddedBlockContext;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.BesuEvents;
import org.hyperledger.besu.plugin.services.MetricsSystem;
import org.hyperledger.besu.plugin.services.metrics.LabelledSuppliedMetric;
import org.hyperledger.besu.plugin.services.txselection.BlockTransactionSelectionService;

/**
 * Implementation of the forced transaction pool. Maintains a queue of pending forced transactions
 * and a cache of inclusion statuses.
 *
 * <p>Transactions are removed from the queue only when they are confirmed in a block (via
 * onBlockAdded), not during block building. This prevents transactions from being lost if block
 * selection is restarted before the block is sealed.
 */
@Slf4j
public class LineaForcedTransactionPool
    implements ForcedTransactionPoolService, BesuEvents.BlockAddedListener {

  public static final int DEFAULT_STATUS_CACHE_SIZE = 10_000;

  private final Deque<ForcedTransaction> pendingQueue = new ConcurrentLinkedDeque<>();
  private final Cache<Long, ForcedTransactionStatus> statusCache;
  private final Map<ForcedTransactionInclusionResult, AtomicLong> inclusionResultCounters =
      new EnumMap<>(ForcedTransactionInclusionResult.class);

  /**
   * Tracks tentative rejections during block building, separated by block number. Only rejections
   * are stored - inclusions are determined from actual block content in onBlockAdded. Outer map is
   * keyed by block number, inner map by forcedTransactionNumber. Rejections are only finalized when
   * onBlockAdded confirms the block. This ensures at most one rejection per block, even if
   * processForBlock is called multiple times for the same block number.
   *
   * <p>Separating by block number prevents race conditions between block building and block import
   * - we only clear rejections for blocks that have been confirmed, not for blocks still being
   * built.
   */
  private final Map<Long, Map<Long, TentativeRejection>> tentativeRejectionsByBlock =
      new ConcurrentHashMap<>();

  private record TentativeRejection(
      ForcedTransaction transaction, ForcedTransactionInclusionResult inclusionResult) {}

  public LineaForcedTransactionPool() {
    this(DEFAULT_STATUS_CACHE_SIZE, null, null);
  }

  public LineaForcedTransactionPool(final int statusCacheSize, final MetricsSystem metricsSystem) {
    this(statusCacheSize, metricsSystem, null);
  }

  public LineaForcedTransactionPool(
      final int statusCacheSize, final MetricsSystem metricsSystem, final BesuEvents besuEvents) {
    this.statusCache = Caffeine.newBuilder().maximumSize(statusCacheSize).build();

    for (ForcedTransactionInclusionResult result : ForcedTransactionInclusionResult.values()) {
      inclusionResultCounters.put(result, new AtomicLong(0));
    }

    if (metricsSystem != null) {
      initMetrics(metricsSystem);
    }

    if (besuEvents != null) {
      besuEvents.addBlockAddedListener(this);
    }
  }

  private void initMetrics(final MetricsSystem metricsSystem) {
    metricsSystem.createGauge(
        LineaMetricCategory.SEQUENCER_FORCED_TX,
        "pool_size",
        "Number of pending forced transactions in the pool",
        this::pendingCount);

    metricsSystem.createGauge(
        LineaMetricCategory.SEQUENCER_FORCED_TX,
        "status_cache_size",
        "Number of entries in the forced transaction status cache",
        () -> (int) statusCache.estimatedSize());

    final LabelledSuppliedMetric inclusionResultGauge =
        metricsSystem.createLabelledSuppliedGauge(
            LineaMetricCategory.SEQUENCER_FORCED_TX,
            "inclusion_result",
            "Total count of forced transactions by inclusion result",
            "result");

    for (ForcedTransactionInclusionResult result : ForcedTransactionInclusionResult.values()) {
      inclusionResultGauge.labels(
          () -> inclusionResultCounters.get(result).doubleValue(), result.name());
    }
  }

  @Override
  public void addForcedTransactions(final List<ForcedTransaction> transactions) {
    for (final ForcedTransaction tx : transactions) {
      pendingQueue.addLast(tx);
      log.atDebug()
          .setMessage("action=add_forced_tx forcedTxNumber={} txHash={} deadlineBlockNumber={}")
          .addArgument(tx::forcedTransactionNumber)
          .addArgument(tx.txHash()::toHexString)
          .addArgument(tx::deadlineBlockNumber)
          .log();
    }
    log.atInfo()
        .setMessage("action=add_forced_txs count={} pendingQueueSize={}")
        .addArgument(transactions::size)
        .addArgument(pendingQueue::size)
        .log();
  }

  @Override
  public void processForBlock(
      final long blockNumber,
      final BlockTransactionSelectionService blockTransactionSelectionService) {

    if (pendingQueue.isEmpty()) {
      return;
    }

    // Get or create the rejections map for this specific block.
    // We do NOT clear between iterations - rejections accumulate/overwrite.
    // This ensures we don't lose rejection reasons if getPayload returns a block from an earlier
    // iteration.
    // Note: We only track rejections, not selections. Inclusions are determined from actual block
    // content.
    final Map<Long, TentativeRejection> blockRejections =
        tentativeRejectionsByBlock.computeIfAbsent(blockNumber, k -> new ConcurrentHashMap<>());

    log.atDebug()
        .setMessage("action=process_forced_txs_start blockNumber={} pendingCount={}")
        .addArgument(blockNumber)
        .addArgument(pendingQueue::size)
        .log();

    int index = 0;

    for (final ForcedTransaction ftx : pendingQueue) {
      log.atTrace()
          .setMessage(
              "action=evaluate_forced_tx forcedTxNumber={} txHash={} index={} blockNumber={}")
          .addArgument(ftx::forcedTransactionNumber)
          .addArgument(ftx.txHash()::toHexString)
          .addArgument(index)
          .addArgument(blockNumber)
          .log();

      final PendingTransaction pendingTx =
          new org.hyperledger.besu.ethereum.eth.transactions.PendingTransaction.Local.Priority(
              ftx.transaction());

      final TransactionSelectionResult result =
          blockTransactionSelectionService.evaluatePendingTransaction(pendingTx);

      if (result.selected()) {
        // Selection is not stored - inclusion will be determined from actual block content
        log.atInfo()
            .setMessage(
                "action=forced_tx_tentatively_selected forcedTxNumber={} txHash={} blockNumber={} index={}")
            .addArgument(ftx::forcedTransactionNumber)
            .addArgument(ftx.txHash()::toHexString)
            .addArgument(blockNumber)
            .addArgument(index)
            .log();
        blockTransactionSelectionService.commit();
        index++;
      } else {
        final ForcedTransactionInclusionResult inclusionResult = mapToInclusionResult(result);
        final boolean isFinalRejection = index == 0 && !inclusionResult.shouldRetry();

        if (isFinalRejection) {
          // Final rejection - track tentatively, finalized on onBlockAdded
          blockRejections.put(
              ftx.forcedTransactionNumber(), new TentativeRejection(ftx, inclusionResult));
          log.atInfo()
              .setMessage(
                  "action=forced_tx_tentatively_rejected forcedTxNumber={} txHash={} blockNumber={} inclusionResult={}")
              .addArgument(ftx::forcedTransactionNumber)
              .addArgument(ftx.txHash()::toHexString)
              .addArgument(blockNumber)
              .addArgument(inclusionResult)
              .log();
        } else {
          // Retry - keep in queue, will try again next block (e.g. unknown rejection reason;
          // status is not recorded so getInclusionStatus returns absent)
          log.atInfo()
              .setMessage(
                  "action=forced_tx_retry_scheduled forcedTxNumber={} txHash={} blockNumber={} index={} inclusionResult={}")
              .addArgument(ftx::forcedTransactionNumber)
              .addArgument(ftx.txHash()::toHexString)
              .addArgument(blockNumber)
              .addArgument(index)
              .addArgument(inclusionResult)
              .log();
        }
        blockTransactionSelectionService.rollback();
        break; // Stop processing - only one invalidity proof per block
      }
    }

    log.atDebug()
        .setMessage(
            "action=process_forced_txs_end blockNumber={} remainingPending={} rejections={}")
        .addArgument(blockNumber)
        .addArgument(pendingQueue::size)
        .addArgument(blockRejections::size)
        .log();
  }

  @Override
  public void onBlockAdded(final AddedBlockContext addedBlockContext) {
    if (addedBlockContext.getEventType() != HEAD_ADVANCED) {
      return;
    }
    final long blockNumber = addedBlockContext.getBlockHeader().getNumber();
    final long blockTimestamp = addedBlockContext.getBlockHeader().getTimestamp();

    tentativeRejectionsByBlock.keySet().removeIf(bn -> bn < blockNumber);

    final Map<Long, TentativeRejection> blockRejections =
        tentativeRejectionsByBlock.remove(blockNumber);

    final Set<Hash> blockTxHashes =
        addedBlockContext.getBlockBody().getTransactions().stream()
            .map(Transaction::getHash)
            .collect(Collectors.toSet());

    final var iterator = pendingQueue.iterator();
    while (iterator.hasNext()) {
      final ForcedTransaction ftx = iterator.next();

      if (blockTxHashes.contains(ftx.txHash())) {
        iterator.remove();
        recordStatus(ftx, blockNumber, blockTimestamp, Included);
        log.atInfo()
            .setMessage("action=forced_tx_included forcedTxNumber={} txHash={} blockNumber={}")
            .addArgument(ftx::forcedTransactionNumber)
            .addArgument(ftx.txHash()::toHexString)
            .addArgument(blockNumber)
            .log();
      } else {
        final TentativeRejection rejection =
            blockRejections != null ? blockRejections.get(ftx.forcedTransactionNumber()) : null;

        if (rejection != null) {
          iterator.remove();
          recordStatus(ftx, blockNumber, blockTimestamp, rejection.inclusionResult());
          log.atInfo()
              .setMessage(
                  "action=forced_tx_rejected forcedTxNumber={} txHash={} blockNumber={} inclusionResult={}")
              .addArgument(ftx::forcedTransactionNumber)
              .addArgument(ftx.txHash()::toHexString)
              .addArgument(blockNumber)
              .addArgument(rejection::inclusionResult)
              .log();
        }
        // Whether rejected or not, stop processing - subsequent FTXs can't be in this block
        // (since FTXs are processed in order and this one wasn't included)
        break;
      }
    }
  }

  @Override
  public Optional<ForcedTransactionStatus> getInclusionStatus(final long forcedTransactionNumber) {
    return Optional.ofNullable(statusCache.getIfPresent(forcedTransactionNumber));
  }

  @Override
  public int pendingCount() {
    return pendingQueue.size();
  }

  private void recordStatus(
      final ForcedTransaction ftx,
      final long blockNumber,
      final long blockTimestamp,
      final ForcedTransactionInclusionResult result) {
    final ForcedTransactionStatus status =
        new ForcedTransactionStatus(
            ftx.forcedTransactionNumber(),
            ftx.txHash(),
            ftx.transaction().getSender(),
            blockNumber,
            blockTimestamp,
            result);
    statusCache.put(ftx.forcedTransactionNumber(), status);
    inclusionResultCounters.get(result).incrementAndGet();
  }

  private final Set<String> nonceErrors =
      buildReasonsStringSet(
          TransactionInvalidReason.NONCE_TOO_HIGH,
          TransactionInvalidReason.NONCE_TOO_LOW,
          TransactionInvalidReason.NONCE_OVERFLOW);

  private final Set<String> balanceErrors =
      buildReasonsStringSet(
          TransactionInvalidReason.UPFRONT_COST_EXCEEDS_BALANCE,
          TransactionInvalidReason.UPFRONT_COST_EXCEEDS_UINT256);

  /**
   * Maps a Besu TransactionSelectionResult to a ForcedTransactionInclusionResult.
   *
   * <p>First checks against known Linea-specific result constants using equality, then falls back
   * to string matching for standard Besu results. Returns Other for unrecognized results, which
   * triggers a retry in the next block.
   */
  private ForcedTransactionInclusionResult mapToInclusionResult(
      final TransactionSelectionResult result) {
    if (result.equals(TX_FILTERED_ADDRESS_FROM)) {
      return FilteredAddressFrom;
    }
    if (result.equals(TX_FILTERED_ADDRESS_TO)) {
      return FilteredAddressTo;
    }

    if (result.maybeInvalidReason().isPresent()) {
      final String invalidReason = result.maybeInvalidReason().get();
      if (nonceErrors.contains(invalidReason)) {
        return BadNonce;
      }

      if (balanceErrors.contains(invalidReason)) {
        return BadBalance;
      }

      if (invalidReason.equals("PRECOMPILE_RIPEMD_BLOCKS")
          || invalidReason.equals("PRECOMPILE_BLAKE_EFFECTIVE_CALLS")) {
        return BadPrecompile;
      }

      if (invalidReason.equals("BLOCK_L2_L1_LOGS")) {
        return TooManyLogs;
      }
    }

    log.atWarn()
        .setMessage("action=unknown_selection_result result={} willRetry=true")
        .addArgument(result.toString().toUpperCase(Locale.ROOT))
        .log();
    return Other;
  }

  private Set<String> buildReasonsStringSet(TransactionInvalidReason... reasons) {
    return Arrays.stream(reasons).map(TransactionInvalidReason::name).collect(Collectors.toSet());
  }
}
