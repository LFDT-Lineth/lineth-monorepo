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

package net.consensys.linea.sequencer.txpoolvalidation;

import static net.consensys.linea.metrics.LineaMetricCategory.TX_POOL_PROFITABILITY;
import static net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator.createLimitModules;

import java.io.File;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Collectors;
import java.util.stream.Stream;

import com.google.auto.service.AutoService;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.AbstractLineaRequiredPlugin;
import net.consensys.linea.config.LineaRejectedTxReportingConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.txpoolvalidation.metrics.TransactionPoolProfitabilityMetrics;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.BesuEvents;
import org.hyperledger.besu.plugin.services.TransactionPoolValidatorService;
import org.hyperledger.besu.plugin.services.TransactionSimulationService;
import org.hyperledger.besu.plugin.services.transactionpool.TransactionPoolService;

/**
 * This class extends the default transaction validation rules for adding transactions to the
 * transaction pool. It leverages the PluginTransactionValidatorService to manage and customize the
 * process of transaction validation. This includes, for example, setting a deny list of addresses
 * that are not allowed to add transactions to the pool.
 */
@Slf4j
@AutoService(BesuPlugin.class)
public class LineaTransactionPoolValidatorPlugin extends AbstractLineaRequiredPlugin {
  private ServiceManager serviceManager;
  private TransactionPoolValidatorService transactionPoolValidatorService;
  private TransactionSimulationService transactionSimulationService;
  private Optional<JsonRpcManager> rejectedTxJsonRpcManager = Optional.empty();

  @Override
  public void doRegister(final ServiceManager serviceManager) {
    this.serviceManager = serviceManager;

    transactionPoolValidatorService =
        serviceManager
            .getService(TransactionPoolValidatorService.class)
            .orElseThrow(
                () ->
                    new RuntimeException(
                        "Failed to obtain TransactionPoolValidationService from the ServiceManager."));

    transactionSimulationService =
        serviceManager
            .getService(TransactionSimulationService.class)
            .orElseThrow(
                () ->
                    new RuntimeException(
                        "Failed to obtain TransactionSimulatorService from the ServiceManager."));

    metricCategoryRegistry.addMetricCategory(TX_POOL_PROFITABILITY);
  }

  @Override
  public void doStart() {
    if (l1L2BridgeSharedConfiguration().equals(LineaL1L2BridgeSharedConfiguration.TEST_DEFAULT)) {
      throw new IllegalArgumentException("L1L2 bridge settings have not been defined.");
    }

    try (Stream<String> lines =
        Files.lines(
            Path.of(new File(transactionPoolValidatorConfiguration().denyListPath()).toURI()))) {
      final Set<Address> deniedAddresses =
          lines.map(l -> Address.fromHexString(l.trim())).collect(Collectors.toUnmodifiableSet());

      // start the optional json rpc manager for rejected tx reporting
      final LineaRejectedTxReportingConfiguration lineaRejectedTxReportingConfiguration =
          rejectedTxReportingConfiguration();
      rejectedTxJsonRpcManager =
          Optional.ofNullable(lineaRejectedTxReportingConfiguration.rejectedTxEndpoint())
              .map(
                  endpoint ->
                      new JsonRpcManager(
                              "linea-tx-pool-validator-plugin",
                              besuConfiguration.getDataPath(),
                              lineaRejectedTxReportingConfiguration)
                          .start());

      transactionPoolValidatorService.registerPluginTransactionValidatorFactory(
          new LineaTransactionPoolValidatorFactory(
              besuConfiguration,
              blockchainService,
              transactionSimulationService,
              transactionPoolValidatorConfiguration(),
              profitabilityConfiguration(),
              deniedAddresses,
              createLimitModules(tracerConfiguration()),
              l1L2BridgeSharedConfiguration(),
              rejectedTxJsonRpcManager));

      if (metricCategoryRegistry.isMetricCategoryEnabled(TX_POOL_PROFITABILITY)) {
        final var besuEventsService =
            serviceManager
                .getService(BesuEvents.class)
                .orElseThrow(
                    () ->
                        new RuntimeException(
                            "Failed to obtain BesuEvents from the ServiceManager."));

        final var transactionPoolService =
            serviceManager
                .getService(TransactionPoolService.class)
                .orElseThrow(
                    () ->
                        new RuntimeException(
                            "Failed to obtain TransactionPoolService from the ServiceManager."));

        final var transactionPoolProfitabilityMetrics =
            new TransactionPoolProfitabilityMetrics(
                besuConfiguration,
                metricsSystem,
                profitabilityConfiguration(),
                transactionPoolService,
                blockchainService);

        besuEventsService.addBlockAddedListener(
            addedBlockContext -> {
              try {
                // on new block let's calculate profitability for every txs in the pool
                transactionPoolProfitabilityMetrics.update();
              } catch (final Exception e) {
                log.warn(
                    "Error calculating transaction profitability for block {}({})",
                    addedBlockContext.getBlockHeader().getNumber(),
                    addedBlockContext.getBlockHeader().getBlockHash(),
                    e);
              }
            });
      }

    } catch (Exception e) {
      throw new RuntimeException(e);
    }
  }

  @Override
  public void stop() {
    super.stop();
    rejectedTxJsonRpcManager.ifPresent(JsonRpcManager::shutdown);
  }
}
