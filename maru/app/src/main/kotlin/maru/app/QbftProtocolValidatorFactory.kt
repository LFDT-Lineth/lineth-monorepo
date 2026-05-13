/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.config.QbftConfig
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.NextBlockTimestampProvider
import maru.consensus.ProtocolFactory
import maru.consensus.QbftConsensusConfig
import maru.consensus.SealedBeaconBlockHandlerAdapter
import maru.consensus.blockimport.NewSealedBeaconBlockHandlerMultiplexer
import maru.consensus.qbft.QbftEventMultiplexer
import maru.consensus.qbft.QbftValidatorFactory
import maru.consensus.state.FinalizationProvider
import maru.core.Protocol
import maru.core.SealedBeaconBlock
import maru.database.BeaconChain
import maru.executionlayer.ExecutionLayerFactory.buildExecutionLayerManager
import maru.p2p.P2PNetwork
import maru.p2p.SealedBeaconBlockBroadcaster
import maru.syncing.SyncStatusProvider
import net.consensys.linea.metrics.MetricsFacade
import org.hyperledger.besu.plugin.services.MetricsSystem
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient
import java.time.Clock

class QbftProtocolValidatorFactory(
  private val qbftOptions: QbftConfig,
  private val privateKeyBytes: ByteArray,
  private val validatorELNodeEngineApiWeb3JClient: Web3JClient,
  private val followerELNodeEngineApiWeb3JClients: Map<String, Web3JClient>,
  private val metricsSystem: MetricsSystem,
  private val finalizationStateProvider: FinalizationProvider,
  private val beaconChain: BeaconChain,
  private val nextTargetBlockTimestampProvider: NextBlockTimestampProvider,
  private val clock: Clock,
  private val p2pNetwork: P2PNetwork,
  private val metricsFacade: MetricsFacade,
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
  /** Optional: called when a block is committed by QBFT consensus. */
  private val onBlockMined: ((SealedBeaconBlock) -> Unit)? = null,
  /** Sync status provider for registering beacon sync completion callbacks. */
  private val syncStatusProvider: SyncStatusProvider,
) : ProtocolFactory {
  override fun create(forkSpec: ForkSpec): Protocol {
    require(forkSpec.configuration is QbftConsensusConfig) {
      "Unexpected fork specification! ${forkSpec.configuration} instead of ${QbftConsensusConfig::class.simpleName}"
    }
    val qbftConsensusConfig = forkSpec.configuration as QbftConsensusConfig

    val payloadValidatorExecutionLayerManager =
      buildExecutionLayerManager(
        web3JEngineApiClient = validatorELNodeEngineApiWeb3JClient,
        elFork = qbftConsensusConfig.elFork,
        metricsFacade = metricsFacade,
      )
    val blockImportHandlers =
      Helpers.createBlockImportHandlers(
        elFork = qbftConsensusConfig.elFork,
        metricsFacade = metricsFacade,
        finalizationStateProvider = finalizationStateProvider,
        followerELNodeEngineApiWeb3JClients = followerELNodeEngineApiWeb3JClients,
      )
    val sealedBlockHandlers =
      mutableMapOf(
        "beacon block handlers" to SealedBeaconBlockHandlerAdapter(blockImportHandlers),
        "p2p broadcast sealed beacon block handler" to
          SealedBeaconBlockBroadcaster(p2pNetwork),
      )
    val sealedBlockHandlerMultiplexer = NewSealedBeaconBlockHandlerMultiplexer<Unit>(handlersMap = sealedBlockHandlers)

    val qbftValidatorFactory =
      QbftValidatorFactory(
        beaconChain = beaconChain,
        privateKeyBytes = privateKeyBytes,
        qbftOptions = qbftOptions,
        metricsSystem = metricsSystem,
        finalizationStateProvider = finalizationStateProvider,
        nextBlockTimestampProvider = nextTargetBlockTimestampProvider,
        newBlockHandler = sealedBlockHandlerMultiplexer,
        executionLayerManager = payloadValidatorExecutionLayerManager,
        clock = clock,
        p2PNetwork = p2pNetwork,
        allowEmptyBlocks = allowEmptyBlocks,
        forksSchedule = forksSchedule,
        payloadValidationEnabled = payloadValidationEnabled,
        onBlockTimerFired = onBlockTimerFired,
        onMessageReceived = onMessageReceived,
        onBlockMined = onBlockMined,
        syncStatusProvider = syncStatusProvider,
      )
    return qbftValidatorFactory.create(forkSpec)
  }
}
