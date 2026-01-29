/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.sequencer.txselection;

import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.PLUGIN_SELECTION_TIMEOUT;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.PLUGIN_SELECTION_TIMEOUT_INVALID_TX;
import static org.hyperledger.besu.plugin.data.TransactionSelectionResult.SELECTION_CANCELLED;

import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicReference;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bl.TransactionProfitabilityCalculator;
import net.consensys.linea.bundles.BundlePoolService;
import net.consensys.linea.bundles.TransactionBundle;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.config.LineaTransactionSelectorConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.metrics.HistogramMetrics;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.forced.ForcedTransactionPoolService;
import net.consensys.linea.sequencer.liveness.LivenessService;
import net.consensys.linea.sequencer.txselection.selectors.LineaTransactionSelector;
import net.consensys.linea.sequencer.txselection.selectors.TransactionEventFilter;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.PendingTransaction;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.txselection.BlockTransactionSelectionService;
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector;
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelectorFactory;
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager;

/**
 * Represents a factory for creating transaction selectors. Note that a new instance of the
 * transaction selector is created everytime a new block creation time is started.
 *
 * <p>Also provides an entrypoint for bundle transactions
 */
@Slf4j
public class LineaTransactionSelectorFactory implements PluginTransactionSelectorFactory {
  private final BlockchainService blockchainService;
  private final Optional<JsonRpcManager> rejectedTxJsonRpcManager;
  private final LineaTransactionSelectorConfiguration txSelectorConfiguration;
  private final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration;
  private final LineaProfitabilityConfiguration profitabilityConfiguration;
  private final LineaTracerConfiguration tracerConfiguration;
  private final Optional<HistogramMetrics> maybeProfitabilityMetrics;
  private final BundlePoolService bundlePoolService;
  private final ForcedTransactionPoolService forcedTransactionPoolService;
  private final Optional<LivenessService> livenessService;
  private final InvalidTransactionByLineCountCache invalidTransactionByLineCountCache;
  private final AtomicReference<LineaTransactionSelector> currSelector = new AtomicReference<>();
  private final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedEvents;
  private final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedBundleEvents;
  private final AtomicReference<Set<Address>> deniedAddresses;
  private final AtomicBoolean isSelectionInterrupted = new AtomicBoolean(false);
  private final TransactionProfitabilityCalculator transactionProfitabilityCalculator;

  public LineaTransactionSelectorFactory(
      final BlockchainService blockchainService,
      final LineaTransactionSelectorConfiguration txSelectorConfiguration,
      final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration,
      final LineaProfitabilityConfiguration profitabilityConfiguration,
      final LineaTracerConfiguration tracerConfiguration,
      final Optional<LivenessService> livenessService,
      final Optional<JsonRpcManager> rejectedTxJsonRpcManager,
      final Optional<HistogramMetrics> maybeProfitabilityMetrics,
      final BundlePoolService bundlePoolService,
      final ForcedTransactionPoolService forcedTransactionPoolService,
      final InvalidTransactionByLineCountCache invalidTransactionByLineCountCache,
      final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedEvents,
      final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedBundleEvents,
      final AtomicReference<Set<Address>> deniedAddresses,
      final TransactionProfitabilityCalculator transactionProfitabilityCalculator) {
    this.blockchainService = blockchainService;
    this.txSelectorConfiguration = txSelectorConfiguration;
    this.l1L2BridgeConfiguration = l1L2BridgeConfiguration;
    this.profitabilityConfiguration = profitabilityConfiguration;
    this.tracerConfiguration = tracerConfiguration;
    this.rejectedTxJsonRpcManager = rejectedTxJsonRpcManager;
    this.maybeProfitabilityMetrics = maybeProfitabilityMetrics;
    this.bundlePoolService = bundlePoolService;
    this.forcedTransactionPoolService = forcedTransactionPoolService;
    this.livenessService = livenessService;
    this.invalidTransactionByLineCountCache = invalidTransactionByLineCountCache;
    this.deniedEvents = deniedEvents;
    this.deniedBundleEvents = deniedBundleEvents;
    this.deniedAddresses = deniedAddresses;
    this.transactionProfitabilityCalculator = transactionProfitabilityCalculator;
  }

  @Override
  public PluginTransactionSelector create(final SelectorsStateManager selectorsStateManager) {
    final var selector =
        new LineaTransactionSelector(
            selectorsStateManager,
            blockchainService,
            txSelectorConfiguration,
            l1L2BridgeConfiguration,
            profitabilityConfiguration,
            tracerConfiguration,
            rejectedTxJsonRpcManager,
            maybeProfitabilityMetrics,
            invalidTransactionByLineCountCache,
            deniedEvents,
            deniedBundleEvents,
            deniedAddresses,
            transactionProfitabilityCalculator);
    currSelector.set(selector);
    return selector;
  }

  @Override
  public void selectPendingTransactions(
      final BlockTransactionSelectionService bts,
      final ProcessableBlockHeader pendingBlockHeader,
      final List<? extends PendingTransaction> candidatePendingTransactions) {
    try {
      final boolean livenessTransactionSelected =
          checkAndSendLivenessBundle(bts, pendingBlockHeader.getNumber());

      if (!livenessTransactionSelected) {
        forcedTransactionPoolService.processForBlock(pendingBlockHeader.getNumber(), bts);
      }

      final var bundlesByBlockNumber =
          bundlePoolService.getBundlesByBlockNumber(pendingBlockHeader.getNumber());

      log.atDebug()
          .setMessage("Bundle pool stats: total={}, for block #{}={}")
          .addArgument(bundlePoolService::size)
          .addArgument(pendingBlockHeader::getNumber)
          .addArgument(bundlesByBlockNumber::size)
          .log();

      final var selectionStartedAt = System.nanoTime();

      bundlesByBlockNumber.stream()
          .takeWhile(unused -> !isSelectionInterrupted.get())
          .forEach(
              bundle -> {
                final var bundleStartedAt = System.nanoTime();
                log.trace("Starting evaluation of bundle {}", bundle.bundleIdentifier());

                var maybeBadBundleRes =
                    bundle.pendingTransactions().stream()
                        .map(bts::evaluatePendingTransaction)
                        .filter(evalRes -> !evalRes.selected())
                        .findFirst();

                final var now = System.nanoTime();
                final var cumulativeBundleSelectionTime = now - selectionStartedAt;
                final var currentBundleSelectionTime = now - bundleStartedAt;

                if (maybeBadBundleRes.isPresent()) {
                  final var notSelectedReason = maybeBadBundleRes.get();

                  if (isSelectionInterrupted(notSelectedReason)) {
                    isSelectionInterrupted.set(true);
                    log.atDebug()
                        .setMessage(
                            "Bundle selection interrupted while processing bundle {},"
                                + " elapsed time: current bundle {}ms, cumulative {}ms")
                        .addArgument(bundle::bundleIdentifier)
                        .addArgument(() -> nanosToMillis(currentBundleSelectionTime))
                        .addArgument(() -> nanosToMillis(cumulativeBundleSelectionTime))
                        .log();
                  } else {
                    log.atDebug()
                        .setMessage(
                            "Failed bundle {}, reason {}, elapsed time: current bundle {}ms, cumulative {}ms")
                        .addArgument(bundle::bundleIdentifier)
                        .addArgument(notSelectedReason)
                        .addArgument(() -> nanosToMillis(currentBundleSelectionTime))
                        .addArgument(() -> nanosToMillis(cumulativeBundleSelectionTime))
                        .log();
                  }

                  rollback(bts);
                } else {
                  log.atDebug()
                      .setMessage(
                          "Selected bundle {}, elapsed time: current bundle {}ms, cumulative {}ms")
                      .addArgument(bundle::bundleIdentifier)
                      .addArgument(() -> nanosToMillis(currentBundleSelectionTime))
                      .addArgument(() -> nanosToMillis(cumulativeBundleSelectionTime))
                      .log();

                  commit(bts);
                }
              });
    } finally {
      currSelector.set(null);
      isSelectionInterrupted.set(false);
    }
  }

  /**
   * Checks if a liveness bundle should be sent and evaluates it.
   *
   * @param bts the block transaction selection service
   * @param pendingBlockNumber the pending block number
   * @return true if a liveness transaction was selected, false otherwise
   */
  private boolean checkAndSendLivenessBundle(
      BlockTransactionSelectionService bts, long pendingBlockNumber) {
    if (livenessService.isEmpty()) {
      return false;
    }
    final var livenessService = this.livenessService.get();
    final long headBlockTimestamp = blockchainService.getChainHeadHeader().getTimestamp();

    Optional<TransactionBundle> livenessBundle =
        livenessService.checkBlockTimestampAndBuildBundle(
            Instant.now().getEpochSecond(), headBlockTimestamp, pendingBlockNumber);

    if (livenessBundle.isEmpty()) {
      return false;
    }

    log.trace("Starting evaluation of liveness bundle {}", livenessBundle.get());
    var badBundleRes =
        livenessBundle.get().pendingTransactions().stream()
            .map(bts::evaluatePendingTransaction)
            .filter(evalRes -> !evalRes.selected())
            .findFirst();

    if (badBundleRes.isPresent()) {
      final var notSelectedReason = badBundleRes.get();

      if (isSelectionInterrupted(notSelectedReason)) {
        isSelectionInterrupted.set(true);
        log.debug(
            "Bundle selection interrupted while processing liveness bundle {}, reason {}",
            livenessBundle.get(),
            notSelectedReason);
      } else {
        log.debug("Failed liveness bundle {}, reason {}", livenessBundle.get(), notSelectedReason);
      }
      livenessService.updateUptimeMetrics(false, headBlockTimestamp);
      rollback(bts);
      return false;
    } else {
      log.debug("Selected liveness bundle {}", livenessBundle.get());
      livenessService.updateUptimeMetrics(true, headBlockTimestamp);
      commit(bts);
      return true;
    }
  }

  private void commit(final BlockTransactionSelectionService bts) {
    currSelector.get().getOperationTracer().commitTransactionBundle();
    bts.commit();
  }

  private void rollback(final BlockTransactionSelectionService bts) {
    currSelector.get().getOperationTracer().popTransactionBundle();
    bts.rollback();
  }

  private long nanosToMillis(final long nanos) {
    return TimeUnit.NANOSECONDS.toMillis(nanos);
  }

  private boolean isSelectionInterrupted(final TransactionSelectionResult selectionResult) {
    return selectionResult.equals(PLUGIN_SELECTION_TIMEOUT)
        || selectionResult.equals(PLUGIN_SELECTION_TIMEOUT_INVALID_TX)
        || selectionResult.equals(SELECTION_CANCELLED);
  }
}
