/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p

import maru.config.P2PConfig
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.metrics.noop.NoOpMetricsSystem
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.time.SystemTimeProvider
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import tech.pegasys.teku.networking.p2p.network.PeerAddress
import tech.pegasys.teku.networking.p2p.peer.DisconnectReason
import tech.pegasys.teku.networking.p2p.peer.NodeId
import tech.pegasys.teku.networking.p2p.reputation.ReputationAdjustment
import java.util.Optional

class MaruReputationManagerTest {
  private val nodeId = mock<NodeId>()
  private val peerAddress =
    mock<PeerAddress>().apply {
      whenever(id).thenReturn(nodeId)
    }
  private val metricsSystem = NoOpMetricsSystem()
  private val timeProvider = mock<SystemTimeProvider>()
  private val reputationConfig = P2PConfig.Reputation()

  @Test
  fun `allows connection by default`() {
    val manager = MaruReputationManager(metricsSystem, timeProvider, { false }, reputationConfig)
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isTrue()
  }

  @Test
  fun `disallows connection after large penalty adjustment`() {
    val manager = MaruReputationManager(metricsSystem, timeProvider, { false }, reputationConfig)
    whenever(timeProvider.timeInMillis).thenReturn(UInt64.ZERO)
    manager.adjustReputation(peerAddress, ReputationAdjustment.LARGE_PENALTY)
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isFalse()
  }

  @Test
  fun `doesn't allow connection after cooldown period expires but allows after ban period`() {
    val manager =
      MaruReputationManager(
        metricsSystem = metricsSystem,
        timeProvider = timeProvider,
        isStaticPeer = { false },
        reputationConfig = reputationConfig,
      )
    whenever(timeProvider.timeInMillis).thenReturn(UInt64.ZERO)
    manager.adjustReputation(peerAddress, ReputationAdjustment.LARGE_PENALTY)
    // Simulate time passing beyond cooldown
    whenever(timeProvider.timeInMillis).thenReturn(
      UInt64.ZERO.plus(reputationConfig.cooldownPeriod.inWholeMilliseconds) + 1,
    )
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isFalse()
    // Simulate time passing beyond ban period
    whenever(timeProvider.timeInMillis).thenReturn(
      UInt64.ZERO.plus(reputationConfig.banPeriod.inWholeMilliseconds) + 1,
    )
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isTrue()
  }

  @Test
  fun `does not adjust reputation for static peer`() {
    val manager = MaruReputationManager(metricsSystem, timeProvider, { true }, reputationConfig)
    val result = manager.adjustReputation(peerAddress, ReputationAdjustment.LARGE_PENALTY)
    assertThat(result).isFalse()
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isTrue()
  }

  @Test
  fun `reports disconnection with permanent reason bans peer`() {
    val manager = MaruReputationManager(metricsSystem, timeProvider, { false }, reputationConfig)
    whenever(timeProvider.timeInMillis).thenReturn(UInt64.ZERO)
    manager.reportDisconnection(peerAddress, Optional.of(DisconnectReason.REMOTE_FAULT), true)
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isFalse()
  }

  @Test
  fun `reports successful connection resets suitability`() {
    val manager = MaruReputationManager(metricsSystem, timeProvider, { false }, reputationConfig)
    whenever(timeProvider.timeInMillis).thenReturn(UInt64.ZERO)
    manager.adjustReputation(peerAddress, ReputationAdjustment.LARGE_PENALTY)
    manager.reportInitiatedConnectionSuccessful(peerAddress)
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isTrue()
  }

  @Test
  fun `re-bans peer on subsequent disconnection after initial ban expires`() {
    val manager = MaruReputationManager(metricsSystem, timeProvider, { false }, reputationConfig)

    // t0: first disconnection with a ban-worthy reason triggers initial ban
    whenever(timeProvider.timeInMillis).thenReturn(UInt64.ZERO)
    manager.reportDisconnection(peerAddress, Optional.of(DisconnectReason.REMOTE_FAULT), true)
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isFalse()

    // t1: move time to just after initial ban expires; peer becomes eligible again
    val t1 = UInt64.ZERO.plus(reputationConfig.banPeriod.inWholeMilliseconds) + 1
    whenever(timeProvider.timeInMillis).thenReturn(t1)
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isTrue()

    // t1: immediately another disconnection with same reason should re-apply the ban
    manager.reportDisconnection(peerAddress, Optional.of(DisconnectReason.REMOTE_FAULT), true)
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isFalse()

    // t2: after second ban expires, peer should be allowed again
    val t2 = t1.plus(reputationConfig.banPeriod.inWholeMilliseconds) + 1
    whenever(timeProvider.timeInMillis).thenReturn(t2)
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isTrue()
  }

  @Test
  fun `does not extend ban when disconnection occurs during active ban`() {
    val manager = MaruReputationManager(metricsSystem, timeProvider, { false }, reputationConfig)

    // t0: ban the peer
    whenever(timeProvider.timeInMillis).thenReturn(UInt64.ZERO)
    manager.reportDisconnection(peerAddress, Optional.of(DisconnectReason.REMOTE_FAULT), true)
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isFalse()

    // t_mid: still within the first ban window
    val half = reputationConfig.banPeriod.inWholeMilliseconds / 2
    val tMid = UInt64.ZERO.plus(half + 1)
    whenever(timeProvider.timeInMillis).thenReturn(tMid)

    // Report another disconnection during active ban; should NOT extend the ban
    manager.reportDisconnection(peerAddress, Optional.of(DisconnectReason.REMOTE_FAULT), true)
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isFalse()

    // t_end: just after the original ban expires; should be allowed
    val tEnd = UInt64.ZERO.plus(reputationConfig.banPeriod.inWholeMilliseconds) + 1
    whenever(timeProvider.timeInMillis).thenReturn(tEnd)
    assertThat(manager.isConnectionInitiationAllowed(peerAddress)).isTrue()
  }
}
