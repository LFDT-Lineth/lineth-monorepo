/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.config.QbftConfig
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.NextBlockTimestampProvider
import maru.consensus.PrevRandaoProvider
import maru.consensus.PrevRandaoProviderImpl
import maru.consensus.ProtocolFactory
import maru.consensus.QbftConsensusConfig
import maru.consensus.StaticValidatorProvider
import maru.consensus.blockimport.BlockBuildingBeaconBlockImporter
import maru.consensus.blockimport.SealedBeaconBlockImporter
import maru.consensus.blockimport.TransactionalSealedBeaconBlockImporter
import maru.consensus.qbft.adapters.ForksScheduleAdapter
import maru.consensus.qbft.adapters.P2PValidatorMulticaster
import maru.consensus.qbft.adapters.ProposerSelectorAdapter
import maru.consensus.qbft.adapters.QbftBlockCodecAdapter
import maru.consensus.qbft.adapters.QbftBlockImporterAdapter
import maru.consensus.qbft.adapters.QbftBlockInterfaceAdapter
import maru.consensus.qbft.adapters.QbftBlockchainAdapter
import maru.consensus.qbft.adapters.QbftFinalStateAdapter
import maru.consensus.qbft.adapters.QbftProtocolScheduleAdapter
import maru.consensus.qbft.adapters.QbftValidatorModeTransitionLoggerAdapter
import maru.consensus.qbft.adapters.QbftValidatorProviderAdapter
import maru.consensus.qbft.adapters.toSealedBeaconBlock
import maru.consensus.state.FinalizationProvider
import maru.consensus.state.StateTransition
import maru.consensus.state.StateTransitionImpl
import maru.consensus.validation.BeaconBlockValidatorFactoryImpl
import maru.core.BeaconState
import maru.core.Protocol
import maru.core.SealedBeaconBlock
import maru.core.Validator
import maru.crypto.Hashing
import maru.crypto.SecpCrypto
import maru.crypto.Signing
import maru.database.BeaconChain
import maru.executionlayer.manager.ExecutionLayerManager
import maru.p2p.P2PNetwork
import maru.p2p.SealedBeaconBlockHandler
import maru.p2p.ValidationResult
import maru.syncing.SyncStatusProvider
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.consensus.common.bft.BftEventQueue
import org.hyperledger.besu.consensus.common.bft.BftExecutors
import org.hyperledger.besu.consensus.common.bft.BlockTimer
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.MessageTracker
import org.hyperledger.besu.consensus.common.bft.RoundTimer
import org.hyperledger.besu.consensus.common.bft.statemachine.FutureMessageBuffer
import org.hyperledger.besu.consensus.qbft.core.payload.MessageFactory
import org.hyperledger.besu.consensus.qbft.core.statemachine.QbftBlockHeightManagerFactory
import org.hyperledger.besu.consensus.qbft.core.statemachine.QbftController
import org.hyperledger.besu.consensus.qbft.core.statemachine.QbftRoundFactory
import org.hyperledger.besu.consensus.qbft.core.types.QbftMessage
import org.hyperledger.besu.consensus.qbft.core.types.QbftMinedBlockObserver
import org.hyperledger.besu.consensus.qbft.core.types.QbftNewChainHead
import org.hyperledger.besu.consensus.qbft.core.validation.MessageValidatorFactory
import org.hyperledger.besu.cryptoservices.KeyPairSecurityModule
import org.hyperledger.besu.cryptoservices.NodeKey
import org.hyperledger.besu.ethereum.core.Util
import org.hyperledger.besu.plugin.services.MetricsSystem
import org.hyperledger.besu.util.Subscribers
import java.time.Clock
import java.util.concurrent.Executors
import kotlin.time.Duration.Companion.seconds

class QbftValidatorFactory(
  private val beaconChain: BeaconChain,
  private val privateKeyBytes: ByteArray,
  private val qbftOptions: QbftConfig,
  private val metricsSystem: MetricsSystem,
  private val finalizationStateProvider: FinalizationProvider,
  private val nextBlockTimestampProvider: NextBlockTimestampProvider,
  private val newBlockHandler: SealedBeaconBlockHandler<*>,
  private val executionLayerManager: ExecutionLayerManager,
  private val clock: Clock,
  private val p2PNetwork: P2PNetwork,
  private val allowEmptyBlocks: Boolean,
  private val forksSchedule: ForksSchedule,
  private val payloadValidationEnabled: Boolean,
  /** Optional: called when BLOCK_TIMER_EXPIRY fires. See [QbftEventMultiplexer.onBlockTimerFired]. */
  private val onBlockTimerFired: ((blockNumber: Long) -> Unit)? = null,
  /**
   * Optional: called when a QBFT message arrives from P2P, before queue insertion.
   * See [QbftMessageProcessor.onMessageReceived].
   */
  private val onMessageReceived: ((msgCode: Int, sequenceNumber: Long) -> Unit)? = null,
  /** Optional: called when a block is committed by the QBFT consensus (mined). */
  private val onBlockMined: ((SealedBeaconBlock) -> Unit)? = null,
  /** Sync status provider for registering beacon sync completion callbacks. */
  private val syncStatusProvider: SyncStatusProvider,
) : ProtocolFactory {
  override fun create(forkSpec: ForkSpec): Protocol {
    val protocolConfig = forkSpec.configuration as QbftConsensusConfig
    val signatureAlgorithm = SecpCrypto.signatureAlgorithm
    val privateKey = signatureAlgorithm.createPrivateKey(Bytes32.wrap(privateKeyBytes))
    val keyPair = signatureAlgorithm.createKeyPair(privateKey)
    val securityModule = KeyPairSecurityModule(keyPair)
    val nodeKey = NodeKey(securityModule)
    val blockChain = QbftBlockchainAdapter(beaconChain)

    val localAddress = Util.publicKeyToAddress(keyPair.publicKey)
    val qbftProposerSelector = ProposerSelectorAdapter(beaconChain, ProposerSelectorImpl)

    val validatorProvider = StaticValidatorProvider(protocolConfig.validatorSet)
    val stateTransition = StateTransitionImpl(validatorProvider)
    val proposerSelector = ProposerSelectorImpl
    val besuValidatorProvider = QbftValidatorProviderAdapter(validatorProvider)
    val localValidator = Validator(localAddress.bytes.toArray())
    val prevRandaoProvider =
      PrevRandaoProviderImpl(
        signer = Signing.ULongSigner(nodeKey),
        hasher = Hashing::keccak,
      )
    val sealedBeaconBlockImporter =
      createSealedBeaconBlockImporter(
        executionLayerManager = executionLayerManager,
        beaconChain = beaconChain,
        stateTransition = stateTransition,
        finalizationStateProvider = finalizationStateProvider,
        prevRandaoProvider = prevRandaoProvider,
        feeRecipient = qbftOptions.feeRecipient,
        localValidator = localValidator,
        proposerSelector = proposerSelector,
      )

    val qbftBlockCreatorFactory =
      QbftBlockCreatorFactory(
        manager = executionLayerManager,
        proposerSelector = qbftProposerSelector,
        validatorProvider = validatorProvider,
        beaconChain = beaconChain,
        finalizationStateProvider = finalizationStateProvider,
        prevRandaoProvider = prevRandaoProvider,
        feeRecipient = qbftOptions.feeRecipient,
        eagerQbftBlockCreatorConfig = EagerQbftBlockCreator.Config(qbftOptions.minBlockBuildTime),
      )

    val besuForksSchedule = ForksScheduleAdapter(forkSpec, qbftOptions)

    val bftExecutors = BftExecutors.create(metricsSystem, BftExecutors.ConsensusType.QBFT)
    val bftEventQueue = BftEventQueue(qbftOptions.messageQueueLimit)
    val roundExpiry = qbftOptions.roundExpiry ?: forkSpec.blockTimeSeconds.toInt().seconds
    val roundTimeExpiryCalculator =
      if (protocolConfig.validatorSet.size == 1) {
        ConstantRoundTimeExpiryCalculator((roundExpiry))
      } else {
        LinearRoundTimeExpiryCalculator(roundExpiry, qbftOptions.roundExpiryCoefficient)
      }
    val roundTimer =
      RoundTimer(
        /* queue = */
        bftEventQueue,
        /* roundExpiryTimeCalculator = */
        roundTimeExpiryCalculator,
        /* bftExecutors = */
        bftExecutors,
      )
    val blockTimer = BlockTimer(bftEventQueue, besuForksSchedule, bftExecutors, clock)
    val validatorMulticaster = P2PValidatorMulticaster(p2PNetwork)
    val finalState =
      QbftFinalStateAdapter(
        localAddress = localAddress,
        nodeKey = nodeKey,
        proposerSelector = qbftProposerSelector,
        validatorMulticaster = validatorMulticaster,
        roundTimer = roundTimer,
        blockTimer = blockTimer,
        blockCreatorFactory = qbftBlockCreatorFactory,
        clock = clock,
        beaconChain = beaconChain,
      )

    val minedBlockObservers = Subscribers.create<QbftMinedBlockObserver>()
    minedBlockObservers.subscribe { qbftBlock ->
      val sealedBlock = qbftBlock.toSealedBeaconBlock()
      newBlockHandler.handleSealedBlock(sealedBlock)
      bftEventQueue.add(QbftNewChainHead(qbftBlock.header))
      onBlockMined?.invoke(sealedBlock)
    }

    val blockImporter =
      QbftBlockImporterAdapter(sealedBeaconBlockImporter)

    val blockCodec = QbftBlockCodecAdapter
    val blockInterface = QbftBlockInterfaceAdapter(stateTransition)
    val beaconBlockValidatorFactory =
      BeaconBlockValidatorFactoryImpl(
        beaconChain = beaconChain,
        proposerSelector = proposerSelector,
        stateTransition = stateTransition,
        executionLayerManager = if (payloadValidationEnabled) executionLayerManager else null,
        allowEmptyBlocks = allowEmptyBlocks,
      )
    val protocolSchedule =
      QbftProtocolScheduleAdapter(
        blockImporter = blockImporter,
        beaconBlockValidatorFactory = beaconBlockValidatorFactory,
      )
    val messageValidatorFactory =
      MessageValidatorFactory(
        /* proposerSelector = */
        qbftProposerSelector,
        /* protocolSchedule = */
        protocolSchedule,
        /* validatorProvider = */
        besuValidatorProvider,
        /* blockInterface = */
        blockInterface,
      )
    val messageFactory = MessageFactory(nodeKey, blockCodec)
    val qbftRoundFactory =
      QbftRoundFactory(
        /* finalState = */
        finalState,
        /* blockInterface = */
        blockInterface,
        /* protocolSchedule = */
        protocolSchedule,
        /* minedBlockObservers = */
        minedBlockObservers,
        /* messageValidatorFactory = */
        messageValidatorFactory,
        /* messageFactory = */
        messageFactory,
      )

    val transitionLogger = QbftValidatorModeTransitionLoggerAdapter()
    val qbftBlockHeightManagerFactory =
      QbftBlockHeightManagerFactory(
        /* finalState = */
        finalState,
        /* roundFactory = */
        qbftRoundFactory,
        /* messageValidatorFactory = */
        messageValidatorFactory,
        /* messageFactory = */
        messageFactory,
        /* validatorProvider = */
        besuValidatorProvider,
        /* validatorModeTransitionLogger = */
        transitionLogger,
      )
    val duplicateMessageTracker = MessageTracker(qbftOptions.duplicateMessageLimit)
    val chainHeaderNumber =
      beaconChain
        .getLatestBeaconState()
        .beaconBlockHeader
        .number
        .toLong()
    val futureMessageBuffer =
      FutureMessageBuffer<QbftMessage>(
        /* futureMessagesMaxDistance = */
        qbftOptions.futureMessageMaxDistance,
        /* futureMessagesLimit = */
        qbftOptions.futureMessagesLimit,
        /* chainHeight = */
        chainHeaderNumber,
      )
    val gossiper = QbftGossiper()
    val qbftController =
      QbftController(
        /* blockchain = */
        blockChain,
        /* finalState = */
        finalState,
        /* qbftBlockHeightManagerFactory = */
        qbftBlockHeightManagerFactory,
        /* gossiper = */
        gossiper,
        /* duplicateMessageTracker = */
        duplicateMessageTracker,
        /* futureMessageBuffer = */
        futureMessageBuffer,
        /* blockEncoder = */
        blockCodec,
      )

    val eventMultiplexer =
      QbftEventMultiplexer(qbftController).also {
        it.onBlockTimerFired = onBlockTimerFired
      }
    val eventProcessor = QbftEventProcessor(bftEventQueue, eventMultiplexer)
    val eventQueueExecutor =
      Executors.newSingleThreadExecutor(
        Thread
          .ofPlatform()
          .name("qbft-event-loop-${localAddress.bytes.toHexString().takeLast(8)}")
          .daemon(true)
          .factory(),
      )

    val messageDecoder = MinimalQbftMessageDecoder(SecpCrypto)
    val qbftMessageProcessor =
      QbftMessageProcessor(
        blockChain = blockChain,
        validatorProvider = besuValidatorProvider,
        localAddress = localAddress,
        bftEventQueue = bftEventQueue,
        messageDecoder = messageDecoder,
      ).also {
        it.onMessageReceived = onMessageReceived
      }

    // Subscribe to QBFT messages from P2P network and validate before adding to event queue
    p2PNetwork.subscribeToQbftMessages(qbftMessageProcessor)

    // When the CL sync pipeline completes, notify QBFT that the chain head may have advanced
    // externally (e.g., blocks imported from other validators while this node was out of the mesh).
    // Without this, QBFT stays at the stale height because QbftNewChainHead events are only
    // emitted when QBFT commits a block through its own consensus.
    syncStatusProvider.onBeaconSyncComplete {
      bftEventQueue.add(QbftNewChainHead(blockChain.chainHeadHeader))
    }

    return QbftConsensusValidator(
      qbftController = qbftController,
      eventProcessor = eventProcessor,
      bftExecutors = bftExecutors,
      eventQueueExecutor = eventQueueExecutor,
    )
  }

  private fun createSealedBeaconBlockImporter(
    executionLayerManager: ExecutionLayerManager,
    beaconChain: BeaconChain,
    stateTransition: StateTransition,
    finalizationStateProvider: FinalizationProvider,
    prevRandaoProvider: PrevRandaoProvider<ULong>,
    feeRecipient: ByteArray,
    localValidator: Validator,
    proposerSelector: ProposerSelector,
  ): SealedBeaconBlockImporter<ValidationResult> {
    val shouldBuildNextBlock =
      { beaconState: BeaconState, nextBlockRoundIdentifier: ConsensusRoundIdentifier, nextBlockTimestamp: ULong ->
        // We shouldn't build next block if this fork ends.
        val nextForkTimestamp =
          forksSchedule.getNextForkByTimestamp(beaconState.beaconBlockHeader.timestamp)?.timestampSeconds
            ?: ULong.MAX_VALUE
        if (nextBlockTimestamp >= nextForkTimestamp) {
          false
        } else {
          // Build only if this node is the round-0 proposer for the next block.
          // Round-change proposers (round 1+) are covered by EagerQbftBlockCreator which
          // sends FCU and waits for the block to build before delegating to DelayedQbftBlockCreator.
          val round0Proposer =
            proposerSelector
              .getProposerForBlock(beaconState, ConsensusRoundIdentifier(nextBlockRoundIdentifier.sequenceNumber, 0))
              .get() // ProposerSelectorImpl is synchronous (pure computation)
          localValidator.address.contentEquals(round0Proposer.address)
        }
      }
    val beaconBlockImporter =
      BlockBuildingBeaconBlockImporter(
        executionLayerManager = executionLayerManager,
        finalizationStateProvider = finalizationStateProvider,
        nextBlockTimestampProvider = nextBlockTimestampProvider,
        prevRandaoProvider = prevRandaoProvider,
        shouldBuildNextBlock = shouldBuildNextBlock,
        feeRecipient = feeRecipient,
      )
    return TransactionalSealedBeaconBlockImporter(
      beaconChain = beaconChain,
      stateTransition = stateTransition,
      beaconBlockImporter = beaconBlockImporter,
    )
  }
}
