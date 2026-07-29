/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.sequencer.txselection;

import static net.consensys.linea.metrics.LineaMetricCategory.SEQUENCER_FORCED_TX;
import static net.consensys.linea.metrics.LineaMetricCategory.SEQUENCER_LIVENESS;
import static net.consensys.linea.metrics.LineaMetricCategory.SEQUENCER_PROFITABILITY;

import com.google.auto.service.AutoService;
import java.math.BigInteger;
import java.util.Optional;
import linea.blob.BlobCompressorSelectorByTimestamp;
import linea.crypto.Secp256k1Signature;
import linea.crypto.Signer;
import linea.signing.NamedSignerProviderService;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.AbstractLineaRequiredPlugin;
import net.consensys.linea.config.LineaLivenessServiceConfiguration;
import net.consensys.linea.config.LineaRejectedTxReportingConfiguration;
import net.consensys.linea.config.LineaTransactionSelectorConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.metrics.HistogramMetrics;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.liveness.LineaLivenessService;
import net.consensys.linea.sequencer.liveness.LineaLivenessTxBuilder;
import net.consensys.linea.sequencer.liveness.LivenessService;
import net.consensys.linea.sequencer.liveness.Web3SignerDigestSigner;
import net.consensys.linea.sequencer.txselection.selectors.ProfitableTransactionSelector;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.TransactionSelectionService;
import org.web3j.crypto.Keys;
import org.web3j.utils.Numeric;

/**
 * This class extends the default transaction selection rules used by Besu. It leverages the
 * TransactionSelectionService to manage and customize the process of transaction selection. This
 * includes setting limits such as 'TraceLineLimit', 'maxBlockGas', and 'maxCallData'.
 */
@Slf4j
@AutoService(BesuPlugin.class)
public class LineaTransactionSelectorPlugin extends AbstractLineaRequiredPlugin {
  private TransactionSelectionService transactionSelectionService;
  private ServiceManager serviceManager;
  private Optional<JsonRpcManager> rejectedTxJsonRpcManager = Optional.empty();

  @Override
  public void doRegister(final ServiceManager serviceManager) {
    this.serviceManager = serviceManager;
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
                        livenessServiceConfiguration(),
                        blockchainService,
                        chainId,
                        resolveLivenessSigner(livenessServiceConfiguration(), serviceManager)),
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

  static Signer<Secp256k1Signature> resolveLivenessSigner(
      final LineaLivenessServiceConfiguration config, final ServiceManager serviceManager) {
    final Signer<Secp256k1Signature> signer =
        switch (config.signerType()) {
          case WEB3SIGNER -> new Web3SignerDigestSigner(config);
          case CUSTOM ->
              serviceManager
                  .getService(NamedSignerProviderService.class)
                  .orElseThrow(
                      () ->
                          new IllegalStateException(
                              "No NamedSignerProviderService is registered for CUSTOM signer '"
                                  + config.signerName()
                                  + "'"))
                  .create(config.signerName());
        };

    final byte[] publicKey = signer.publicKey();
    if (publicKey.length != 64) {
      throw new IllegalArgumentException(
          config.signerType()
              + " signer public key must be 64-byte secp256k1 coordinates (x || y), got "
              + publicKey.length
              + " bytes");
    }
    String derivedAddress = Numeric.prependHexPrefix(Keys.getAddress(new BigInteger(1, publicKey)));
    if (!derivedAddress.equalsIgnoreCase(config.signerAddress())) {
      throw new IllegalArgumentException(
          "Configured liveness signer address does not match "
              + config.signerType()
              + " signer '"
              + (config.signerName() != null ? config.signerName() : config.signerKeyId())
              + "'");
    }
    return signer;
  }

  @Override
  public void stop() {
    super.stop();
    rejectedTxJsonRpcManager.ifPresent(JsonRpcManager::shutdown);
  }
}
