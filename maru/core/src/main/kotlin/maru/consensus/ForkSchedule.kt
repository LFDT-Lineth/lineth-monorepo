/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import org.apache.logging.log4j.LogManager
import java.util.NavigableSet
import java.util.TreeSet
import kotlin.reflect.KClass

enum class ClFork(
  val version: Byte,
) {
  QBFT_PHASE0(0x0), // current QBFT
  QBFT_PHASE1(0x1), // ony used for testing forkId Digester for now
}

enum class ElFork(
  val version: Byte,
) {
  // London(0x0),
  Paris(0x1),
  Shanghai(0x2),
  Cancun(0x3),
  Prague(0x4),
  Osaka(0x5),
}

data class ChainFork(
  val clFork: ClFork,
  val elFork: ElFork,
)

data class ForkSpec(
  val timestampSeconds: ULong,
  val blockTimeSeconds: UInt,
  val configuration: ConsensusConfig,
) {
  init {
    require(blockTimeSeconds > 0UL) { "blockTimeSeconds must be greater or equal to 1 second" }
  }

  override fun toString(): String =
    "ForkSpec(timestampSeconds=$timestampSeconds, blockTimeSeconds=$blockTimeSeconds, configuration=$configuration)"
}

class ForksSchedule(
  val chainId: UInt,
  forks: Collection<ForkSpec>,
) {
  private val log = LogManager.getLogger(this.javaClass)

  val forks: NavigableSet<ForkSpec> =
    run {
      val newForks =
        TreeSet(
          Comparator
            .comparing(ForkSpec::timestampSeconds)
            .reversed(),
        )
      newForks.addAll(forks)
      require(newForks.size == forks.size) { "Fork timestamps must be unique" }
      log.debug("Forks: {}", newForks)
      newForks
    }

  fun getForkByTimestamp(timestamp: ULong): ForkSpec =
    forks.firstOrNull { timestamp >= it.timestampSeconds } ?: throw IllegalArgumentException(
      "No fork found for $timestamp, first known fork is at ${forks.last.timestampSeconds}",
    )

  fun getNextForkByTimestamp(timestamp: ULong): ForkSpec? =
    forks
      .reversed()
      .firstOrNull { timestamp < it.timestampSeconds }

  fun getPreviousForkByTimestamp(timestamp: ULong): ForkSpec? {
    val previousForks =
      forks
        .filter { timestamp >= it.timestampSeconds }
        .take(2)
    return previousForks.getOrNull(1)
  }

  fun <T : ConsensusConfig> getForkByConfigType(configClass: KClass<T>): ForkSpec {
    // Uses findLast since the list is reversed to get the first matching fork
    val fork = forks.findLast { it.configuration::class == configClass }
    return fork ?: throw IllegalArgumentException(
      "No fork found for config type ${configClass.simpleName}",
    )
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ForksSchedule

    if (chainId != other.chainId) return false
    if (forks.toSet() != other.forks.toSet()) return false

    return true
  }

  override fun hashCode(): Int {
    var result = chainId.hashCode()
    result = 31 * result + forks.hashCode()
    return result
  }

  override fun toString(): String = "ForksSchedule(chainId=$chainId, forks=$forks)"
}
