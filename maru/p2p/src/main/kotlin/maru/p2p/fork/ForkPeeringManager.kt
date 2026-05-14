/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.fork

import linea.kotlin.encodeHex
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ForkSpec
import maru.database.BeaconChain
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.time.Clock
import kotlin.time.Duration
import kotlin.time.ExperimentalTime
import kotlin.time.Instant
import kotlin.time.toKotlinInstant

data class ForkInfo(
  val forkSpec: ForkSpec,
  val forkIdDigest: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ForkInfo

    if (forkSpec != other.forkSpec) return false
    if (!forkIdDigest.contentEquals(other.forkIdDigest)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = forkSpec.hashCode()
    result = 31 * result + forkIdDigest.contentHashCode()
    return result
  }

  override fun toString(): String = "ForkInfo(forkSpec=$forkSpec, forkIdDigest=${forkIdDigest.encodeHex()})"

  /**
   * {ts=1653855232 time=2025-10-09T11:53:52Z ttd=3200000000 fork=QBFT_Phase0/Paris forkId=0xasdsd},
   * {ts=1653855232 time=2025-10-09T11:53:52Z fork=QBFT_Phase0/Shanghai forkDigest=0xasdsd}
   */
  @OptIn(ExperimentalTime::class)
  fun toLogString(): String =
    StringBuilder()
      .append("{")
      .append("ts=")
      .append(forkSpec.timestampSeconds.toLong())
      .append(" time=")
      .append(Instant.fromEpochSeconds(forkSpec.timestampSeconds.toLong()))
      .apply {
        if (forkSpec.configuration is DifficultyAwareQbftConfig) {
          append(" ttd=")
          append((forkSpec.configuration as DifficultyAwareQbftConfig).terminalTotalDifficulty)
        }
      }.append(" fork=")
      .append(forkSpec.configuration.fork.clFork)
      .append("/")
      .append(forkSpec.configuration.fork.elFork)
      .append(" forkDigest=")
      .append(forkIdDigest.encodeHex())
      .append("}")
      .toString()
}

interface ForkPeeringManager {
  /**
   *  Returns the current fork hash, based on current time.
   */
  fun currentForkHash(): ByteArray

  fun isValidForPeering(otherForkIdHash: ByteArray): Boolean
}

class LenientForkPeeringManager internal constructor(
  private val clock: Clock,
  private val forks: List<ForkInfo>,
  private val peeringForkMismatchLeewayTime: Duration,
  private val log: Logger = LogManager.getLogger(LenientForkPeeringManager::class.java),
) : ForkPeeringManager {
  init {
    require(forks.isNotEmpty()) { "empty forks list" }
    logForksInfo(forks, log)
  }

  @OptIn(ExperimentalTime::class)
  private fun ForkInfo.isWithinLeeway(): Boolean {
    val forkTime = Instant.fromEpochSeconds(forkSpec.timestampSeconds.toLong())
    val currentTime = clock.instant().toKotlinInstant()
    val currentTimeMinusLeeway = currentTime.minus(peeringForkMismatchLeewayTime)
    val currentTimePlusLeeway = currentTime.plus(peeringForkMismatchLeewayTime)
    return forkTime in currentTimeMinusLeeway..currentTimePlusLeeway
  }

  /**
   *  List of [ForkInfo] sorted by their [ForkSpec.timestampSeconds] descending order (newest first).
   */
  internal val forksInfo: List<ForkInfo> = forks.sortedBy { it.forkSpec.timestampSeconds }.reversed()

  private fun currentForkIndex(): Int {
    val currentTimestamp = clock.instant().epochSecond.toULong()
    return forksInfo
      .indexOfFirst { currentTimestamp >= it.forkSpec.timestampSeconds }
      .let {
        // if no matching fork found, it means 1st fork is in the future so
        // we default the first fork that is on last index
        if (it < 0) forksInfo.size - 1 else it
      }
  }

  internal fun currentFork(): ForkInfo = forksInfo[currentForkIndex()]

  internal fun nextFork(): ForkInfo? = forksInfo.getOrNull(index = currentForkIndex() - 1)

  internal fun prevFork(): ForkInfo? = forksInfo.getOrNull(index = currentForkIndex() + 1)

  override fun currentForkHash(): ByteArray = currentFork().forkIdDigest

  override fun isValidForPeering(otherForkIdHash: ByteArray): Boolean {
    val currentFork = currentFork()
    log.debug(
      "validating peer connection: currentFork={} peerForkId={}",
      { currentFork.toLogString() },
      { otherForkIdHash.encodeHex() },
    )

    if (currentFork.forkIdDigest.contentEquals(otherForkIdHash)) {
      // most probable case
      return true
    }
    val otherPeerFork = forksInfo.firstOrNull { it.forkIdDigest.contentEquals(otherForkIdHash) }
    if (otherPeerFork == null) {
      // it means peer fork does not match any of our forks,
      // possible cases:
      // A - peer has outdated genesis file, missing latest fork configs
      // B - peer has a different genesis file, a real network fork
      log.info(
        "invalid peer fork: reason=FORK_UNKNOWN currentFork={} peerForkId={}",
        currentFork.toLogString(),
        otherForkIdHash.encodeHex(),
      )
      return false
    }
    // to handle cases around network switching to the next fork
    // also fork may mismatch due to network latency, clock out of sync
    if (otherPeerFork == nextFork() && otherPeerFork.isWithinLeeway()) {
      // A - peer has already switched to the next fork
      return true
    }
    if (otherPeerFork == prevFork() && currentFork().isWithinLeeway()) {
      // B - we already switched to the next fork, peer still on the previous fork
      // but allow to connect because we just switched recently and is within leeway
      return true
    }

    log.info(
      "invalid peer fork: reason=FORK_MISMATCH currentFork={} peerFork={} currentTime={} leeway={}",
      currentFork.toLogString(),
      otherPeerFork.toLogString(),
      clock.instant(),
      peeringForkMismatchLeewayTime,
    )
    return false
  }

  companion object {
    fun create(
      chainId: UInt,
      beaconChain: BeaconChain,
      forks: List<ForkSpec>,
      peeringForkMismatchLeewayTime: Duration,
      clock: Clock,
    ): LenientForkPeeringManager {
      val digestsCalculator = RollingForwardForkIdDigestCalculator(chainId, beaconChain)
      return LenientForkPeeringManager(
        clock = clock,
        peeringForkMismatchLeewayTime = peeringForkMismatchLeewayTime,
        forks = digestsCalculator.calculateForkDigests(forks),
      )
    }

    @OptIn(ExperimentalTime::class)
    fun logForksInfo(
      forks: List<ForkInfo>,
      log: Logger = LogManager.getLogger(LenientForkPeeringManager::class.java),
    ) {
      val ascendingForks =
        forks
          .sortedBy { it.forkSpec.timestampSeconds }
          .reversed()
      log.info(
        "forks: ${ascendingForks.joinToString(",", "[", "]", transform = ForkInfo::toLogString)}",
      )
    }
  }
}
