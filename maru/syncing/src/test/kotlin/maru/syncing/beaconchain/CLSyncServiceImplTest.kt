/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.syncing.beaconchain

import linea.timer.JvmTimerFactory
import linea.timer.TimerFactory
import maru.config.P2PConfig
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ConsensusConfig
import maru.consensus.ElFork
import maru.consensus.ForkIdManagerFactory
import maru.consensus.QbftConsensusConfig
import maru.consensus.StaticValidatorProvider
import maru.consensus.qbft.DelayedQbftBlockCreator
import maru.core.BeaconState
import maru.core.Seal
import maru.core.SealedBeaconBlock
import maru.core.Validator
import maru.core.ext.DataGenerators
import maru.core.ext.metrics.TestMetrics.TestMetricsFacade
import maru.core.ext.metrics.TestMetrics.TestMetricsSystemAdapter
import maru.crypto.SecpCrypto
import maru.database.BeaconChain
import maru.database.InMemoryBeaconChain
import maru.database.InMemoryP2PState
import maru.database.P2PState
import maru.extensions.fromHexToByteArray
import maru.p2p.P2PNetworkImpl
import maru.p2p.PeerLookup
import maru.p2p.fork.ForkPeeringManager
import maru.p2p.messages.StatusManager
import maru.serialization.rlp.RLPSerializers
import maru.syncing.beaconchain.pipeline.BeaconChainDownloadPipelineFactory.Config
import net.consensys.linea.metrics.Counter
import net.consensys.linea.metrics.MetricsFacade
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.hyperledger.besu.crypto.KeyPair
import org.hyperledger.besu.crypto.SignatureAlgorithm
import org.hyperledger.besu.ethereum.core.Util
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.mockito.Mockito.mock
import org.mockito.Mockito.times
import org.mockito.kotlin.any
import org.mockito.kotlin.doAnswer
import org.mockito.kotlin.never
import org.mockito.kotlin.reset
import org.mockito.kotlin.spy
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.networking.p2p.libp2p.MultiaddrPeerAddress
import java.util.SequencedSet
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import kotlin.time.Duration.Companion.seconds

class CLSyncServiceImplTest {
  companion object {
    private const val CHAIN_ID = 1337u
    private const val IPV4 = "127.0.0.1"
    private const val PEER_ID_NODE_2: String = "16Uiu2HAmVXtqhevTAJqZucPbR2W4nCMpetrQASgjZpcxDEDaUPPt"
    private const val BEACON_CHAIN_2_HEAD = 100UL
    private val targetNodeKey =
      "0x0802122012c0b113e2b0c37388e2b484112e13f05c92c4471e3ee1dfaa368fa5045325b2"
        .fromHexToByteArray()
    private val sourceNodeKey =
      "0x0802122100f3d2fffa99dc8906823866d96316492ebf7a8478713a89a58b7385af85b088a1"
        .fromHexToByteArray()
    private val backoffDelay = 1.seconds

    fun createForkIdHashProvider(beaconChain: BeaconChain): ForkPeeringManager {
      val consensusConfig: ConsensusConfig =
        QbftConsensusConfig(
          validatorSet = setOf(
            Validator(ByteArray(20) { 0 }),
            Validator(ByteArray(20) { 1 }),
          ),
          fork = ChainFork(ClFork.QBFT_PHASE0, elFork = ElFork.Prague),
        )

      return ForkIdManagerFactory.createForkIdHashManager(
        chainId = CHAIN_ID,
        beaconChain = beaconChain,
        elFork = ElFork.Prague,
        consensusConfig = consensusConfig,
      )
    }
  }

  private val synced = AtomicBoolean(false)
  private lateinit var signatureAlgorithm: SignatureAlgorithm
  private lateinit var keypair: KeyPair
  private lateinit var targetBeaconChain: BeaconChain
  private lateinit var sourceBeaconChain: BeaconChain
  private lateinit var validators: SequencedSet<Validator>
  private lateinit var targetP2pNetwork: P2PNetworkImpl
  private lateinit var sourceP2pNetwork: P2PNetworkImpl
  private lateinit var clSyncService: CLSyncServiceImpl
  private lateinit var peerLookup: PeerLookup
  private lateinit var executorService: ExecutorService
  private val defaultPipelineConfig =
    Config(
      blockRangeRequestTimeout = 5.seconds,
      blocksBatchSize = 10u,
      blocksParallelism = 1u,
      backoffDelay = backoffDelay,
      maxRetries = 5u,
      useUnconditionalRandomDownloadPeer = false,
    )

  @BeforeEach
  fun setUp() {
    signatureAlgorithm = SecpCrypto.signatureAlgorithm
    keypair = signatureAlgorithm.generateKeyPair()
    validators = sortedSetOf(Validator(Util.publicKeyToAddress(keypair.publicKey).bytes.toArray()))

    val genesisTimestamp = DataGenerators.randomTimestamp()
    val (genesisBeaconState, genesisBeaconBlock) = DataGenerators.genesisState(genesisTimestamp, validators)
    targetBeaconChain = spy(InMemoryBeaconChain(genesisBeaconState, genesisBeaconBlock))
    sourceBeaconChain = spy(InMemoryBeaconChain(genesisBeaconState, genesisBeaconBlock))

    targetP2pNetwork = createNetwork(
      targetBeaconChain,
      targetNodeKey,
      0u,
      InMemoryP2PState(),
      JvmTimerFactory(),
    )
    sourceP2pNetwork = createNetwork(
      sourceBeaconChain,
      sourceNodeKey,
      0u,
      InMemoryP2PState(),
      JvmTimerFactory(),
    )

    createBlocks(
      beaconChain = sourceBeaconChain,
      genesisBeaconBlock = genesisBeaconBlock,
      genesisTimestamp = genesisBeaconBlock.beaconBlock.beaconBlockHeader.timestamp,
      validators = validators,
      signatureAlgorithm = signatureAlgorithm,
      keypair = keypair,
    )
    peerLookup = spy(targetP2pNetwork.getPeerLookup())
    executorService = Executors.newCachedThreadPool()
    clSyncService = CLSyncServiceImpl(
      beaconChain = targetBeaconChain,
      executorService = executorService,
      validatorProvider = StaticValidatorProvider(validators),
      allowEmptyBlocks = true,
      peerLookup = peerLookup,
      besuMetrics = TestMetricsSystemAdapter,
      metricsFacade = TestMetricsFacade,
      pipelineConfig = defaultPipelineConfig,
    )

    try {
      targetP2pNetwork.start().get()
      sourceP2pNetwork.start().get()
      targetP2pNetwork.addStaticPeer(createPeerAddress(sourceP2pNetwork.port))

      awaitUntilAsserted(timeout = 30L, timeUnit = TimeUnit.SECONDS) {
        assertNetworkHasPeers(
          network = targetP2pNetwork,
          peers = 1,
        )
      }
      awaitUntilAsserted(timeout = 30L, timeUnit = TimeUnit.SECONDS) {
        assertNetworkHasPeers(
          network = sourceP2pNetwork,
          peers = 1,
        )
      }
    } catch (e: Exception) {
      targetP2pNetwork.stop()
      sourceP2pNetwork.stop()
      throw IllegalStateException("Failed to start P2P networks", e)
    }
  }

  @AfterEach
  fun tearDown() {
    clSyncService.stop()
    targetP2pNetwork.stop()
    sourceP2pNetwork.stop()
    executorService.shutdownNow()
    executorService.awaitTermination(5, TimeUnit.SECONDS)
  }

  @Test
  fun `chain sync downloads and imports blocks from another node`() {
    syncToTarget(BEACON_CHAIN_2_HEAD)
    verifyChain(BEACON_CHAIN_2_HEAD, sourceBeaconChain.getLatestBeaconState())
  }

  @Test
  fun `sync target can be updated ahead of current target and continue downloading`() {
    // sync to block 50
    syncToTarget(50UL)
    verifyChain(50UL, sourceBeaconChain.getBeaconState(50uL)!!)

    // update sync target to BEACON_CHAIN_2_HEAD
    synced.set(false)
    syncToTarget(BEACON_CHAIN_2_HEAD)
    verifyChain(BEACON_CHAIN_2_HEAD, sourceBeaconChain.getLatestBeaconState())
  }

  @Test
  fun `chain sync download restarts on errors`() {
    val peerLookup = spy(targetP2pNetwork.getPeerLookup())

    // Fail the first two calls to getPeers() to simulate failure getting peers and not downloading blocks
    val expectedRetries = 2
    var retries = 0
    doAnswer {
      if (retries < expectedRetries) {
        retries++
        throw IllegalStateException("Simulated failure for testing")
      } else {
        it.callRealMethod()
      }
    }.whenever(peerLookup).getPeers()

    val metricsFacade = mock(MetricsFacade::class.java)
    val retriesCounter = mock(Counter::class.java)
    whenever(metricsFacade.createCounter(any(), any(), any(), any())).thenReturn(retriesCounter)

    val restartClSyncService =
      CLSyncServiceImpl(
        beaconChain = targetBeaconChain,
        executorService = Executors.newCachedThreadPool(),
        validatorProvider = StaticValidatorProvider(validators),
        allowEmptyBlocks = true,
        peerLookup = peerLookup,
        besuMetrics = TestMetricsSystemAdapter,
        metricsFacade = metricsFacade,
        pipelineConfig = defaultPipelineConfig,
      )

    syncToTarget(BEACON_CHAIN_2_HEAD, restartClSyncService)
    assertThat(retries).isEqualTo(2)
    verifyChain(BEACON_CHAIN_2_HEAD, sourceBeaconChain.getLatestBeaconState())
    verify(retriesCounter, times(expectedRetries)).increment()
  }

  @Test
  fun `sync target set to same target returns immediately`() {
    // sync to block 50
    syncToTarget(50UL)
    verifyChain(50UL, sourceBeaconChain.getBeaconState(50uL)!!)

    // update the sync target to 50 again
    synced.set(false)
    reset(targetBeaconChain, sourceBeaconChain)
    syncToTarget(50UL)

    // Ensure the state remains unchanged
    verifyChain(50UL, sourceBeaconChain.getBeaconState(50uL)!!)

    // Verify that beaconChain for node1 was not updated, and there are no reads on node2 beaconChain
    verify(targetBeaconChain, never()).newBeaconChainUpdater()
    verify(sourceBeaconChain, never()).getSealedBeaconBlocks(any(), any())
  }

  @Test
  fun `chain syncs to updated lower sync target`() {
    clSyncService.start()
    clSyncService.onSyncComplete { synced.set(true) }
    clSyncService.setSyncTarget(150UL)
    clSyncService.setSyncTarget(100UL)
    awaitUntilAsserted { assertThat(synced).isTrue() }

    verifyChain(100UL, sourceBeaconChain.getBeaconState(100UL)!!)
  }

  @Test
  fun `sync target set to older target returns immediately`() {
    // sync to block 50
    syncToTarget(50UL)
    verifyChain(50UL, sourceBeaconChain.getBeaconState(50uL)!!)

    // update the sync target to 40, should return immediately
    synced.set(false)
    reset(targetBeaconChain, sourceBeaconChain)
    syncToTarget(40uL)

    // Ensure the state remains unchanged
    verifyChain(50UL, sourceBeaconChain.getBeaconState(50uL)!!)

    // Verify that beaconChain for node1 was not updated, and there are no reads on node2 beaconChain
    verify(targetBeaconChain, never()).newBeaconChainUpdater()
    verify(sourceBeaconChain, never()).getSealedBeaconBlocks(any(), any())
  }

  @Test
  fun `chain sync does not download after stopped is called`() {
    clSyncService.stop()
    assertThrows<IllegalStateException> { clSyncService.setSyncTarget(BEACON_CHAIN_2_HEAD) }
  }

  @Test
  fun `onSyncComplete handler is called only once per sync`() {
    var callCount = 0
    clSyncService.start()
    clSyncService.onSyncComplete { callCount++ }
    clSyncService.setSyncTarget(50uL)
    awaitUntilAsserted { assertThat(callCount).isEqualTo(1) }
  }

  @Test
  fun `onSyncComplete handler is called only once per sync - after call to same target`() {
    var callCount = 0
    clSyncService.start()
    clSyncService.onSyncComplete { callCount++ }
    clSyncService.setSyncTarget(50uL)
    clSyncService.setSyncTarget(50uL)
    awaitUntilAsserted { assertThat(callCount).isEqualTo(1) }
  }

  @Test
  fun `onSyncComplete handler is called only once per sync - after call to past target`() {
    var callCount = 0
    clSyncService.start()
    clSyncService.onSyncComplete { callCount++ }
    clSyncService.setSyncTarget(50uL)
    clSyncService.setSyncTarget(20uL)
    awaitUntilAsserted { assertThat(callCount).isEqualTo(1) }
  }

  @Test
  fun `onSyncComplete handler has expected sync target`() {
    var handlerResult = 0uL
    clSyncService.start()
    clSyncService.onSyncComplete { handlerResult = it }
    clSyncService.setSyncTarget(50uL)
    awaitUntilAsserted { assertThat(handlerResult).isEqualTo(50uL) }
  }

  @Test
  fun `onSyncComplete handler returns latest expected sync target`() {
    var handlerResult = 0uL
    clSyncService.start()
    clSyncService.onSyncComplete { handlerResult = it }
    clSyncService.setSyncTarget(50uL)
    clSyncService.setSyncTarget(100uL)
    awaitUntilAsserted { assertThat(handlerResult).isEqualTo(100uL) }
  }

  @Test
  fun `multiple onSyncComplete handlers are called`() {
    var handler1Called = false
    var handler2Called = false
    clSyncService.onSyncComplete { handler1Called = true }
    clSyncService.onSyncComplete { handler2Called = true }

    clSyncService.start()
    clSyncService.setSyncTarget(50uL)
    awaitUntilAsserted {
      assertThat(handler1Called).isTrue()
      assertThat(handler2Called).isTrue()
    }
  }

  private fun syncToTarget(
    syncTarget: ULong,
    clSyncService: CLSyncServiceImpl = this.clSyncService,
  ) {
    clSyncService.start()
    clSyncService.setSyncTarget(syncTarget)
    clSyncService.onSyncComplete { synced.set(true) }
    awaitUntilAsserted { assertThat(synced).isTrue() }
  }

  private fun verifyChain(
    expectedHeadBlockNumber: ULong,
    expectedHeadState: BeaconState,
  ) {
    assertThat(
      targetBeaconChain.getLatestBeaconState().beaconBlockHeader.number,
    ).isEqualTo(expectedHeadBlockNumber)
    assertThat(targetBeaconChain.getLatestBeaconState()).isEqualTo(expectedHeadState)
    for (i in 1uL..expectedHeadBlockNumber) {
      assertThat(targetBeaconChain.getSealedBeaconBlock(i)).isEqualTo(sourceBeaconChain.getSealedBeaconBlock(i))
      assertThat(targetBeaconChain.getBeaconState(i)).isEqualTo(sourceBeaconChain.getBeaconState(i))
    }
  }

  private fun createPeerAddress(port: UInt): MultiaddrPeerAddress =
    MultiaddrPeerAddress.fromAddress("/ip4/$IPV4/tcp/$port/p2p/$PEER_ID_NODE_2")

  private fun createNetwork(
    beaconChain: BeaconChain,
    key: ByteArray,
    port: UInt,
    p2PState: P2PState,
    timerFactory: TimerFactory,
  ): P2PNetworkImpl {
    val forkIdHashProvider = createForkIdHashProvider(beaconChain)
    val statusManager = StatusManager(beaconChain, forkIdHashProvider)
    val p2pNetworkImpl =
      P2PNetworkImpl(
        privateKeyBytes = key,
        p2pConfig = P2PConfig(
          ipAddress = IPV4,
          port = port,
          staticPeers = emptyList(),
        ),
        chainId = CHAIN_ID,
        serDe = RLPSerializers.SealedBeaconBlockSerializer,
        metricsFacade = TestMetricsFacade,
        statusManager = statusManager,
        beaconChain = beaconChain,
        metricsSystem = TestMetricsSystemAdapter,
        forkIdHashManager = forkIdHashProvider,
        isBlockImportEnabledProvider = { true },
        p2PState = p2PState,
        timerFactory = timerFactory,
      )
    return p2pNetworkImpl
  }

  private fun createBlocks(
    beaconChain: BeaconChain,
    genesisBeaconBlock: SealedBeaconBlock,
    genesisTimestamp: ULong,
    validators: SequencedSet<Validator>,
    signatureAlgorithm: SignatureAlgorithm,
    keypair: KeyPair,
  ) {
    val updater = beaconChain.newBeaconChainUpdater()
    var parentSealedBeaconBlock = genesisBeaconBlock
    for (i in 1uL..BEACON_CHAIN_2_HEAD) {
      val parentElBlockNumber = parentSealedBeaconBlock.beaconBlock.beaconBlockBody.executionPayload.blockNumber
      val executionPayload = DataGenerators.randomExecutionPayload().copy(blockNumber = parentElBlockNumber + 1u)

      val beaconBlock =
        DelayedQbftBlockCreator.createBeaconBlock(
          parentSealedBeaconBlock = parentSealedBeaconBlock,
          executionPayload = executionPayload,
          round = 0,
          timestamp = genesisTimestamp + i,
          proposer = validators.first().address,
          validators = validators,
        )
      val seal = signatureAlgorithm.sign(Bytes32.wrap(beaconBlock.beaconBlockHeader.hash), keypair)
      val sealedBlock =
        SealedBeaconBlock(
          beaconBlock = beaconBlock,
          setOf(Seal(seal.encodedBytes().toArray())),
        )
      val beaconState =
        BeaconState(
          beaconBlockHeader = beaconBlock.beaconBlockHeader,
          validators = validators,
        )
      updater.putSealedBeaconBlock(sealedBlock)
      updater.putBeaconState(beaconState)
      parentSealedBeaconBlock = sealedBlock
    }
    updater.commit()
  }

  private fun awaitUntilAsserted(
    timeout: Long = 6000L,
    timeUnit: TimeUnit = TimeUnit.MILLISECONDS,
    condition: () -> Unit,
  ) {
    await()
      .timeout(timeout, timeUnit)
      .untilAsserted(condition)
  }

  private fun assertNetworkHasPeers(
    network: P2PNetworkImpl,
    peers: Int,
  ) {
    assertThat(network.peerCount).isEqualTo(peers)
  }
}
