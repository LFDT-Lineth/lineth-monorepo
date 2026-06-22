/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import io.libp2p.core.PeerId
import io.libp2p.core.crypto.unmarshalPrivateKey
import linea.timer.TimerFactory
import maru.config.P2PConfig
import maru.consensus.ForkSpec
import maru.core.SealedBeaconBlock
import maru.crypto.SecpCrypto.privateKeyBytesWithoutPrefix
import maru.database.BeaconChain
import maru.database.P2PState
import maru.metrics.MaruMetricsCategory
import maru.p2p.NetworkHelper.listIpsV4
import maru.p2p.discovery.MaruDiscoveryService
import maru.p2p.fork.ForkPeeringManager
import maru.p2p.messages.StatusManager
import maru.p2p.topics.ImmediateTopicHandler
import maru.p2p.topics.QbftMessageSerDe
import maru.p2p.topics.TopicHandlerWithInOrderDelivering
import maru.serialization.SerDe
import maru.serialization.rlp.MaruCompressorRLPSerDe
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.metrics.Tag
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes
import org.ethereum.beacon.discovery.schema.NodeRecord
import org.hyperledger.besu.consensus.qbft.core.types.QbftMessage
import tech.pegasys.teku.infrastructure.async.AsyncRunnerFactory
import tech.pegasys.teku.infrastructure.async.MetricTrackingExecutorFactory
import tech.pegasys.teku.infrastructure.async.SafeFuture
import tech.pegasys.teku.infrastructure.time.SystemTimeProvider
import tech.pegasys.teku.networking.p2p.libp2p.LibP2PNodeId
import tech.pegasys.teku.networking.p2p.libp2p.MultiaddrPeerAddress
import tech.pegasys.teku.networking.p2p.libp2p.PeerAlreadyConnectedException
import tech.pegasys.teku.networking.p2p.network.PeerAddress
import tech.pegasys.teku.networking.p2p.peer.DisconnectReason
import tech.pegasys.teku.networking.p2p.peer.NodeId
import tech.pegasys.teku.networking.p2p.peer.Peer
import java.util.Optional
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.TimeUnit
import kotlin.jvm.optionals.getOrElse
import kotlin.jvm.optionals.getOrNull
import org.hyperledger.besu.plugin.services.MetricsSystem as BesuMetricsSystem

class P2PNetworkImpl(
  private val privateKeyBytes: ByteArray,
  private val p2pConfig: P2PConfig,
  private val chainId: UInt,
  private val serDe: SerDe<SealedBeaconBlock>,
  private val metricsFacade: MetricsFacade,
  metricsSystem: BesuMetricsSystem,
  private val statusManager: StatusManager,
  private val beaconChain: BeaconChain,
  private val forkIdHashManager: ForkPeeringManager,
  isBlockImportEnabledProvider: () -> Boolean,
  private val p2PState: P2PState,
  private val timerFactory: TimerFactory,
  // for testing:
  private val rpcMethodsFactory: (
    StatusManager,
    LineaRpcProtocolIdGenerator,
    () -> PeerLookup,
    BeaconChain,
  ) -> RpcMethods = ::RpcMethods,
  private val qbftMessageSerDe: SerDe<QbftMessage> = MaruCompressorRLPSerDe(QbftMessageSerDe()),
) : P2PNetwork {
  private val log: Logger = LogManager.getLogger(this.javaClass)
  internal lateinit var maruPeerManager: MaruPeerManager
  private val topicIdGenerator = LineaMessageIdGenerator(chainId)
  private val sealedBlocksTopicId =
    topicIdGenerator.id(
      GossipMessageType.BEACON_BLOCK.name,
      Version.V1,
      Encoding.RLP_SNAPPY,
    )
  private val qbftTopicId =
    topicIdGenerator.id(
      GossipMessageType.QBFT.name,
      Version.V1,
      Encoding.RLP_SNAPPY,
    )
  private val sealedBlocksSubscriptionManager = SubscriptionManager<SealedBeaconBlock>()
  private val sealedBlocksTopicHandler =
    TopicHandlerWithInOrderDelivering(
      subscriptionManager = sealedBlocksSubscriptionManager,
      sequenceNumberExtractor = { it.beaconBlock.beaconBlockHeader.number },
      deserializer = serDe,
      topicId = sealedBlocksTopicId,
      isHandlingEnabled = isBlockImportEnabledProvider,
      nextExpectedSequenceNumberProvider = { beaconChain.getLatestBeaconState().beaconBlockHeader.number + 1UL },
    )
  private val qbftMessagesSubscriptionManager = SubscriptionManager<QbftMessage>()
  private val qbftMessagesTopicHandler =
    ImmediateTopicHandler(
      subscriptionManager = qbftMessagesSubscriptionManager,
      deserializer = qbftMessageSerDe,
      topicId = qbftTopicId,
    )
  private val broadcastMessageCounterFactory =
    metricsFacade.createCounterFactory(
      category = MaruMetricsCategory.P2P_NETWORK,
      name = "message.broadcast.counter",
      description = "Count of messages broadcasted over the P2P network",
    )

  private val metricTrackingExecutorFactory = MetricTrackingExecutorFactory(metricsSystem)
  private val asyncRunner = AsyncRunnerFactory.createDefault(metricTrackingExecutorFactory).create("maru", 2)

  private fun buildP2PNetwork(
    privateKeyBytes: ByteArray,
    p2pConfig: P2PConfig,
    besuMetricsSystem: BesuMetricsSystem,
  ): TekuLibP2PNetwork {
    val privateKey = unmarshalPrivateKey(privateKeyBytes)
    val rpcIdGenerator = LineaRpcProtocolIdGenerator(chainId)

    val reputationManager =
      MaruReputationManager(besuMetricsSystem, SystemTimeProvider(), this::isStaticPeer, p2pConfig.reputation)

    val rpcMethods = rpcMethodsFactory(statusManager, rpcIdGenerator, { maruPeerManager }, beaconChain)
    maruPeerManager = MaruPeerManager(
      maruPeerFactory = DefaultMaruPeerFactory(
        rpcMethods = rpcMethods,
        statusManager = statusManager,
        p2pConfig = p2pConfig,
      ),
      p2pConfig = p2pConfig,
      reputationManager = reputationManager,
      isStaticPeer = this::isStaticPeer,
    )

    return Libp2pNetworkFactory(LINEA_DOMAIN).build(
      privateKey = privateKey,
      ipAddress = p2pConfig.ipAddress,
      port = p2pConfig.port,
      sealedBlocksTopicHandler = sealedBlocksTopicHandler,
      sealedBlocksTopicId = sealedBlocksTopicId,
      qbftTopicHandler = qbftMessagesTopicHandler,
      qbftTopicId = qbftTopicId,
      rpcMethods = rpcMethods.all(),
      maruPeerManager = maruPeerManager,
      metricsSystem = besuMetricsSystem,
      asyncRunner = asyncRunner,
      reputationManager = reputationManager,
      gossipingConfig = p2pConfig.gossiping,
    )
  }

  private val builtNetwork: TekuLibP2PNetwork = buildP2PNetwork(privateKeyBytes, p2pConfig, metricsSystem)
  internal val p2pNetwork = builtNetwork.p2PNetwork
  private var discoveryService: MaruDiscoveryService? = null

  override val localNodeRecord: NodeRecord?
    get() = discoveryService?.getLocalNodeRecord()
  override val enr: String?
    get() = localNodeRecord?.asEnr()

  // TODO: We need to call the updateForkId method on the discovery service when the forkId changes internal
  private val peerLookup = builtNetwork.peerLookup
  private val executor: ScheduledExecutorService =
    Executors.newSingleThreadScheduledExecutor(
      Thread.ofVirtual().factory(),
    )
  private val delayedExecutor =
    SafeFuture.delayedExecutor(p2pConfig.reconnectDelay.inWholeMilliseconds, TimeUnit.MILLISECONDS, executor)
  private val staticPeerMap = ConcurrentHashMap<NodeId, MultiaddrPeerAddress>()

  override val nodeId: String = p2pNetwork.nodeId.toBase58()
  override val discoveryAddresses: List<String>
    get() = p2pNetwork.discoveryAddresses.getOrElse { emptyList() }
  override val nodeAddresses: List<String>
    get() = p2pNetwork.nodeAddresses

  private fun logEnr(nodeRecord: NodeRecord) {
    log.info(
      "tcpAddr={} udpAddr={} enr={} ",
      nodeRecord.tcpAddress.getOrNull(),
      nodeRecord.udpAddress.getOrNull(),
      nodeRecord.asEnr(),
    )
  }

  override fun start(): SafeFuture<Unit> =
    p2pNetwork
      .start()
      .thenApply {
        log.info(
          "starting P2P network: nodeId={}",
          p2pNetwork.nodeId,
        )
        p2pConfig.staticPeers.forEach { peer ->
          p2pNetwork
            .createPeerAddress(peer)
            ?.let { address -> addStaticPeer(address as MultiaddrPeerAddress) }
        }
        discoveryService = p2pConfig.discovery?.let { discoveryConfig ->
          MaruDiscoveryService(
            privateKeyBytes = privateKeyBytesWithoutPrefix(privateKeyBytes),
            p2pConfig = p2pConfig.copy(
              port = port, // use actual listening port
              discovery = if (discoveryConfig.port == 0u) discoveryConfig.copy(port = port) else discoveryConfig,
            ),
            forkIdHashManager = forkIdHashManager,
            p2PState = p2PState,
            timerFactory = timerFactory,
          )
        }
        discoveryService?.start()
        maruPeerManager.start(discoveryService, p2pNetwork)
        metricsFacade.createGauge(
          category = MaruMetricsCategory.P2P_NETWORK,
          name = "peer.count",
          description = "Number of peers connected to the P2P network",
          measurementSupplier = { peerCount.toLong() },
        )
        nodeRecords().forEach(::logEnr)
      }

  fun nodeRecords(): List<NodeRecord> {
    val enrs: List<NodeRecord> =
      (listIpsV4(excludeLoopback = true) + discoveryAddresses)
        .toSet()
        .map {
          ENR.nodeRecord(
            privateKeyBytes = privateKeyBytes,
            seq = 0,
            ipv4 = it,
            ipv4UdpPort = p2pConfig.discovery?.port?.toInt() ?: port.toInt(),
            ipv4TcpPort = port.toInt(),
          )
        }
    return enrs + listOfNotNull(localNodeRecord)
  }

  override fun stop(): SafeFuture<Unit> {
    log.info("Stopping={}", this::class.simpleName)
    maruPeerManager.stop()
    return discoveryService?.stop()?.toSafeFuture().let { discoveryServiceStopFuture ->
      if (discoveryServiceStopFuture != null) {
        discoveryServiceStopFuture.thenCompose { p2pNetwork.stop() }.thenApply { }
      } else {
        p2pNetwork.stop().thenApply { }
      }
    }
  }

  override fun close() {
    asyncRunner.shutdown()
    executor.shutdown()
  }

  override fun broadcastMessage(message: Message<*, GossipMessageType>): SafeFuture<*> {
    log.trace("Broadcasting message={}", message)
    broadcastMessageCounterFactory
      .create(
        listOf(
          Tag("message.type", message.type.name),
          Tag("message.version", message.version.name),
        ),
      ).increment()
    return when (message.type) {
      GossipMessageType.QBFT -> {
        require(message.payload is QbftMessage) { "QBFT message payload must be QbftMessage" }
        val serializedQbftMessage = Bytes.wrap(qbftMessageSerDe.serialize(message.payload as QbftMessage))
        p2pNetwork.gossip(
          qbftTopicId,
          serializedQbftMessage,
        )
      }
      GossipMessageType.BEACON_BLOCK -> {
        require(message.payload is SealedBeaconBlock)
        val serializedSealedBeaconBlock = Bytes.wrap(serDe.serialize(message.payload as SealedBeaconBlock))
        p2pNetwork.gossip(
          topicIdGenerator.id(message.type.name, message.version, Encoding.RLP_SNAPPY),
          serializedSealedBeaconBlock,
        )
      }
    }
  }

  override fun subscribeToBlocks(subscriber: SealedBeaconBlockHandler<ValidationResult>): Int {
    log.trace("Subscribing on new sealed blocks")
    val subscriptionManagerHadSubscriptions = sealedBlocksSubscriptionManager.hasSubscriptions()

    return sealedBlocksSubscriptionManager.subscribe(subscriber::handleSealedBlock).also {
      if (!subscriptionManagerHadSubscriptions) {
        log.trace(
          "First ever subscription on new sealed blocks topicId={}. Subscribing on network level",
          sealedBlocksTopicId,
        )
        p2pNetwork.subscribe(sealedBlocksTopicId, sealedBlocksTopicHandler)
      }
    }
  }

  /**
   * Unsubscribes the handler with a given subscriptionId from sealed block handling.
   * Note that it's impossible to unsubscribe from a topic on LibP2P level, so the messages will still be received and
   * handled by LibP2P, but not processed by Maru
   */
  override fun unsubscribeFromBlocks(subscriptionId: Int) = sealedBlocksSubscriptionManager.unsubscribe(subscriptionId)

  override fun subscribeToQbftMessages(subscriber: QbftMessageHandler<ValidationResult>): Int {
    log.trace("Subscribing to QBFT messages")
    val subscriptionManagerHadSubscriptions = qbftMessagesSubscriptionManager.hasSubscriptions()

    return qbftMessagesSubscriptionManager.subscribe(subscriber::handleQbftMessage).also {
      if (!subscriptionManagerHadSubscriptions) {
        log.trace(
          "First ever subscription to QBFT messages topicId={}. Subscribing on network level",
          qbftTopicId,
        )
        p2pNetwork.subscribe(qbftTopicId, qbftMessagesTopicHandler)
      }
    }
  }

  override fun unsubscribeFromQbftMessages(subscriptionId: Int) =
    qbftMessagesSubscriptionManager.unsubscribe(subscriptionId)

  fun addStaticPeer(peerAddress: MultiaddrPeerAddress) {
    if (peerAddress.id == p2pNetwork.nodeId) { // Don't connect to self
      log.debug("Not adding static peer as it is the local node")
      return
    }
    synchronized(this) {
      if (staticPeerMap.containsKey(peerAddress.id)) {
        return
      }
      staticPeerMap[peerAddress.id] = peerAddress
    }
    maintainPersistentConnection(peerAddress)
  }

  override fun isStaticPeer(nodeId: NodeId): Boolean = staticPeerMap.containsKey(nodeId)

  fun removeStaticPeer(peerAddress: PeerAddress) {
    synchronized(this) {
      staticPeerMap.remove(peerAddress.id)
      p2pNetwork.getPeer(peerAddress.id).ifPresent { peer -> peer.disconnectImmediately(Optional.empty(), true) }
    }
  }

  private fun maintainPersistentConnection(peerAddress: MultiaddrPeerAddress): SafeFuture<Unit> =
    p2pNetwork
      .connect(peerAddress)
      .whenComplete { peer: Peer?, t: Throwable? ->
        if (t != null) {
          if (t.cause is PeerAlreadyConnectedException) {
            log.trace("Already connected to peer={}. errorMessage={}", peerAddress, t.message)
            reconnectWhenDisconnected(peer!!, peerAddress)
          } else {
            log.debug(
              "failed to connect to static peer={} retrying after {}. errorMessage={}",
              peerAddress,
              p2pConfig.reconnectDelay,
              t.message,
              t,
            )
            if (t.cause?.message != "Transport is closed") {
              SafeFuture
                .runAsync({ maintainPersistentConnection(peerAddress) }, delayedExecutor)
            }
          }
        } else {
          log.trace("Created persistent connection to peer={}", peerAddress)
          reconnectWhenDisconnected(peer!!, peerAddress)
        }
      }.thenApply {}

  private fun reconnectWhenDisconnected(
    peer: Peer,
    peerAddress: MultiaddrPeerAddress,
  ) {
    peer.subscribeDisconnect { _: Optional<DisconnectReason>, _: Boolean ->
      if (staticPeerMap.containsKey(peerAddress.id)) {
        SafeFuture.runAsync({ maintainPersistentConnection(peerAddress) }, delayedExecutor)
      }
    }
  }

  override val port
    get(): UInt =
      builtNetwork.host.network.transports
        .first()
        .listenAddresses()
        .first()
        .components
        .last()
        .stringValue!!
        .toUInt()

  override val peerCount: Int
    get() = maruPeerManager.peerCount

  internal fun isConnected(peer: String): Boolean =
    maruPeerManager.getPeer(LibP2PNodeId(PeerId.fromBase58(peer))) != null

  internal fun dropPeer(
    peer: String,
    reason: DisconnectReason,
  ): SafeFuture<Unit> {
    val maybePeer =
      p2pNetwork
        .getPeer(LibP2PNodeId(PeerId.fromBase58(peer)))
        .getOrNull()
    return if (maybePeer == null) {
      log.warn("Trying to disconnect from peer={}, but there's no connection to it!", peer)
      SafeFuture.completedFuture(Unit)
    } else {
      maybePeer.disconnectCleanly(reason).thenApply { }
    }
  }

  override fun getPeers(): List<PeerInfo> = peerLookup.getPeers().map { it.toPeerInfo() }

  override fun getPeer(peerId: String): PeerInfo? =
    peerLookup.getPeer(LibP2PNodeId(PeerId.fromBase58(peerId)))?.toPeerInfo()

  override fun getPeerLookup(): PeerLookup = peerLookup

  override fun dropPeer(peer: PeerInfo) {
    staticPeerMap[LibP2PNodeId(PeerId.fromBase58(peer.nodeId))]?.let { staticPeer ->
      removeStaticPeer(staticPeer)
    } ?: dropPeer(
      peer = peer.nodeId,
      reason = DisconnectReason.SHUTTING_DOWN,
    ).get()
  }

  override fun addPeer(address: String) {
    addStaticPeer(MultiaddrPeerAddress.fromAddress(address))
  }

  override fun handleForkTransition(forkSpec: ForkSpec) {
    discoveryService?.updateForkIdHash(forkIdHashManager.currentForkHash())
  }
}
