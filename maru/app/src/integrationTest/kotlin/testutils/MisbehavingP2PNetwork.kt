/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import linea.timer.TimerFactory
import maru.config.P2PConfig
import maru.core.SealedBeaconBlock
import maru.database.BeaconChain
import maru.database.P2PState
import maru.p2p.P2PNetworkImpl
import maru.p2p.RpcMethods
import maru.p2p.fork.ForkPeeringManager
import maru.p2p.messages.BeaconBlocksByRangeRequest
import maru.p2p.messages.BlockRetrievalStrategy
import maru.p2p.messages.StatusManager
import maru.serialization.SerDe
import net.consensys.linea.metrics.MetricsFacade
import kotlin.time.Duration
import org.hyperledger.besu.plugin.services.MetricsSystem as BesuMetricsSystem

class MisbehavingP2PNetwork(
  privateKeyBytes: ByteArray,
  p2pConfig: P2PConfig,
  chainId: UInt,
  serDe: SerDe<SealedBeaconBlock>,
  metricsFacade: MetricsFacade,
  metricsSystem: BesuMetricsSystem,
  statusManager: StatusManager,
  chain: BeaconChain,
  forkIdHashManager: ForkPeeringManager,
  isBlockImportEnabledProvider: () -> Boolean,
  p2pState: P2PState,
  blockRetrievalStrategy: BlockRetrievalStrategy,
  timerFactory: TimerFactory,
) {
  val p2pNetwork: P2PNetworkImpl =
    P2PNetworkImpl(
      privateKeyBytes = privateKeyBytes,
      p2pConfig = p2pConfig,
      chainId = chainId,
      serDe = serDe,
      metricsFacade = metricsFacade,
      metricsSystem = metricsSystem,
      statusManager = statusManager,
      beaconChain = chain,
      forkIdHashManager = forkIdHashManager,
      isBlockImportEnabledProvider = isBlockImportEnabledProvider,
      p2PState = p2pState,
      rpcMethodsFactory = { statusMessageFactory, lineaRpcProtocolIdGenerator, peerLookup, beaconChain ->
        RpcMethods(statusMessageFactory, lineaRpcProtocolIdGenerator, peerLookup, beaconChain, blockRetrievalStrategy)
      },
      timerFactory = timerFactory,
    )
}

class FourEmptyResponsesStrategy : BlockRetrievalStrategy {
  var numCalls = 0

  override fun getBlocks(
    beaconChain: BeaconChain,
    request: BeaconBlocksByRangeRequest,
    maxBlocks: ULong,
  ): List<SealedBeaconBlock> {
    if (numCalls < 4) {
      numCalls++
      return emptyList()
    } else {
      return beaconChain.getSealedBeaconBlocks(
        startBlockNumber = request.startBlockNumber,
        count = maxBlocks,
      )
    }
  }
}

class TimeOutResponsesStrategy(
  var delay: Duration,
) : BlockRetrievalStrategy {
  override fun getBlocks(
    beaconChain: BeaconChain,
    request: BeaconBlocksByRangeRequest,
    maxBlocks: ULong,
  ): List<SealedBeaconBlock> {
    Thread.sleep(delay.inWholeMilliseconds)
    return beaconChain.getSealedBeaconBlocks(
      startBlockNumber = request.startBlockNumber,
      count = maxBlocks,
    )
  }
}
