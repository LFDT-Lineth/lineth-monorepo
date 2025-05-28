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

package net.consensys.linea.sequencer.txselection;

import static net.consensys.linea.metrics.LineaMetricCategory.SEQUENCER_PROFITABILITY;
import static net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator.createLimitModules;

import java.util.Optional;

import com.google.auto.service.AutoService;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.AbstractLineaRequiredPlugin;
import net.consensys.linea.config.LineaRejectedTxReportingConfiguration;
import net.consensys.linea.config.LineaTransactionSelectorConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.metrics.HistogramMetrics;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.txselection.selectors.ProfitableTransactionSelector;
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

    transactionSelectionService.registerPluginTransactionSelectorFactory(
        new LineaTransactionSelectorFactory(
            blockchainService,
            txSelectorConfiguration,
            l1L2BridgeSharedConfiguration(),
            profitabilityConfiguration(),
            tracerConfiguration(),
            createLimitModules(tracerConfiguration()),
            rejectedTxJsonRpcManager,
            maybeProfitabilityMetrics,
            bundlePoolService));
  }

  @Override
  public void stop() {
    super.stop();
    rejectedTxJsonRpcManager.ifPresent(JsonRpcManager::shutdown);
  }
}
