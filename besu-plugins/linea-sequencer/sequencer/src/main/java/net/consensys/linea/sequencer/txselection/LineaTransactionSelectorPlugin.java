/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.sequencer.txselection;

import static net.consensys.linea.metrics.LineaMetricCategory.SEQUENCER_LIVENESS;
import static net.consensys.linea.metrics.LineaMetricCategory.SEQUENCER_PROFITABILITY;

import com.google.auto.service.AutoService;
import java.math.BigInteger;
import java.util.Collections;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.atomic.AtomicReference;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.AbstractLineaRequiredPlugin;
import net.consensys.linea.config.LineaRejectedTxReportingConfiguration;
import net.consensys.linea.config.LineaTransactionPoolValidatorCliOptions;
import net.consensys.linea.config.LineaTransactionSelectorCliOptions;
import net.consensys.linea.config.LineaTransactionSelectorConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.metrics.HistogramMetrics;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.liveness.LineaLivenessService;
import net.consensys.linea.sequencer.liveness.LineaLivenessTxBuilder;
import net.consensys.linea.sequencer.liveness.LivenessService;
import net.consensys.linea.sequencer.txselection.selectors.ProfitableTransactionSelector;
import net.consensys.linea.sequencer.txselection.selectors.TransactionEventFilter;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.TransactionSelectionService;

/**
 * This class extends the default transaction selection rules used by Besu. It leverages the
 * TransactionSelectionService to manage and customize the process of transaction selection. This
 * includes setting limits such as 'TraceLineLimit', 'maxBlockGas', and 'maxCallData'.
 */
@Slf4j
@AutoService(BesuPlugin.class)
public class LineaTransactionSelectorPlugin extends AbstractLineaRequiredPlugin {
  private TransactionSelectionService transactionSelectionService;
  private Optional<JsonRpcManager> rejectedTxJsonRpcManager = Optional.empty();
  private final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedEvents =
      new AtomicReference<>(Collections.emptyMap());
  private final AtomicReference<Map<Address, Set<TransactionEventFilter>>> deniedBundleEvents =
      new AtomicReference<>(Collections.emptyMap());
  private final AtomicReference<Set<Address>> deniedAddresses =
      new AtomicReference<>(Collections.emptySet());

  @Override
  public void doRegister(final ServiceManager serviceManager) {
    transactionSelectionService =
        serviceManager
            .getService(TransactionSelectionService.class)
            .orElseThrow(
                () ->
                    new RuntimeException(
                        "Failed to obtain TransactionSelectionService from the ServiceManager."));

    metricCategoryRegistry.addMetricCategory(SEQUENCER_PROFITABILITY);
    metricCategoryRegistry.addMetricCategory(SEQUENCER_LIVENESS);
  }

  @Override
  public void doStart() {
    if (l1L2BridgeSharedConfiguration().equals(LineaL1L2BridgeSharedConfiguration.TEST_DEFAULT)) {
      throw new IllegalArgumentException("L1L2 bridge settings have not been defined.");
    }

    final LineaTransactionSelectorConfiguration txSelectorConfiguration =
        transactionSelectorConfiguration();

    final LineaRejectedTxReportingConfiguration lineaRejectedTxReportingConfiguration =
        rejectedTxReportingConfiguration();
    rejectedTxJsonRpcManager =
        Optional.ofNullable(lineaRejectedTxReportingConfiguration.rejectedTxEndpoint())
            .map(
                endpoint ->
                    new JsonRpcManager(
                            "linea-tx-selector-plugin",
                            besuConfiguration.getDataPath(),
                            lineaRejectedTxReportingConfiguration)
                        .start());

    final Optional<HistogramMetrics> maybeProfitabilityMetrics =
        metricCategoryRegistry.isMetricCategoryEnabled(SEQUENCER_PROFITABILITY)
            ? Optional.of(
                new HistogramMetrics(
                    metricsSystem,
                    SEQUENCER_PROFITABILITY,
                    "ratio",
                    "sequencer profitability ratio",
                    profitabilityConfiguration().profitabilityMetricsBuckets(),
                    ProfitableTransactionSelector.Phase.class))
            : Optional.empty();

    final BigInteger chainId =
        blockchainService
            .getChainId()
            .orElseThrow(
                () -> new RuntimeException("Failed to get chain Id from the BlockchainService."));
    final Optional<LivenessService> livenessService =
        livenessServiceConfiguration().enabled()
            ? Optional.of(
                new LineaLivenessService(
                    livenessServiceConfiguration(),
                    rpcEndpointService,
                    new LineaLivenessTxBuilder(
                        livenessServiceConfiguration(), blockchainService, chainId),
                    metricCategoryRegistry,
                    metricsSystem))
            : Optional.empty();

    deniedEvents.set(txSelectorConfiguration.eventsDenyList());
    deniedBundleEvents.set(txSelectorConfiguration.eventsBundleDenyList());
    deniedAddresses.set(transactionPoolValidatorConfiguration().deniedAddresses());

    transactionSelectionService.registerPluginTransactionSelectorFactory(
        new LineaTransactionSelectorFactory(
            blockchainService,
            txSelectorConfiguration,
            l1L2BridgeSharedConfiguration(),
            profitabilityConfiguration(),
            tracerConfiguration(),
            livenessService,
            rejectedTxJsonRpcManager,
            maybeProfitabilityMetrics,
            bundlePoolService,
            getInvalidTransactionByLineCountCache(),
            deniedEvents,
            deniedBundleEvents,
            deniedAddresses,
            transactionProfitabilityCalculator));
  }

  @Override
  public void stop() {
    super.stop();
    rejectedTxJsonRpcManager.ifPresent(JsonRpcManager::shutdown);
  }

  @Override
  public CompletableFuture<Void> reloadConfiguration() {
    try {
      Map<Address, Set<TransactionEventFilter>> newDeniedEvents =
          LineaTransactionSelectorCliOptions.create()
              .parseTransactionEventDenyList(
                  transactionSelectorConfiguration().eventsDenyListPath());
      deniedEvents.set(newDeniedEvents);

      Map<Address, Set<TransactionEventFilter>> newDeniedBundleEvents =
          LineaTransactionSelectorCliOptions.create()
              .parseTransactionEventDenyList(
                  transactionSelectorConfiguration().eventsBundleDenyListPath());
      deniedBundleEvents.set(newDeniedBundleEvents);
      Set<Address> newDeniedAddresses =
          LineaTransactionPoolValidatorCliOptions.create()
              .parseDeniedAddresses(transactionPoolValidatorConfiguration().denyListPath());
      deniedAddresses.set(newDeniedAddresses);
      return CompletableFuture.completedFuture(null);
    } catch (Exception e) {
      return CompletableFuture.failedFuture(e);
    }
  }
}
