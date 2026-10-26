/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.cluster

import linea.timer.TimerFactory
import maru.app.MaruApp
import maru.app.MaruAppFactory
import maru.config.MaruConfig
import maru.config.P2PConfig
import maru.consensus.ForksSchedule
import maru.crypto.PrivateKeyGenerator
import maru.database.BeaconChain
import maru.database.P2PState
import maru.p2p.P2PNetwork
import maru.p2p.P2PNetworkImpl
import maru.p2p.fork.ForkPeeringManager
import maru.p2p.messages.StatusManager
import maru.serialization.rlp.ForkAwareBlockHashing
import net.consensys.linea.metrics.MetricsFacade
import org.hyperledger.besu.plugin.services.MetricsSystem as BesuMetricsSystem

typealias P2pNetworkFactory = (
  ByteArray,
  P2PConfig,
  UInt,
  ForkAwareBlockHashing,
  MetricsFacade,
  BesuMetricsSystem,
  StatusManager,
  BeaconChain,
  ForkPeeringManager,
  () -> Boolean,
  P2PState,
  TimerFactory,
) -> P2PNetworkImpl

fun createMaru(
  elNode: ElNode? = null,
  config: MaruConfig,
  bootnodes: List<String> = emptyList(),
  staticpeers: List<String> = emptyList(),
  nodeKeyData: PrivateKeyGenerator.KeyData,
  nodeRole: NodeRole,
  forkSchedule: ForksSchedule,
  overridingP2PNetwork: P2PNetwork?,
  p2pNetworkFactory: P2pNetworkFactory,
  payloadValidationEnabled: Boolean,
  maruConfigurator: (MaruConfig) -> MaruConfig,
): MaruApp {
  var effectiveConfig = config
  effectiveConfig = setValidatorConfig(
    config = effectiveConfig,
    payloadValidationEnabled = payloadValidationEnabled,
    elNode = elNode,
  )
  effectiveConfig = maruConfigurator(effectiveConfig)
  initPersistence(effectiveConfig.persistence, nodeKeyData)
  effectiveConfig = setQbftConfigIfSequencer(
    config = effectiveConfig,
    isSequencer = nodeRole == NodeRole.Sequencer,
    nodeKeyData = nodeKeyData,
  )
  effectiveConfig = setP2pConfig(config = effectiveConfig, bootnodes = bootnodes, staticpeers = staticpeers)

  return MaruAppFactory().create(
    config = effectiveConfig,
    beaconGenesisConfig = forkSchedule,
    overridingP2PNetwork = overridingP2PNetwork,
    p2pNetworkFactory = p2pNetworkFactory,
  )
}
