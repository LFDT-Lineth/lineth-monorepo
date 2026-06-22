/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import io.libp2p.core.PeerId
import io.libp2p.core.crypto.unmarshalPrivateKey
import io.micrometer.core.instrument.MeterRegistry
import io.vertx.core.Vertx
import io.vertx.micrometer.MicrometerMetricsOptions
import io.vertx.micrometer.backends.BackendRegistries
import linea.contract.l1.LineaRollupSmartContractClientReadOnly
import linea.contract.l1.Web3JLineaRollupSmartContractClientReadOnly
import linea.kotlin.encodeHex
import linea.timer.JvmTimerFactory
import linea.timer.TimerFactory
import linea.timer.VertxTimerFactory
import linea.web3j.createWeb3jHttpClient
import linea.web3j.ethapi.createEthApiClient
import maru.api.ApiServer
import maru.api.ApiServerImpl
import maru.api.ChainDataProviderImpl
import maru.config.MaruConfig
import maru.config.P2PConfig
import maru.config.SyncingConfig
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ElFork
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.consensus.StaticValidatorProvider
import maru.consensus.blockimport.ElForkAwareBlockImporter
import maru.consensus.state.FinalizationProvider
import maru.consensus.state.InstantFinalizationProvider
import maru.core.SealedBeaconBlock
import maru.database.BeaconChain
import maru.database.P2PState
import maru.database.kv.KvDatabaseFactory
import maru.executionlayer.ExecutionLayerFactory.buildExecutionEngineClient
import maru.executionlayer.manager.JsonRpcExecutionLayerManager
import maru.finalization.LineaFinalizationProvider
import maru.metrics.BesuMetricsCategoryAdapter
import maru.metrics.BesuMetricsSystemAdapter
import maru.metrics.MaruMetricsCategory
import maru.p2p.NetworkHelper
import maru.p2p.NoOpP2PNetwork
import maru.p2p.P2PNetwork
import maru.p2p.P2PNetworkDataProvider
import maru.p2p.P2PNetworkImpl
import maru.p2p.P2PPeersHeadBlockProvider
import maru.p2p.fork.ForkPeeringManager
import maru.p2p.fork.LenientForkPeeringManager
import maru.p2p.messages.StatusManager
import maru.serialization.SerDe
import maru.serialization.rlp.RLPSerializers
import maru.services.LongRunningService
import maru.services.NoOpLongRunningService
import maru.syncing.AlwaysSyncedController
import maru.syncing.BeaconSyncControllerImpl
import maru.syncing.ELSyncService
import maru.syncing.ELSyncStatus
import maru.syncing.HighestHeadTargetSelector
import maru.syncing.MostFrequentHeadTargetSelector
import maru.syncing.PeerChainTracker
import maru.syncing.SyncController
import maru.syncing.SyncTargetSelector
import maru.syncing.beaconchain.pipeline.BeaconChainDownloadPipelineFactory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.vertx.VertxFactory
import org.apache.logging.log4j.LogManager
import org.web3j.protocol.Web3j
import org.web3j.protocol.http.HttpService
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient
import tech.pegasys.teku.networking.p2p.network.config.GeneratingFilePrivateKeySource
import java.nio.file.Files
import java.nio.file.Path
import java.time.Clock
import kotlin.io.path.createDirectories
import kotlin.io.path.exists
import org.hyperledger.besu.plugin.services.MetricsSystem as BesuMetricsSystem

interface MaruAppFactoryCreator {
  fun create(
    config: MaruConfig,
    beaconGenesisConfig: ForksSchedule,
    clock: Clock = Clock.systemUTC(),
    overridingP2PNetwork: P2PNetwork? = null,
    overridingFinalizationProvider: FinalizationProvider? = null,
    overridingLineaContractClient: LineaRollupSmartContractClientReadOnly? = null,
    overridingApiServer: ApiServer? = null,
    p2pNetworkFactory: (
      ByteArray,
      P2PConfig,
      UInt,
      SerDe<SealedBeaconBlock>,
      MetricsFacade,
      BesuMetricsSystem,
      StatusManager,
      BeaconChain,
      ForkPeeringManager,
      () -> Boolean,
      P2PState,
      TimerFactory,
    ) -> P2PNetworkImpl = ::P2PNetworkImpl,
  ): LongRunningCloseable
}

class MaruAppFactory : MaruAppFactoryCreator {
  private val log = LogManager.getLogger(this.javaClass)

  override fun create(
    config: MaruConfig,
    beaconGenesisConfig: ForksSchedule,
    clock: Clock,
    overridingP2PNetwork: P2PNetwork?,
    overridingFinalizationProvider: FinalizationProvider?,
    overridingLineaContractClient: LineaRollupSmartContractClientReadOnly?,
    overridingApiServer: ApiServer?,
    p2pNetworkFactory: (
      ByteArray,
      P2PConfig,
      UInt,
      SerDe<SealedBeaconBlock>,
      MetricsFacade,
      BesuMetricsSystem,
      StatusManager,
      BeaconChain,
      ForkPeeringManager,
      () -> Boolean,
      P2PState,
      TimerFactory,
    ) -> P2PNetworkImpl,
  ): MaruApp {
    log.info("configs={}", config)
    log.info("beaconGenesisConfig={}", beaconGenesisConfig)

    val l2EthWeb3j: Web3j? =
      config.forkTransition.l2EthApiEndpoint?.let {
        Web3j.build(HttpService(it.endpoint.toString()))
      }

    checkL2EthApiEndpointAndForks(clock, beaconGenesisConfig, l2EthWeb3j)

    config.persistence.dataPath.createDirectories()
    val privateKey = getOrGeneratePrivateKey(config.persistence.privateKeyPath)
    val nodeId = PeerId.fromPubKey(unmarshalPrivateKey(privateKey).publicKey())
    val vertx =
      VertxFactory.createVertx(
        jvmMetricsEnabled = config.observability.jvmMetricsEnabled,
        prometheusMetricsEnabled = config.observability.prometheusMetricsEnabled,
      )
    val timerFactory =
      if (config.useVertxTimers) {
        VertxTimerFactory(vertx)
      } else {
        JvmTimerFactory()
      }
    val metricsFacade =
      MicrometerMetricsFacade(
        registry = getMetricsRegistry(),
        metricsPrefix = "maru",
        allMetricsCommonTags = listOf(Tag("nodeid", nodeId.toBase58())),
      )
    val besuMetricsSystemAdapter =
      BesuMetricsSystemAdapter(
        metricsFacade = metricsFacade,
        vertx = vertx,
      )

    val kvDatabase =
      KvDatabaseFactory
        .createRocksDbDatabase(
          databasePath = config.persistence.dataPath,
          metricsSystem = besuMetricsSystemAdapter,
          metricCategory = BesuMetricsCategoryAdapter.from(MaruMetricsCategory.STORAGE),
        ).also {
          dbInitialization(beaconGenesisConfig, it)
        }

    val qbftFork = beaconGenesisConfig.getForkByConfigType(QbftConsensusConfig::class)
    val qbftConfig = qbftFork.configuration as QbftConsensusConfig

    val engineApiWeb3jClient: Web3JClient? =
      config.validatorElNode?.let {
        Helpers.createWeb3jClient(
          apiEndpointConfig = it.engineApiEndpoint,
          log = LogManager.getLogger("maru.clients.el.engineapi"),
        )
      }

    val elManagerMap =
      if (engineApiWeb3jClient != null) {
        ElFork.entries.associateWith {
          val engineApiClient =
            buildExecutionEngineClient(
              web3JEngineApiClient = engineApiWeb3jClient,
              elFork = it,
              metricsFacade = metricsFacade,
            )
          JsonRpcExecutionLayerManager(engineApiClient)
        }
      } else {
        emptyMap()
      }

    // Because of the circular dependency between SyncStatusProvider, P2PNetwork and P2PPeersHeadBlockProvider
    var syncControllerImpl: SyncController? = null
    val p2pNetwork =
      overridingP2PNetwork ?: setupP2PNetwork(
        forkSchedule = beaconGenesisConfig,
        p2pConfig = config.p2p,
        privateKey = privateKey,
        chainId = beaconGenesisConfig.chainId,
        beaconChain = kvDatabase,
        metricsFacade = metricsFacade,
        besuMetricsSystem = besuMetricsSystemAdapter,
        isBlockImportEnabledProvider = {
          if (config.validatorElNode?.payloadValidationEnabled == true) {
            syncControllerImpl!!.isNodeFullInSync()
          } else {
            syncControllerImpl!!.isBeaconChainSynced()
          }
        },
        p2PState = kvDatabase,
        clock = clock,
        timerFactory = timerFactory,
        p2pNetworkFactory = p2pNetworkFactory,
      )
    val peersHeadBlockProvider = P2PPeersHeadBlockProvider(p2pNetwork.getPeerLookup())
    val finalizationProvider =
      overridingFinalizationProvider
        ?: setupFinalizationProvider(config, overridingLineaContractClient, vertx, timerFactory)
    syncControllerImpl = if (config.p2p != null) {
      val followerELNodeEngineApiWeb3JClients: Map<String, Web3JClient> =
        config.followers.followers.mapValues { (followerLabel, apiEndpointConfig) ->
          Helpers.createWeb3jClient(
            apiEndpointConfig = apiEndpointConfig,
            log = LogManager.getLogger("maru.clients.follower.$followerLabel"),
          )
        }
      val elSyncBlockImportHandlers =
        Helpers.createForkAwareBlockImportHandlers(
          forksSchedule = beaconGenesisConfig,
          metricsFacade = metricsFacade,
          followerELNodeEngineApiWeb3JClients = followerELNodeEngineApiWeb3JClients,
          finalizationProvider = finalizationProvider,
        )
      // Validators manage EL sync through QBFT consensus itself.
      // Followers only use ELSyncService when an explicit polling interval is configured.
      val elSyncEnabled = config.qbft == null && config.syncing.elSyncStatusRefreshInterval != null
      val elSyncServiceFactory: ((ELSyncStatus) -> Unit) -> LongRunningService = { onStatusChange ->
        val refreshInterval = config.syncing.elSyncStatusRefreshInterval
        if (elSyncEnabled && engineApiWeb3jClient != null) {
          val validatorImportHandler =
            ElForkAwareBlockImporter(
              forksSchedule = beaconGenesisConfig,
              elManagerMap = elManagerMap,
              importerName = "El sync payload validator",
              finalizationProvider = finalizationProvider,
            )
          ELSyncService(
            config = ELSyncService.Config(refreshInterval!!),
            beaconChain = kvDatabase,
            eLValidatorBlockImportHandler = validatorImportHandler,
            followerELBLockImportHandler = elSyncBlockImportHandlers,
            onStatusChange = onStatusChange,
            timerFactory = timerFactory,
          )
        } else {
          NoOpLongRunningService
        }
      }
      BeaconSyncControllerImpl.create(
        beaconChain = kvDatabase,
        peersHeadsProvider = peersHeadBlockProvider,
        targetChainHeadCalculator = createSyncTargetSelector(config.syncing.syncTargetSelection),
        validatorProvider = StaticValidatorProvider(qbftConfig.validatorSet),
        peerLookup = p2pNetwork.getPeerLookup(),
        besuMetrics = besuMetricsSystemAdapter,
        metricsFacade = metricsFacade,
        peerChainTrackerConfig = PeerChainTracker.Config(config.syncing.peerChainHeightPollingInterval),
        elSyncServiceFactory = elSyncServiceFactory,
        elSyncEnabled = elSyncEnabled,
        desyncTolerance = config.syncing.desyncTolerance,
        pipelineConfig = BeaconChainDownloadPipelineFactory.Config(
          blockRangeRequestTimeout = config.syncing.download.blockRangeRequestTimeout,
          backoffDelay = config.syncing.download.backoffDelay,
          blocksBatchSize = config.syncing.download.blocksBatchSize,
          blocksParallelism = config.syncing.download.blocksParallelism,
          maxRetries = config.syncing.download.maxRetries,
          useUnconditionalRandomDownloadPeer = config.syncing.download.useUnconditionalRandomDownloadPeer,
        ),
        allowEmptyBlocks = config.allowEmptyBlocks,
        timerFactory = timerFactory,
      )
    } else {
      AlwaysSyncedController(kvDatabase)
    }

    val apiServer =
      overridingApiServer
        ?: ApiServerImpl(
          config = ApiServerImpl.Config(port = config.api.port),
          networkDataProvider = P2PNetworkDataProvider(p2pNetwork),
          versionProvider = MaruVersionProvider(),
          chainDataProvider = ChainDataProviderImpl(kvDatabase),
          syncStatusProvider = syncControllerImpl,
          isElOnlineProvider = {
            if (engineApiWeb3jClient != null) {
              elManagerMap.values
                .firstOrNull()
                ?.isOnline()
                ?.get() ?: true
            } else {
              true
            }
          },
        )

    return MaruApp(
      config = config,
      beaconGenesisConfig = beaconGenesisConfig,
      clock = clock,
      p2pNetwork = p2pNetwork,
      privateKeyProvider = { privateKey },
      finalizationProvider = finalizationProvider,
      metricsFacade = metricsFacade,
      vertx = vertx,
      beaconChain = kvDatabase,
      metricsSystem = besuMetricsSystemAdapter,
      l2EthWeb3j = l2EthWeb3j,
      validatorELNodeEngineApiWeb3JClient = engineApiWeb3jClient,
      apiServer = apiServer,
      syncControllerManager = syncControllerImpl,
      timerFactory = timerFactory,
    )
  }

  companion object {
    private val log = LogManager.getLogger(MaruAppFactory::class.java)

    // the second case can happen in case of concurrent access i.e. in the tests
    private fun getMetricsRegistry(): MeterRegistry =
      BackendRegistries.getDefaultNow() ?: BackendRegistries
        .setupBackend(
          MicrometerMetricsOptions(),
          null,
        ).let { BackendRegistries.getDefaultNow() }

    private fun setupFinalizationProvider(
      config: MaruConfig,
      overridingLineaContractClient: LineaRollupSmartContractClientReadOnly?,
      vertx: Vertx,
      timerFactory: TimerFactory,
    ): FinalizationProvider =
      config.linea
        ?.let { lineaConfig ->
          val web3jClient =
            createWeb3jHttpClient(
              rpcUrl = lineaConfig.l1EthApiEndpoint.endpoint.toString(),
              log = LogManager.getLogger("clients.l1.eth.ftx"),
            )
          val contractClient =
            overridingLineaContractClient
              ?: Web3JLineaRollupSmartContractClientReadOnly(
                web3j = web3jClient,
                contractAddress = lineaConfig.contractAddress.encodeHex(),
                ethLogsClient = createEthApiClient(
                  web3jClient = web3jClient,
                  vertx = vertx,
                ),
                log = LogManager.getLogger("maru.clients.l1.linea"),
              )
          val l2Endpoint = lineaConfig.l2EthApiEndpoint
          LineaFinalizationProvider(
            lineaContract = contractClient,
            l2EthApi = createEthApiClient(
              rpcUrl = l2Endpoint.endpoint
                .toString(),
              log = LogManager.getLogger("maru.clients.l2.eth.el"),
              requestRetryConfig = l2Endpoint.requestRetries,
              vertx = vertx,
              stopRetriesOnErrorPredicate = { true },
            ),
            pollingUpdateInterval = lineaConfig.l1PollingInterval,
            l1HighestBlock = lineaConfig.l1HighestBlockTag,
            timerFactory = timerFactory,
          )
        } ?: InstantFinalizationProvider

    private fun setupP2PNetwork(
      forkSchedule: ForksSchedule,
      p2pConfig: P2PConfig?,
      privateKey: ByteArray,
      chainId: UInt,
      beaconChain: BeaconChain,
      isBlockImportEnabledProvider: () -> Boolean,
      metricsFacade: MetricsFacade,
      besuMetricsSystem: BesuMetricsSystem,
      clock: Clock,
      p2PState: P2PState,
      timerFactory: TimerFactory,
      p2pNetworkFactory: (
        ByteArray,
        P2PConfig,
        UInt,
        SerDe<SealedBeaconBlock>,
        MetricsFacade,
        BesuMetricsSystem,
        StatusManager,
        BeaconChain,
        ForkPeeringManager,
        () -> Boolean,
        P2PState,
        TimerFactory,
      ) -> P2PNetworkImpl = ::P2PNetworkImpl,
    ): P2PNetwork {
      if (p2pConfig == null) {
        log.info("No P2P configuration provided, using NoOpP2PNetwork")
        return NoOpP2PNetwork
      }
      val forkIdHashManager =
        LenientForkPeeringManager.create(
          chainId = chainId,
          beaconChain = beaconChain,
          forks = forkSchedule.forks.toList(),
          peeringForkMismatchLeewayTime = p2pConfig.peeringForkMismatchLeewayTime,
          clock = clock,
        )
      val statusManager = StatusManager(beaconChain, forkIdHashManager)

      return p2pNetworkFactory(
        privateKey,
        NetworkHelper
          .selectIpV4ForP2P(
            targetIpV4 = p2pConfig.ipAddress,
            excludeLoopback = true,
          ).also { log.info("using p2p ip={}", it) }
          .let { p2pConfig.copy(ipAddress = it) },
        chainId,
        RLPSerializers.SealedBeaconBlockCompressorSerializer,
        metricsFacade,
        besuMetricsSystem,
        statusManager,
        beaconChain,
        forkIdHashManager,
        isBlockImportEnabledProvider,
        p2PState,
        timerFactory,
      )
    }

    private fun createSyncTargetSelector(config: SyncingConfig.SyncTargetSelection): SyncTargetSelector =
      when (config) {
        is SyncingConfig.SyncTargetSelection.Highest -> {
          HighestHeadTargetSelector()
        }

        is SyncingConfig.SyncTargetSelection.MostFrequent -> {
          MostFrequentHeadTargetSelector(config.peerChainHeightGranularity)
        }
      }

    private fun getOrGeneratePrivateKey(privateKeyPath: Path): ByteArray {
      if (!privateKeyPath
          .toFile()
          .exists()
      ) {
        log.info(
          "Private key file {} does not exist. A new private key will be generated and stored in that location.",
          privateKeyPath.toString(),
        )
      } else {
        log.info("Maru is using private key defined in file={}", privateKeyPath.toString())
      }

      ensureDirectoryExists(privateKeyPath.parent)

      return GeneratingFilePrivateKeySource(privateKeyPath.toString()).privateKeyBytes.toArray()
    }

    private fun ensureDirectoryExists(path: Path) {
      if (!path.exists()) Files.createDirectories(path)
    }
  }

  private fun dbInitialization(
    beaconGenesisConfig: ForksSchedule,
    beaconChain: BeaconChain,
  ) {
    val qbftForkConfig = beaconGenesisConfig.getForkByConfigType(QbftConsensusConfig::class)
    val beaconChainInitialization =
      BeaconChainInitialization(
        beaconChain = beaconChain,
      )
    val qbftConsensusConfig = qbftForkConfig.configuration as QbftConsensusConfig
    beaconChainInitialization.ensureDbIsInitialized(qbftConsensusConfig.validatorSet)
  }

  internal fun checkL2EthApiEndpointAndForks(
    clock: Clock,
    forksSchedule: ForksSchedule,
    l2EthWeb3j: Any?,
  ) {
    val nowTs = clock.instant().epochSecond.toULong()

    val currentForkConfig = forksSchedule.getForkByTimestamp(nowTs).configuration
    val hasFutureDifficultyAwareQbft =
      forksSchedule.forks.any { fork ->
        fork.timestampSeconds > nowTs && fork.configuration is DifficultyAwareQbftConfig
      }
    if (hasFutureDifficultyAwareQbft || currentForkConfig is DifficultyAwareQbftConfig) {
      require(l2EthWeb3j != null) {
        "Configuration error: a future fork enables DifficultyAwareQbft (by timestamp) but l2EthWeb3j " +
          "is not configured, so there is no way to check the current difficulty. Provide L2 Ethereum JSON-RPC " +
          "endpoint in configuration so the app can start."
      }
    }
  }
}
