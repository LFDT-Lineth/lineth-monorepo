/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import io.vertx.core.Vertx
import linea.timer.TimerFactory
import maru.api.ApiServer
import maru.config.MaruConfig
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.NextBlockTimestampProviderImpl
import maru.consensus.OmniProtocolFactory
import maru.consensus.ProtocolStarter
import maru.consensus.QbftConsensusConfig
import maru.consensus.qbft.DifficultyAwareQbftFactory
import maru.consensus.state.FinalizationProvider
import maru.core.Protocol
import maru.core.SealedBeaconBlock
import maru.core.Validator
import maru.crypto.SecpCrypto
import maru.database.BeaconChain
import maru.extensions.encodeHex
import maru.finalization.LineaFinalizationProvider
import maru.metrics.MaruMetricsCategory
import maru.p2p.P2PNetwork
import maru.services.LongRunningService
import maru.subscription.InOrderFanoutSubscriptionManager
import maru.syncing.SyncController
import net.consensys.linea.async.get
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.vertx.ObservabilityServer
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.plugin.services.MetricsSystem
import org.web3j.protocol.Web3j
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient
import java.time.Clock
import java.util.concurrent.CompletableFuture

interface LongRunningCloseable :
  LongRunningService,
  AutoCloseable

class MaruApp(
  val config: MaruConfig,
  val beaconGenesisConfig: ForksSchedule,
  clock: Clock = Clock.systemUTC(),
  // This will only be used if config.p2pConfig is undefined
  val p2pNetwork: P2PNetwork,
  val beaconChain: BeaconChain,
  private val privateKeyProvider: () -> ByteArray,
  private val finalizationProvider: FinalizationProvider,
  private val vertx: Vertx,
  private val metricsFacade: MetricsFacade,
  private val metricsSystem: MetricsSystem,
  private val l2EthWeb3j: Web3j?,
  private val validatorELNodeEngineApiWeb3JClient: Web3JClient?,
  private val apiServer: ApiServer,
  private val syncControllerManager: SyncController,
  private val timerFactory: TimerFactory,
) : LongRunningCloseable {
  private val log: Logger = LogManager.getLogger(this.javaClass)

  private fun getPrivateKeyWithoutPrefix() = SecpCrypto.privateKeyBytesWithoutPrefix(privateKeyProvider())

  /**
   * Micrometer-based consensus metrics. Non-null when this node is a validator (config.qbft != null).
   * Records QBFT phase latencies as histograms, queryable via Prometheus in K8S or via MeterRegistry in tests.
   */
  private val consensusMetrics: ConsensusMetrics? =
    if (config.qbft != null) {
      ConsensusMetrics(
        metricsFacade = metricsFacade,
        currentHeightProvider = {
          beaconChain
            .getLatestBeaconState()
            .beaconBlockHeader.number
            .toLong()
        },
      )
    } else {
      null
    }

  init {
    if (config.qbft == null) {
      log.info("Qbft options are not defined. nodeRole=follower")
    } else {
      val localValidator = SecpCrypto.privateKeyToValidator(getPrivateKeyWithoutPrefix())
      log.info("Qbft options are defined. nodeRole=validator with address={}", localValidator.address.encodeHex())
      // TODO: This may be not needed when we use dynamic validator set from a smart contract
      warnIfValidatorIsNotInTheGenesis(localValidator)
    }

    metricsFacade.createGauge(
      category = MaruMetricsCategory.METADATA,
      name = "cl.block.height",
      description = "Latest CL block height",
      measurementSupplier = {
        beaconChain
          .getLatestBeaconState()
          .beaconBlockHeader.number
          .toLong()
      },
    )

    if (config.p2p != null) {
      metricsFacade.createGauge(
        category = MaruMetricsCategory.P2P_NETWORK,
        name = "peers.max",
        description = "Max peers configuration",
        measurementSupplier = {
          config.p2p!!.maxPeers
        },
      )
    }
  }

  private val followerELNodeEngineApiWeb3JClients: Map<String, Web3JClient> =
    config.followers.followers.mapValues { (followerLabel, apiEndpointConfig) ->
      Helpers.createWeb3jClient(
        apiEndpointConfig = apiEndpointConfig,
        log = LogManager.getLogger("maru.clients.follower.$followerLabel"),
      )
    }

  fun p2pPort(): UInt = p2pNetwork.port

  fun apiPort(): UInt = apiServer.port().toUInt()

  /**
   * Optional observer called when a sealed beacon block is committed to the BeaconChain database
   * via the QBFT consensus path (validator nodes only).
   * Parameters: (sealedBeaconBlock: SealedBeaconBlock).
   * Note: this callback is NOT invoked for blocks imported via the follower or CL-sync paths.
   * Must be set before [start].
   */
  var onBlockCommitted: ((SealedBeaconBlock) -> Unit)? = null

  private val nextTargetBlockTimestampProvider =
    NextBlockTimestampProviderImpl(
      clock = clock,
      forksSchedule = beaconGenesisConfig,
    )
  private val protocolStarter =
    createProtocolStarter(
      config = config,
      beaconGenesisConfig = beaconGenesisConfig,
      clock = clock,
      beaconChain = beaconChain,
      privateKeyWithoutPrefix = getPrivateKeyWithoutPrefix(),
      timerFactory = timerFactory,
    )

  override fun start(): CompletableFuture<Unit> {
    if (finalizationProvider is LineaFinalizationProvider) {
      start("Finalization Provider") { finalizationProvider.start().get() }
    }
    start("P2P Network") { p2pNetwork.start().get() }
    // Gate consensus start on CL sync: register before syncControllerManager.start() so the
    // callback fires on the first sync completion (which is immediate for a node at genesis).
    // We intentionally do NOT pause consensus on subsequent CLSyncStatus.SYNCING transitions:
    // with desyncTolerance tuned appropriately, QBFT handles brief catch-up gaps naturally
    // without needing an external pause, and pausing on every sync event would stall block
    // production whenever a peer momentarily reports a higher head.
    syncControllerManager.onBeaconSyncComplete { protocolStarter.start() }
    start("Sync Service") { syncControllerManager.start().get() }
    start("Beacon Api") { apiServer.start().get() }
    // observability shall be the last to start because of liveness/readiness probe
    start("Observability Server") {
      ObservabilityServer(
        ObservabilityServer.Config(applicationName = "maru", port = config.observability.port.toInt()),
      ).let { vertx.deployVerticle(it).get() }
    }
    log.info("Maru is up")
    return CompletableFuture.completedFuture(Unit)
  }

  override fun stop(): CompletableFuture<Unit> {
    stop("Sync service") { syncControllerManager.stop().get() }
    stop("P2P Network") { p2pNetwork.stop().get() }
    if (finalizationProvider is LineaFinalizationProvider) {
      stop("Finalization Provider") { finalizationProvider.stop().get() }
    }
    stop("Beacon API") { apiServer.stop().get() }
    stop("Protocol") { protocolStarter.pause() }
    stop("vertx verticles") {
      vertx.deploymentIDs().forEach {
        vertx.undeploy(it).get()
      }
    }
    log.info("Maru is down")
    return CompletableFuture.completedFuture(Unit)
  }

  override fun close() {
    validatorELNodeEngineApiWeb3JClient?.eth1Web3j?.shutdown()
    l2EthWeb3j?.shutdown()
    followerELNodeEngineApiWeb3JClients.forEach { (_, web3jClient) -> web3jClient.eth1Web3j.shutdown() }
    p2pNetwork.close()
    vertx.close()
    protocolStarter.close()
    // close db last, otherwise other components may fail trying to save data
    beaconChain.close()
  }

  private fun start(
    serviceName: String,
    action: () -> Unit,
  ) {
    runCatching(action)
      .onSuccess { log.info("{} started!", serviceName) }
      .onFailure { log.error("Failed to start {}, errorMessage={}", serviceName, it.message, it) }
      .getOrThrow()
  }

  private fun stop(
    serviceName: String,
    action: () -> Unit,
  ) = runCatching(action)
    .getOrElse { log.warn("Failed to stop {}, errorMessage={}", serviceName, it.message, it) }

  fun peersConnected(): UInt =
    p2pNetwork
      .peerCount
      .toUInt()

  private fun createProtocolStarter(
    config: MaruConfig,
    beaconGenesisConfig: ForksSchedule,
    clock: Clock,
    beaconChain: BeaconChain,
    privateKeyWithoutPrefix: ByteArray,
    timerFactory: TimerFactory,
  ): Protocol {
    val qbftFactory =
      if (config.qbft != null) {
        QbftProtocolValidatorFactory(
          qbftOptions = config.qbft!!,
          privateKeyBytes = privateKeyWithoutPrefix,
          validatorELNodeEngineApiWeb3JClient = validatorELNodeEngineApiWeb3JClient!!,
          followerELNodeEngineApiWeb3JClients = followerELNodeEngineApiWeb3JClients,
          metricsSystem = metricsSystem,
          finalizationStateProvider = finalizationProvider,
          nextTargetBlockTimestampProvider = nextTargetBlockTimestampProvider,
          beaconChain = beaconChain,
          clock = clock,
          p2pNetwork = p2pNetwork,
          metricsFacade = metricsFacade,
          allowEmptyBlocks = config.allowEmptyBlocks,
          forksSchedule = beaconGenesisConfig,
          payloadValidationEnabled = config.validatorElNode!!.payloadValidationEnabled,
          onBlockTimerFired = { blockNumber -> consensusMetrics?.recordTimerFire(blockNumber) },
          onMessageReceived = { msgCode, seqNum -> consensusMetrics?.recordMessageReceived(msgCode, seqNum) },
          onBlockMined = { sealedBlock ->
            consensusMetrics?.recordBlockCommitted(sealedBlock)
            onBlockCommitted?.invoke(sealedBlock)
          },
          syncStatusProvider = syncControllerManager,
        )
      } else {
        QbftFollowerFactory(
          p2pNetwork = p2pNetwork,
          beaconChain = beaconChain,
          validatorELNodeEngineApiWeb3JClient = validatorELNodeEngineApiWeb3JClient,
          followerELNodeEngineApiWeb3JClients = followerELNodeEngineApiWeb3JClients,
          metricsFacade = metricsFacade,
          allowEmptyBlocks = config.allowEmptyBlocks,
          finalizationStateProvider = finalizationProvider,
          payloadValidationEnabled = config.validatorElNode?.payloadValidationEnabled ?: false,
        )
      }
    val forkTransitionSubscriptionManager = InOrderFanoutSubscriptionManager<ForkSpec>()
    forkTransitionSubscriptionManager.addSyncSubscriber(p2pNetwork::handleForkTransition)
    val difficultyAwareQbftFactory =
      DifficultyAwareQbftFactory(
        ethereumJsonRpcClient = l2EthWeb3j,
        postTtdProtocolFactory = qbftFactory,
        timerFactory = timerFactory,
      )
    val protocolStarter =
      ProtocolStarter(
        forksSchedule = beaconGenesisConfig,
        protocolFactory = OmniProtocolFactory(
          qbftConsensusFactory = qbftFactory,
          difficultyAwareQbftFactory = difficultyAwareQbftFactory,
        ),
        nextBlockTimestampProvider = nextTargetBlockTimestampProvider,
        forkTransitionCheckInterval = config.forkTransition.protocolTransitionPollingInterval,
        forkTransitionNotifier = forkTransitionSubscriptionManager,
        clock = clock,
        timerFactory = timerFactory,
      )

    return protocolStarter
  }

  private fun warnIfValidatorIsNotInTheGenesis(localValidator: Validator) {
    val validatorsFromAllForks: Set<Validator> =
      beaconGenesisConfig.forks
        .flatMap<ForkSpec, Validator> {
          when (val configuration = it.configuration) {
            is DifficultyAwareQbftConfig -> configuration.postTtdConfig.validatorSet
            is QbftConsensusConfig -> configuration.validatorSet
            else -> throw IllegalArgumentException("")
          }
        }.toSet()
    if (!validatorsFromAllForks.contains(localValidator)) {
      log.warn(
        "localValidator={} isn't found in any of validatorSet-s in any of the Forks in the Genesis file!",
        localValidator,
      )
    }
  }
}
