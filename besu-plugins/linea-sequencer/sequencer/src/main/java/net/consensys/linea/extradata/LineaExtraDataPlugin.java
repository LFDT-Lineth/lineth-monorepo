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

package net.consensys.linea.extradata;

import static net.consensys.linea.metrics.LineaMetricCategory.PRICING_CONF;

import java.util.concurrent.atomic.AtomicBoolean;

import com.google.auto.service.AutoService;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.AbstractLineaRequiredPlugin;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.data.AddedBlockContext;
import org.hyperledger.besu.plugin.services.BesuEvents;
import org.hyperledger.besu.plugin.services.BesuEvents.InitialSyncCompletionListener;

/** This plugin registers handlers that are activated when new blocks are imported */
@Slf4j
@AutoService(BesuPlugin.class)
public class LineaExtraDataPlugin extends AbstractLineaRequiredPlugin {
  private ServiceManager serviceManager;

  @Override
  public void doRegister(final ServiceManager context) {
    serviceManager = context;

    metricCategoryRegistry.addMetricCategory(PRICING_CONF);
  }

  /**
   * Starts this plugin and in case the extra data pricing is enabled, as first thing it tries to
   * extract extra data pricing configuration from the chain head, then it starts listening for new
   * imported block, in order to update the extra data pricing on every incoming block.
   */
  @Override
  public void doStart() {
    if (profitabilityConfiguration().extraDataPricingEnabled()) {
      final var besuEventsService =
          serviceManager
              .getService(BesuEvents.class)
              .orElseThrow(
                  () ->
                      new RuntimeException("Failed to obtain BesuEvents from the ServiceManager."));

      // assume that we are in sync by default to support reading extra data at genesis
      final AtomicBoolean inSync = new AtomicBoolean(true);

      besuEventsService.addSyncStatusListener(
          maybeSyncStatus ->
              inSync.set(
                  maybeSyncStatus
                      .map(
                          syncStatus ->
                              syncStatus.getHighestBlock() - syncStatus.getCurrentBlock() < 5)
                      .orElse(true)));

      // wait for the initial sync phase to complete before starting parsing extra data
      // to avoid parsing errors
      besuEventsService.addInitialSyncCompletionListener(
          new InitialSyncCompletionListener() {
            long blockAddedListenerId = -1;

            @Override
            public synchronized void onInitialSyncCompleted() {
              blockAddedListenerId = enableExtraDataHandling(besuEventsService, inSync);
            }

            @Override
            public synchronized void onInitialSyncRestart() {
              besuEventsService.removeBlockAddedListener(blockAddedListenerId);
              blockAddedListenerId = -1;
            }
          });
    }

    if (metricCategoryRegistry.isMetricCategoryEnabled(PRICING_CONF)) {
      initMetrics(profitabilityConfiguration());
    }
  }

  private void initMetrics(final LineaProfitabilityConfiguration lineaProfitabilityConfiguration) {
    final var confLabelledGauge =
        metricsSystem.createLabelledSuppliedGauge(
            PRICING_CONF, "values", "Profitability configuration values at runtime", "field");
    confLabelledGauge.labels(lineaProfitabilityConfiguration::fixedCostWei, "fixed_cost_wei");
    confLabelledGauge.labels(lineaProfitabilityConfiguration::variableCostWei, "variable_cost_wei");
    confLabelledGauge.labels(lineaProfitabilityConfiguration::ethGasPriceWei, "eth_gas_price_wei");
  }

  private long enableExtraDataHandling(
      final BesuEvents besuEventsService, final AtomicBoolean inSync) {

    final var extraDataHandler =
        new LineaExtraDataHandler(rpcEndpointService, profitabilityConfiguration());

    if (inSync.get()) {
      final var chainHeadHeader = blockchainService.getChainHeadHeader();
      final var initialExtraData = chainHeadHeader.getExtraData();
      try {
        extraDataHandler.handle(initialExtraData);
      } catch (final Exception e) {
        // this could normally happen if for example the genesis block has not a valid extra data
        // field so we keep this log at debug
        log.debug(
            "Failed setting initial pricing conf from extra data field ({}) of the chain head block {}({})",
            initialExtraData,
            chainHeadHeader.getNumber(),
            chainHeadHeader.getBlockHash(),
            e);
      }
    }

    return besuEventsService.addBlockAddedListener(
        addedBlockContext -> {
          if (inSync.get()) {
            processNewBlock(extraDataHandler, addedBlockContext);
          }
        });
  }

  private void processNewBlock(
      final LineaExtraDataHandler extraDataHandler, final AddedBlockContext addedBlockContext) {
    final var importedBlockHeader = addedBlockContext.getBlockHeader();
    final var latestExtraData = importedBlockHeader.getExtraData();

    try {
      extraDataHandler.handle(latestExtraData);
    } catch (final Exception e) {
      log.warn(
          "Failed setting pricing conf from extra data field ({}) of latest imported block {}({})",
          latestExtraData,
          importedBlockHeader.getNumber(),
          importedBlockHeader.getBlockHash(),
          e);
    }
  }
}
