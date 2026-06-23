/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.fork

import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import maru.core.ext.DataGenerators
import maru.database.BeaconChain
import maru.database.InMemoryBeaconChain
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import java.time.Clock
import java.time.Instant
import java.time.ZoneId
import java.time.ZoneOffset
import kotlin.random.Random
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.ExperimentalTime
import kotlin.time.toJavaInstant

class LenientForkPeeringManagerTest {
  private lateinit var clock: MutableClock
  private lateinit var beaconChain: BeaconChain
  private val validator = Validator(ByteArray(20) { 0x02 })
  private lateinit var forks: Map<ElFork, ForkInfo>

  // Mutable clock for testing
  private class MutableClock(
    var instant: Instant,
  ) : Clock() {
    override fun getZone() = ZoneOffset.UTC

    override fun withZone(zone: ZoneId?) = this

    override fun instant() = instant

    fun setEpochSeconds(epochSeconds: Long) {
      instant = Instant.ofEpochSecond(epochSeconds)
    }

    @OptIn(ExperimentalTime::class)
    fun setTime(time: kotlin.time.Instant) {
      instant = time.toJavaInstant()
    }
  }

  @BeforeEach
  fun setup() {
    beaconChain = inMemoryBeaconChainFromGenesis(genesisTimestampSeconds = 100u)
    forks = listOf(
      forkSpec(0UL, ElFork.Paris),
      forkSpec(1_000UL, ElFork.Shanghai),
      forkSpec(2_000UL, ElFork.Cancun),
      forkSpec(3_000UL, ElFork.Prague),
    ).mapIndexed { index, forkSpec ->
      ForkInfo(forkSpec, forkIdDigest = ByteArray(1) { index.toByte() })
    }.associateBy { (it.forkSpec.configuration as QbftConsensusConfig).elFork }
    clock = MutableClock(Instant.ofEpochSecond((forks[ElFork.Paris]!!.forkSpec.timestampSeconds + 100UL).toLong()))
  }

  private fun forkSpec(
    ts: ULong,
    elFork: ElFork,
  ): ForkSpec =
    ForkSpec(
      timestampSeconds = ts,
      blockTimeSeconds = 1U,
      configuration = QbftConsensusConfig(setOf(validator), fork = ChainFork(ClFork.QBFT_PHASE0, elFork)),
    )

  private fun forkIdManager(
    forks: List<ForkInfo> = this.forks.values.toList(),
    peeringForkMismatchLeewayTime: Duration = 5.seconds,
  ): LenientForkPeeringManager =
    LenientForkPeeringManager(
      forks = forks,
      peeringForkMismatchLeewayTime = peeringForkMismatchLeewayTime,
      clock = clock,
    )

  private fun inMemoryBeaconChainFromGenesis(
    genesisTimestampSeconds: ULong = 0UL,
  ): InMemoryBeaconChain {
    val (beaconState, sealedBlock) = DataGenerators.genesisState(
      genesisTimestamp = genesisTimestampSeconds,
    )
    return InMemoryBeaconChain(beaconState, sealedBlock)
  }

  @Test
  fun `currentFork should return correct fork`() {
    val manager = forkIdManager()
    clock.setEpochSeconds(100)
    assertThat(manager.currentFork()).isEqualTo(forks[ElFork.Paris])

    clock.setEpochSeconds(forks[ElFork.Cancun]!!.forkSpec.timestampSeconds.toLong())
    assertThat(manager.currentFork()).isEqualTo(forks[ElFork.Cancun])
    assertThat(manager.prevFork()).isEqualTo(forks[ElFork.Shanghai])
    assertThat(manager.nextFork()).isEqualTo(forks[ElFork.Prague])

    clock.setEpochSeconds(forks[ElFork.Cancun]!!.forkSpec.timestampSeconds.toLong() + 1)
    assertThat(manager.currentFork()).isEqualTo(forks[ElFork.Cancun])

    clock.setEpochSeconds(forks[ElFork.Prague]!!.forkSpec.timestampSeconds.toLong() - 1)
    assertThat(manager.currentFork()).isEqualTo(forks[ElFork.Cancun])

    clock.setEpochSeconds(forks[ElFork.Prague]!!.forkSpec.timestampSeconds.toLong())
    assertThat(manager.currentFork()).isEqualTo(forks[ElFork.Prague])

    clock.setEpochSeconds(forks[ElFork.Prague]!!.forkSpec.timestampSeconds.toLong() + 1)
    assertThat(manager.nextFork()).isNull()
  }

  @Test
  fun `isValidForkIdForPeering should return false when fork is not valid`() {
    val manager = forkIdManager()
    clock.setEpochSeconds(forks[ElFork.Paris]!!.forkSpec.timestampSeconds.toLong() + 1)
    assertThat(manager.isValidForPeering(Random.Default.nextBytes(4))).isFalse()
  }

  @Test
  fun `isValidForkIdForPeering should return true when fork is same as current fork`() {
    val manager = forkIdManager()
    clock.setEpochSeconds(forks[ElFork.Paris]!!.forkSpec.timestampSeconds.toLong() + 1)
    val currentFork = forks[ElFork.Paris]!!
    assertThat(manager.isValidForPeering(currentFork.forkIdDigest)).isTrue()
  }

  @Test
  fun `isValidForkIdForPeering should return true when fork is previous and within leeway`() {
    val manager =
      forkIdManager(
        peeringForkMismatchLeewayTime = 10.seconds,
      )
    clock.setEpochSeconds(forks[ElFork.Cancun]!!.forkSpec.timestampSeconds.toLong() + 9)
    assertThat(manager.currentFork()).isEqualTo(forks[ElFork.Cancun]) // sanity check
    val previousFork = forks[ElFork.Shanghai]!!
    assertThat(manager.isValidForPeering(previousFork.forkIdDigest)).isTrue()
  }

  @Test
  fun `isValidForkIdForPeering should return false when fork is previous outside leeway`() {
    val manager =
      forkIdManager(
        peeringForkMismatchLeewayTime = 10.seconds,
      )
    clock.setEpochSeconds(forks[ElFork.Cancun]!!.forkSpec.timestampSeconds.toLong() + 11)
    assertThat(manager.currentFork()).isEqualTo(forks[ElFork.Cancun]) // sanity check
    val previousFork = forks[ElFork.Shanghai]!!
    assertThat(manager.isValidForPeering(previousFork.forkIdDigest)).isFalse()
  }

  @Test
  fun `isValidForkIdForPeering should return true when fork is next within leeway`() {
    val manager =
      forkIdManager(
        peeringForkMismatchLeewayTime = 10.seconds,
      )
    clock.setEpochSeconds(forks[ElFork.Prague]!!.forkSpec.timestampSeconds.toLong() - 9)
    assertThat(manager.currentFork()).isEqualTo(forks[ElFork.Cancun]) // sanity check
    val nextFork = forks[ElFork.Prague]!!

    assertThat(manager.isValidForPeering(nextFork.forkIdDigest)).isTrue()
  }

  @Test
  fun `isValidForkIdForPeering should return false when fork is next outside leeway`() {
    val manager =
      forkIdManager(
        peeringForkMismatchLeewayTime = 10.seconds,
      )
    clock.setEpochSeconds(forks[ElFork.Shanghai]!!.forkSpec.timestampSeconds.toLong() - 11)
    assertThat(manager.currentFork()).isEqualTo(forks[ElFork.Paris]) // sanity check
    val nextFork = forks[ElFork.Cancun]!!
    assertThat(manager.isValidForPeering(nextFork.forkIdDigest)).isFalse()

    val nextNextFork = forks[ElFork.Prague]!!
    assertThat(manager.isValidForPeering(nextNextFork.forkIdDigest)).isFalse()
  }
}
