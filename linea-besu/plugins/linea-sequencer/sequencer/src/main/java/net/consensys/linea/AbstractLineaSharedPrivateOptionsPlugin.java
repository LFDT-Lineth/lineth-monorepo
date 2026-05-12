/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea;

import java.util.HashMap;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.atomic.AtomicBoolean;
import linea.blob.BlobCompressorSelectorByTimestamp;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bl.TransactionProfitabilityCalculator;
import net.consensys.linea.bundles.BundlePoolService;
import net.consensys.linea.bundles.LineaLimitedBundlePool;
import net.consensys.linea.config.LineaBundleCliOptions;
import net.consensys.linea.config.LineaBundleConfiguration;
import net.consensys.linea.config.LineaForcedTransactionCliOptions;
import net.consensys.linea.config.LineaForcedTransactionConfiguration;
import net.consensys.linea.config.LineaLivenessServiceCliOptions;
import net.consensys.linea.config.LineaLivenessServiceConfiguration;
import net.consensys.linea.config.LineaProfitabilityCliOptions;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.config.LineaRejectedTxReportingCliOptions;
import net.consensys.linea.config.LineaRejectedTxReportingConfiguration;
import net.consensys.linea.config.LineaRpcCliOptions;
import net.consensys.linea.config.LineaRpcConfiguration;
import net.consensys.linea.config.LineaTracerCliOptions;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.config.LineaTracerLineLimitConfiguration;
import net.consensys.linea.config.LineaTransactionPoolValidatorCliOptions;
import net.consensys.linea.config.LineaTransactionPoolValidatorConfiguration;
import net.consensys.linea.config.LineaTransactionSelectorCliOptions;
import net.consensys.linea.config.LineaTransactionSelectorConfiguration;
import net.consensys.linea.config.LineaTransactionValidatorCliOptions;
import net.consensys.linea.config.LineaTransactionValidatorConfiguration;
import net.consensys.linea.config.ReloadableMap;
import net.consensys.linea.config.ReloadableSet;
import net.consensys.linea.plugins.AbstractLineaSharedOptionsPlugin;
import net.consensys.linea.plugins.LineaOptionsPluginConfiguration;
import net.consensys.linea.plugins.config.LineaTracerSharedCliOptions;
import net.consensys.linea.plugins.config.LineaTracerSharedConfiguration;
import net.consensys.linea.sequencer.forced.ForcedTransactionPoolService;
import net.consensys.linea.sequencer.forced.LineaForcedTransactionPool;
import net.consensys.linea.sequencer.txselection.InvalidTransactionByLineCountCache;
import net.consensys.linea.sequencer.txselection.selectors.TransactionEventFilter;
import net.consensys.linea.utils.CachingTransactionCompressor;
import net.consensys.linea.utils.TransactionCompressor;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.BesuConfiguration;
import org.hyperledger.besu.plugin.services.BesuEvents;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.MetricsSystem;
import org.hyperledger.besu.plugin.services.RpcEndpointService;
import org.hyperledger.besu.plugin.services.WorldStateService;
import org.hyperledger.besu.plugin.services.metrics.MetricCategoryRegistry;

/**
 * This abstract class is used as superclass for all the plugins that share one or more
 * configuration options, services and common initializations.
 *
 * <p>Configuration options that are exclusive of a single plugin, are not required to be added
 * here, but they could stay in the class that implement a plugin, but in case that configuration
 * becomes to be used by multiple plugins, then to avoid code duplications and possible different
 * management of the options, it is better to move the configuration here so all plugins will
 * automatically see it.
 *
 * <p>Same for services and other initialization tasks, that are shared by more than one plugin,
 * like registration of metrics categories or check to perform once at startup
 */
@Slf4j
public abstract class AbstractLineaSharedPrivateOptionsPlugin
    extends AbstractLineaSharedOptionsPlugin {
  protected static BesuConfiguration besuConfiguration;
  protected static BlockchainService blockchainService;
  protected static WorldStateService worldStateService;
  protected static MetricsSystem metricsSystem;
  protected static BesuEvents besuEvents;
  protected static BundlePoolService bundlePoolService;
  protected static ForcedTransactionPoolService forcedTransactionPoolService;
  protected static MetricCategoryRegistry metricCategoryRegistry;
  protected static RpcEndpointService rpcEndpointService;
  protected static InvalidTransactionByLineCountCache invalidTransactionByLineCountCache;
  protected static BlobCompressorSelectorByTimestamp blobCompressorSelectorByTimestamp;
  protected static TransactionCompressor transactionCompressor;
  protected static TransactionProfitabilityCalculator transactionProfitabilityCalculator;

  public static final int DEFAULT_COMPRESSED_SIZE_LIMIT = 128 * 1024;
  // Shared reloadable deny lists - centralized management for all plugins
  protected static ReloadableSet<Address> sharedDeniedAddresses;
  protected static ReloadableSet<Address> sharedBundleDeniedAddresses;
  protected static ReloadableMap<Address, Set<TransactionEventFilter>> sharedDeniedEvents;
  protected static ReloadableMap<Address, Set<TransactionEventFilter>> sharedDeniedBundleEvents;

  private static final AtomicBoolean sharedRegisterTasksDone = new AtomicBoolean(false);
  private static final AtomicBoolean sharedStartTasksDone = new AtomicBoolean(false);
  private static final AtomicBoolean reloadInProgress = new AtomicBoolean(false);

  private ServiceManager serviceManager;

  @Override
  public Map<String, LineaOptionsPluginConfiguration> getLineaPluginConfigMap() {
    final var configMap = new HashMap<>(super.getLineaPluginConfigMap());

    configMap.put(
        LineaTransactionSelectorCliOptions.CONFIG_KEY,
        LineaTransactionSelectorCliOptions.create().asPluginConfig());
    configMap.put(
        LineaTransactionPoolValidatorCliOptions.CONFIG_KEY,
        LineaTransactionPoolValidatorCliOptions.create().asPluginConfig());
    configMap.put(LineaRpcCliOptions.CONFIG_KEY, LineaRpcCliOptions.create().asPluginConfig());
    configMap.put(
        LineaProfitabilityCliOptions.CONFIG_KEY,
        LineaProfitabilityCliOptions.create().asPluginConfig());
    configMap.put(
        LineaTracerCliOptions.CONFIG_KEY, LineaTracerCliOptions.create().asPluginConfig());
    configMap.put(
        LineaRejectedTxReportingCliOptions.CONFIG_KEY,
        LineaRejectedTxReportingCliOptions.create().asPluginConfig());
    configMap.put(
        LineaBundleCliOptions.CONFIG_KEY, LineaBundleCliOptions.create().asPluginConfig());
    configMap.put(
        LineaTransactionValidatorCliOptions.CONFIG_KEY,
        LineaTransactionValidatorCliOptions.create().asPluginConfig());
    configMap.put(
        LineaLivenessServiceCliOptions.CONFIG_KEY,
        LineaLivenessServiceCliOptions.create().asPluginConfig());
    configMap.put(
        LineaForcedTransactionCliOptions.CONFIG_KEY,
        LineaForcedTransactionCliOptions.create().asPluginConfig());
    return configMap;
  }

  public LineaTransactionSelectorConfiguration transactionSelectorConfiguration() {
    return (LineaTransactionSelectorConfiguration)
        getConfigurationByKey(LineaTransactionSelectorCliOptions.CONFIG_KEY).optionsConfig();
  }

  public LineaTransactionPoolValidatorConfiguration transactionPoolValidatorConfiguration() {
    return (LineaTransactionPoolValidatorConfiguration)
        getConfigurationByKey(LineaTransactionPoolValidatorCliOptions.CONFIG_KEY).optionsConfig();
  }

  public LineaRpcConfiguration lineaRpcConfiguration() {
    return (LineaRpcConfiguration)
        getConfigurationByKey(LineaRpcCliOptions.CONFIG_KEY).optionsConfig();
  }

  public LineaProfitabilityConfiguration profitabilityConfiguration() {
    return (LineaProfitabilityConfiguration)
        getConfigurationByKey(LineaProfitabilityCliOptions.CONFIG_KEY).optionsConfig();
  }

  public LineaTracerConfiguration tracerConfiguration() {
    var tracerLineLimitConfig =
        (LineaTracerLineLimitConfiguration)
            getConfigurationByKey(LineaTracerCliOptions.CONFIG_KEY).optionsConfig();
    var tracerSharedConfig =
        (LineaTracerSharedConfiguration)
            getConfigurationByKey(LineaTracerSharedCliOptions.CONFIG_KEY).optionsConfig();
    return new LineaTracerConfiguration(
        tracerLineLimitConfig.moduleLimitsFilePath(),
        tracerLineLimitConfig.moduleLimitsMap(),
        tracerSharedConfig.isLimitless());
  }

  public LineaRejectedTxReportingConfiguration rejectedTxReportingConfiguration() {
    return (LineaRejectedTxReportingConfiguration)
        getConfigurationByKey(LineaRejectedTxReportingCliOptions.CONFIG_KEY).optionsConfig();
  }

  public LineaBundleConfiguration bundleConfiguration() {
    return (LineaBundleConfiguration)
        getConfigurationByKey(LineaBundleCliOptions.CONFIG_KEY).optionsConfig();
  }

  public LineaTransactionValidatorConfiguration transactionValidatorConfiguration() {
    return (LineaTransactionValidatorConfiguration)
        getConfigurationByKey(LineaTransactionValidatorCliOptions.CONFIG_KEY).optionsConfig();
  }

  public LineaLivenessServiceConfiguration livenessServiceConfiguration() {
    return (LineaLivenessServiceConfiguration)
        getConfigurationByKey(LineaLivenessServiceCliOptions.CONFIG_KEY).optionsConfig();
  }

  public LineaForcedTransactionConfiguration forcedTransactionConfiguration() {
    return (LineaForcedTransactionConfiguration)
        getConfigurationByKey(LineaForcedTransactionCliOptions.CONFIG_KEY).optionsConfig();
  }

  protected InvalidTransactionByLineCountCache getInvalidTransactionByLineCountCache() {
    return invalidTransactionByLineCountCache;
  }

  @Override
  public synchronized void register(final ServiceManager serviceManager) {
    super.register(serviceManager);

    this.serviceManager = serviceManager;

    if (sharedRegisterTasksDone.compareAndSet(false, true)) {
      performSharedRegisterTasksOnce(serviceManager);
    }
  }

  protected static void performSharedRegisterTasksOnce(final ServiceManager serviceManager) {
    besuConfiguration =
        serviceManager
            .getService(BesuConfiguration.class)
            .orElseThrow(
                () ->
                    new RuntimeException(
                        "Failed to obtain BesuConfiguration from the ServiceManager."));
    blockchainService =
        serviceManager
            .getService(BlockchainService.class)
            .orElseThrow(
                () ->
                    new RuntimeException(
                        "Failed to obtain BlockchainService from the ServiceManager."));

    metricCategoryRegistry =
        serviceManager
            .getService(MetricCategoryRegistry.class)
            .orElseThrow(
                () ->
                    new RuntimeException(
                        "Failed to obtain MetricCategoryRegistry from the ServiceManager."));

    rpcEndpointService =
        serviceManager
            .getService(RpcEndpointService.class)
            .orElseThrow(
                () ->
                    new RuntimeException(
                        "Failed to obtain RpcEndpointService from the ServiceManager."));
  }

  @Override
  public void start() {
    super.start();

    if (sharedStartTasksDone.compareAndSet(false, true)) {
      performSharedStartTasksOnce(serviceManager);
    }
  }

  private void performSharedStartTasksOnce(final ServiceManager serviceManager) {

    blockchainService
        .getChainId()
        .ifPresentOrElse(
            chainId -> {
              if (chainId.signum() <= 0) {
                throw new IllegalArgumentException("Chain id must be greater than zero.");
              }
            },
            () -> {
              throw new IllegalArgumentException("Chain id required");
            });

    metricsSystem =
        serviceManager
            .getService(MetricsSystem.class)
            .orElseThrow(
                () ->
                    new RuntimeException("Failed to obtain MetricSystem from the ServiceManager."));

    besuEvents =
        serviceManager
            .getService(BesuEvents.class)
            .orElseThrow(
                () -> new RuntimeException("Failed to obtain BesuEvents from the ServiceManager."));

    worldStateService =
        serviceManager
            .getService(WorldStateService.class)
            .orElseThrow(
                () ->
                    new RuntimeException(
                        "Failed to obtain WorldStateService from the ServiceManager."));

    bundlePoolService =
        new LineaLimitedBundlePool(
            besuConfiguration.getDataPath(),
            transactionSelectorConfiguration().maxBundlePoolSizeBytes(),
            besuEvents,
            blockchainService);
    bundlePoolService.loadFromDisk();

    forcedTransactionPoolService =
        new LineaForcedTransactionPool(
            forcedTransactionConfiguration().statusCacheSize(), metricsSystem, besuEvents);

    invalidTransactionByLineCountCache =
        new InvalidTransactionByLineCountCache(
            transactionSelectorConfiguration().overLinesLimitCacheSize());

    // Initialise the native compressor once with the authoritative limit so that
    // CachingTransactionCompressor and CompressionAwareTransactionSelector share
    // the same instance. Fall back to the default when no blob size limit is configured.
    final int effectiveBlobLimit =
        transactionSelectorConfiguration().blobSizeLimit() != null
            ? transactionSelectorConfiguration().blobSizeLimit()
            : DEFAULT_COMPRESSED_SIZE_LIMIT;
    blobCompressorSelectorByTimestamp =
        new BlobCompressorSelectorByTimestamp(
            transactionSelectorConfiguration().blobCompressorVersionActivationTimes(),
            effectiveBlobLimit);

    final LineaProfitabilityConfiguration profitabilityConfiguration = profitabilityConfiguration();
    transactionCompressor =
        new CachingTransactionCompressor(
            profitabilityConfiguration.compressedTxCacheSize(), blobCompressorSelectorByTimestamp);
    transactionProfitabilityCalculator =
        new TransactionProfitabilityCalculator(profitabilityConfiguration, transactionCompressor);

    initializeSharedDenyLists();
  }

  private void initializeSharedDenyLists() {
    final var txPoolConfig = transactionPoolValidatorConfiguration();
    final var txSelectorConfig = transactionSelectorConfiguration();

    // Initialize shared address deny lists
    sharedDeniedAddresses =
        new ReloadableSet<>(
            LineaTransactionPoolValidatorCliOptions.create()
                .parseDeniedAddresses(txPoolConfig.denyListPath()));

    sharedBundleDeniedAddresses =
        new ReloadableSet<>(
            LineaTransactionPoolValidatorCliOptions.create()
                .parseDeniedAddresses(txPoolConfig.bundleOverridingDenyListPath()));

    // Initialize shared events deny lists
    sharedDeniedEvents =
        new ReloadableMap<>(
            LineaTransactionSelectorCliOptions.create()
                .parseTransactionEventDenyList(txSelectorConfig.eventsDenyListPath()));

    sharedDeniedBundleEvents =
        new ReloadableMap<>(
            LineaTransactionSelectorCliOptions.create()
                .parseTransactionEventDenyList(txSelectorConfig.eventsBundleDenyListPath()));
  }

  /**
   * Reloads configuration when requested via the Besu plugin reload mechanism. This method is
   * guarded by an atomic boolean to ensure that only one plugin actually performs the reload when
   * multiple plugins receive the reload request simultaneously.
   *
   * @return a CompletableFuture that completes when reload is done
   */
  @Override
  public CompletableFuture<Void> reloadConfiguration() {
    // Guard: only one plugin actually reloads, others return completed
    if (!reloadInProgress.compareAndSet(false, true)) {
      return CompletableFuture.completedFuture(null);
    }

    return CompletableFuture.supplyAsync(
        () -> {
          try {
            reloadSharedDenyLists();
            return null;
          } catch (final Exception e) {
            log.error("Failed to reload shared deny lists", e);
            throw e;
          } finally {
            reloadInProgress.set(false);
          }
        });
  }

  /**
   * Reloads all shared deny lists from their configured sources using an atomic swap pattern. All
   * new values are loaded first, and only if all loads succeed are the values swapped atomically.
   * This ensures no partial updates occur if any load fails.
   */
  private void reloadSharedDenyLists() {
    final var txPoolConfig = transactionPoolValidatorConfiguration();
    final var txSelectorConfig = transactionSelectorConfiguration();

    // Load all new values first (may throw)
    final Set<Address> newDeniedAddresses =
        LineaTransactionPoolValidatorCliOptions.create()
            .parseDeniedAddresses(txPoolConfig.denyListPath());
    final Set<Address> newBundleDeniedAddresses =
        LineaTransactionPoolValidatorCliOptions.create()
            .parseDeniedAddresses(txPoolConfig.bundleOverridingDenyListPath());
    final Map<Address, Set<TransactionEventFilter>> newDeniedEvents =
        LineaTransactionSelectorCliOptions.create()
            .parseTransactionEventDenyList(txSelectorConfig.eventsDenyListPath());
    final Map<Address, Set<TransactionEventFilter>> newDeniedBundleEvents =
        LineaTransactionSelectorCliOptions.create()
            .parseTransactionEventDenyList(txSelectorConfig.eventsBundleDenyListPath());

    // All loads succeeded - swap each deny list (not atomic)
    sharedDeniedAddresses.swap(newDeniedAddresses);
    sharedBundleDeniedAddresses.swap(newBundleDeniedAddresses);
    sharedDeniedEvents.swap(newDeniedEvents);
    sharedDeniedBundleEvents.swap(newDeniedBundleEvents);

    log.info("Successfully reloaded all shared deny lists");
  }

  @Override
  public void stop() {
    super.stop();
    sharedRegisterTasksDone.set(false);
    sharedStartTasksDone.set(false);
    reloadInProgress.set(false);
    blockchainService = null;
    metricsSystem = null;
    blobCompressorSelectorByTimestamp = null;
    sharedDeniedAddresses = null;
    sharedBundleDeniedAddresses = null;
    sharedDeniedEvents = null;
    sharedDeniedBundleEvents = null;
  }
}
