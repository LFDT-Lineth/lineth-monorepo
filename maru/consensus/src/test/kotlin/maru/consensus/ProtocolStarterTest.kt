/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import linea.timer.TimerFactory
import maru.core.Protocol
import maru.subscription.InOrderFanoutSubscriptionManager
import maru.subscription.SubscriptionNotifier
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import testutils.maru.TestablePeriodicTimerFactory
import java.time.Clock
import java.time.Instant
import java.time.ZoneId
import java.time.ZoneOffset
import kotlin.time.Duration.Companion.seconds

class ProtocolStarterTest {
  private class StubProtocol : Protocol {
    var started = false
    var closed = false

    override fun start() {
      require(!closed) { "The protocol can't be started after a close!" }
      started = true
    }

    override fun pause() {
      started = false
    }

    override fun close() {
      pause()
      closed = true
    }
  }

  private val chainId = 1337u

  private lateinit var protocol1: StubProtocol
  private lateinit var protocol2: StubProtocol

  private val protocolConfig1 =
    object : ConsensusConfig {
      override val fork: ChainFork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Shanghai)
    }
  private val protocolConfig2 =
    object : ConsensusConfig {
      override val fork: ChainFork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague)
    }
  private val forkSpec1 = ForkSpec(0UL, 5u, protocolConfig1)
  private val forkSpec2 = ForkSpec(15UL, 2u, protocolConfig2)

  val forkTransitions = mutableListOf<ForkSpec>()
  val forkTransitionNotifier =
    InOrderFanoutSubscriptionManager<ForkSpec>().also {
      it.addSyncSubscriber(forkTransitions::add)
    }

  @BeforeEach
  fun initializeStubProtocols() {
    protocol1 = StubProtocol()
    protocol2 = StubProtocol()
  }

  @Test
  fun `ProtocolStarter kickstarts the protocol based on current time`() {
    val forksSchedule =
      ForksSchedule(
        chainId,
        listOf(
          forkSpec1,
          forkSpec2,
        ),
      )
    val protocolStarter =
      createProtocolStarter(
        forksSchedule = forksSchedule,
        clockMilliseconds = 16000, // After fork transition at 15
        forkTransitionNotifier = forkTransitionNotifier,
      )
    protocolStarter.start()
    assertActiveProtocol2(protocolStarter)
    assertThat(forkTransitions.size).isEqualTo(1)
    assertThat(forkTransitions.first()).isEqualTo(forkSpec2)
  }

  @Test
  fun `ProtocolStarter uses first fork when current time is before any fork transition`() {
    val forksSchedule =
      ForksSchedule(
        chainId,
        listOf(
          forkSpec1,
          forkSpec2,
        ),
      )
    val protocolStarter =
      createProtocolStarter(
        forksSchedule = forksSchedule,
        clockMilliseconds = 5000, // Before fork transition at 15
        forkTransitionNotifier = forkTransitionNotifier,
      )
    protocolStarter.start()
    assertActiveProtocol1(protocolStarter)
    assertThat(forkTransitions.size).isEqualTo(1)
    assertThat(forkTransitions.first()).isEqualTo(forkSpec1)
  }

  @Test
  fun `ProtocolStarter switches if the next block timestamp past the next fork`() {
    val forksSchedule =
      ForksSchedule(
        chainId,
        listOf(
          forkSpec1,
          forkSpec2,
        ),
      )
    val protocolStarter =
      createProtocolStarter(
        forksSchedule = forksSchedule,
        clockMilliseconds = 10000, // 5 seconds before fork transition at 15
        forkTransitionNotifier = forkTransitionNotifier,
      )
    protocolStarter.start()

    assertActiveProtocol2(protocolStarter)
    assertThat(forkTransitions.size).isEqualTo(1)
    assertThat(forkTransitions.first()).isEqualTo(forkSpec2)
  }

  @Test
  fun `ProtocolStarter doesn't switch if it's too early`() {
    val forksSchedule =
      ForksSchedule(
        chainId,
        listOf(
          forkSpec1,
          forkSpec2,
        ),
      )
    val protocolStarter =
      createProtocolStarter(
        forksSchedule = forksSchedule,
        clockMilliseconds = 5000, // Before fork transition at 15, next block still in forkSpec1 period
        forkTransitionNotifier = forkTransitionNotifier,
      )
    protocolStarter.start()

    assertActiveProtocol1(protocolStarter)
    assertThat(forkTransitions.size).isEqualTo(1)
    assertThat(forkTransitions.first()).isEqualTo(forkSpec1)
  }

  @Test
  fun `ProtocolStarter switches protocols during periodic polling`() {
    val forksSchedule =
      ForksSchedule(
        chainId,
        listOf(forkSpec1, forkSpec2),
      )

    var currentTimeMillis = 8000L // Next block at ~13 seconds (before fork at 15)
    val timerFactory = TestablePeriodicTimerFactory()

    val protocolStarter =
      createProtocolStarter(
        forksSchedule = forksSchedule,
        clockMilliseconds = currentTimeMillis,
        timerFactory = timerFactory,
        forkTransitionNotifier = forkTransitionNotifier,
      ) { currentTimeMillis }

    protocolStarter.start()
    val timer = timerFactory.getTimer("ProtocolStarterPoller")!!

    // Initially should be on first protocol since next block is before fork transition
    assertActiveProtocol1(protocolStarter)

    currentTimeMillis = 12000L // Next block at ~17 seconds (after fork at 15)
    timer.runNextTask()

    // Should switch to second protocol since next block needs forkSpec2
    assertActiveProtocol2(protocolStarter)

    assertThat(forkTransitions.size).isEqualTo(2)
    assertThat(forkTransitions.first()).isEqualTo(forkSpec1)
    assertThat(forkTransitions.last()).isEqualTo(forkSpec2)
  }

  @Test
  fun `ProtocolStarter does not switch if next block still uses same protocol`() {
    val forksSchedule =
      ForksSchedule(
        chainId,
        listOf(forkSpec1, forkSpec2),
      )

    var currentTimeMillis = 8000L // Next block at ~13 seconds (before fork at 15)
    val timerFactory = TestablePeriodicTimerFactory()
    val protocolStarter =
      createProtocolStarter(
        forksSchedule = forksSchedule,
        clockMilliseconds = currentTimeMillis,
        timerFactory = timerFactory,
        forkTransitionNotifier = forkTransitionNotifier,
      ) { currentTimeMillis }

    protocolStarter.start()
    val timer = timerFactory.getTimer("ProtocolStarterPoller")!!
    assertActiveProtocol1(protocolStarter)

    currentTimeMillis = 9000L // Next block at ~14 seconds (still before fork at 15)
    timer.runNextTask()

    assertActiveProtocol1(protocolStarter)
    assertThat(forkTransitions.size).isEqualTo(1)
    assertThat(forkTransitions.first()).isEqualTo(forkSpec1)
  }

  @Test
  fun `ProtocolStarter switches when next block will be produced by new protocol at startup`() {
    val forksSchedule =
      ForksSchedule(
        chainId,
        listOf(forkSpec1, forkSpec2),
      )

    var currentTimeMillis = 11000L // With 5-second block time, next block at ~16 seconds (after fork at 15)
    val timerFactory = TestablePeriodicTimerFactory()
    val protocolStarter =
      createProtocolStarter(
        forksSchedule = forksSchedule,
        clockMilliseconds = currentTimeMillis,
        timerFactory = timerFactory,
        forkTransitionNotifier = forkTransitionNotifier,
      ) { currentTimeMillis }

    protocolStarter.start()
    val timer = timerFactory.getTimer("ProtocolStarterPoller")!!

    assertActiveProtocol2(protocolStarter)

    currentTimeMillis = 15000L // Exactly at fork transition
    timer.runNextTask()

    assertActiveProtocol2(protocolStarter)
    assertThat(forkTransitions.size).isEqualTo(1)
    assertThat(forkTransitions.first()).isEqualTo(forkSpec2)
  }

  @Test
  fun `ProtocolStarter close method closes active protocol and stops timer`() {
    val forksSchedule =
      ForksSchedule(
        chainId,
        listOf(forkSpec1, forkSpec2),
      )
    val protocolStarter =
      createProtocolStarter(
        forksSchedule = forksSchedule,
        clockMilliseconds = 5000, // Before fork transition, should start with protocol1
        forkTransitionNotifier = forkTransitionNotifier,
      )

    // Start the protocol starter to activate a protocol
    protocolStarter.start()
    assertActiveProtocol1(protocolStarter)

    // Close the protocol starter
    protocolStarter.close()

    // Verify the active protocol is closed
    assertThat(protocol1.closed).isTrue()
    assertThat(protocol1.started).isFalse() // pause() should have been called in close()
  }

  private val protocolFactory =
    object : ProtocolFactory {
      override fun create(forkSpec: ForkSpec): Protocol =
        when (forkSpec.configuration) {
          protocolConfig1 -> protocol1
          protocolConfig2 -> protocol2
          else -> error("invalid protocol config")
        }
    }

  private fun createProtocolStarter(
    forksSchedule: ForksSchedule,
    clockMilliseconds: Long,
    timerFactory: TimerFactory = TestablePeriodicTimerFactory(),
    forkTransitionNotifier: SubscriptionNotifier<ForkSpec>,
    timeProvider: (() -> Long)? = null,
  ): ProtocolStarter {
    val clock =
      if (timeProvider != null) {
        object : Clock() {
          override fun getZone() = ZoneOffset.UTC

          override fun instant() = Instant.ofEpochMilli(timeProvider())

          override fun withZone(zone: ZoneId?) = this
        }
      } else {
        Clock.fixed(Instant.ofEpochMilli(clockMilliseconds), ZoneOffset.UTC)
      }

    return ProtocolStarter(
      forksSchedule = forksSchedule,
      protocolFactory = protocolFactory,
      nextBlockTimestampProvider = NextBlockTimestampProviderImpl(
        clock = clock,
        forksSchedule = forksSchedule,
      ),
      forkTransitionCheckInterval = 1.seconds,
      clock = clock,
      timerFactory = timerFactory,
      forkTransitionNotifier = forkTransitionNotifier,
    )
  }

  private fun assertActiveProtocol1(protocolStarter: ProtocolStarter) {
    val currentProtocol = protocolStarter.currentProtocolWithForkReference.get()
    assertThat(currentProtocol.fork).isEqualTo(forkSpec1)
    assertThat(protocol1.started).isTrue()
    assertThat(protocol2.started).isFalse()
  }

  private fun assertActiveProtocol2(protocolStarter: ProtocolStarter) {
    val currentProtocol = protocolStarter.currentProtocolWithForkReference.get()
    assertThat(currentProtocol.fork).isEqualTo(forkSpec2)
    assertThat(protocol2.started).isTrue()
    assertThat(protocol1.started).isFalse()
  }
}
