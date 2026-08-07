/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package lineth.sequencer.txselection;

import static lineth.metrics.LineaMetricCategory.SEQUENCER_FORCED_TX;
import static lineth.metrics.LineaMetricCategory.SEQUENCER_LIVENESS;
import static lineth.metrics.LineaMetricCategory.SEQUENCER_PROFITABILITY;

import com.google.auto.service.AutoService;
import java.math.BigInteger;
import java.util.Optional;
import linea.blob.BlobCompressorSelectorByTimestamp;
import lombok.extern.slf4j.Slf4j;
import lineth.AbstractLineaRequiredPlugin;
import lineth.config.LineaRejectedTxReportingConfiguration;
import lineth.config.LineaTransactionSelectorConfiguration;
import lineth.jsonrpc.JsonRpcManager;
import lineth.metrics.HistogramMetrics;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import lineth.sequencer.liveness.LineaLivenessService;
import lineth.sequencer.liveness.LineaLivenessTxBuilder;
import lineth.sequencer.liveness.LivenessService;
import lineth.sequencer.txselection.selectors.ProfitableTransactionSelector;
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
    metricCategoryRegistry.addMetricCategory(SEQUENCER_FORCED_TX);
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

    // blobCompressor is initialised in AbstractLineaSharedPrivateOptionsPlugin with the effective
    // limit. Only pass it to the factory when a blob size limit is explicitly configured so that
    // CompressionAwareTransactionSelector is only active when intentionally enabled.
    final BlobCompressorSelectorByTimestamp blobCompressorSelector =
        txSelectorConfiguration.blobSizeLimit() != null ? blobCompressorSelectorByTimestamp : null;

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
            forcedTransactionPoolService,
            getInvalidTransactionByLineCountCache(),
            sharedDeniedEvents,
            sharedDeniedBundleEvents,
            sharedDeniedAddresses,
            transactionProfitabilityCalculator,
            transactionCompressor,
            blobCompressorSelector));
  }

  @Override
  public void stop() {
    super.stop();
    rejectedTxJsonRpcManager.ifPresent(JsonRpcManager::shutdown);
  }
}
