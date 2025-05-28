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

import java.util.Map;
import java.util.Optional;
import java.util.concurrent.atomic.AtomicReference;

import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bundles.BundlePoolService;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.config.LineaTransactionSelectorConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.metrics.HistogramMetrics;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.txselection.selectors.LineaTransactionSelector;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
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
  private final Map<String, Integer> limitsMap;
  private final AtomicReference<LineaTransactionSelector> currSelector = new AtomicReference<>();

  public LineaTransactionSelectorFactory(
      final BlockchainService blockchainService,
      final LineaTransactionSelectorConfiguration txSelectorConfiguration,
      final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration,
      final LineaProfitabilityConfiguration profitabilityConfiguration,
      final LineaTracerConfiguration tracerConfiguration,
      final Map<String, Integer> limitsMap,
      final Optional<JsonRpcManager> rejectedTxJsonRpcManager,
      final Optional<HistogramMetrics> maybeProfitabilityMetrics,
      final BundlePoolService bundlePoolService) {
    this.blockchainService = blockchainService;
    this.txSelectorConfiguration = txSelectorConfiguration;
    this.l1L2BridgeConfiguration = l1L2BridgeConfiguration;
    this.profitabilityConfiguration = profitabilityConfiguration;
    this.tracerConfiguration = tracerConfiguration;
    this.limitsMap = limitsMap;
    this.rejectedTxJsonRpcManager = rejectedTxJsonRpcManager;
    this.maybeProfitabilityMetrics = maybeProfitabilityMetrics;
    this.bundlePoolService = bundlePoolService;
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
            bundlePoolService,
            limitsMap,
            rejectedTxJsonRpcManager,
            maybeProfitabilityMetrics);
    currSelector.set(selector);
    return selector;
  }

  public void selectPendingTransactions(
      final BlockTransactionSelectionService bts, final ProcessableBlockHeader pendingBlockHeader) {
    final var bundlesByBlockNumber =
        bundlePoolService.getBundlesByBlockNumber(pendingBlockHeader.getNumber());

    log.atDebug()
        .setMessage("Bundle pool stats: total={}, for block #{}={}")
        .addArgument(bundlePoolService::size)
        .addArgument(pendingBlockHeader::getNumber)
        .addArgument(bundlesByBlockNumber::size)
        .log();

    bundlesByBlockNumber.forEach(
        bundle -> {
          log.trace("Starting evaluation of bundle {}", bundle);
          var badBundleRes =
              bundle.pendingTransactions().stream()
                  .map(bts::evaluatePendingTransaction)
                  .filter(evalRes -> !evalRes.selected())
                  .findFirst();

          if (badBundleRes.isPresent()) {
            log.trace("Failed bundle {}, reason {}", bundle, badBundleRes);
            rollback(bts);
          } else {
            log.trace("Selected bundle {}", bundle);
            commit(bts);
          }
        });
    currSelector.set(null);
  }

  private void commit(final BlockTransactionSelectionService bts) {
    currSelector.get().getOperationTracer().commitTransactionBundle();
    bts.commit();
  }

  private void rollback(final BlockTransactionSelectionService bts) {
    currSelector.get().getOperationTracer().popTransactionBundle();
    bts.rollback();
  }
}
