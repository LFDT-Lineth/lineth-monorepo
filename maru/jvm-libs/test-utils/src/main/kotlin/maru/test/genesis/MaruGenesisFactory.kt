/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.test.genesis

import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ForksSchedule
import maru.consensus.QbftConsensusConfig
import maru.core.Validator
import kotlin.time.Clock
import kotlin.time.Instant

class MaruGenesisFactory(
  private val clock: Clock = Clock.System,
) {
  fun create(
    chainId: UInt,
    validators: List<ByteArray>,
    blockTimeSeconds: UInt = 1u,
    forks: Map<Instant, ChainFork> = emptyMap(),
    terminalTotalDifficulty: ULong? = null,
  ): ForksSchedule {
    require(blockTimeSeconds in 1u..60u) { "blockTimeSeconds must be between 1 and 60 seconds" }
    require(validators.isNotEmpty()) { "Validators must not be empty" }
    require(terminalTotalDifficulty == null || terminalTotalDifficulty > 0uL) {
      "terminalTotalDifficulty must be greater than 0 or null"
    }

    val validatorsSet = validators.map { Validator(it) }.toSet()
    require(validatorsSet.size == validators.size) { "Validators are duplicated" }

    if (forks.isEmpty()) {
      // Create a default fork schedule with a single fork at timestamp 0 for QBFT_PHASE0 / Prague
      val initialForkConfig =
        if (terminalTotalDifficulty != null) {
          DifficultyAwareQbftConfig(
            postTtdConfig = QbftConsensusConfig(
              validatorSet = validatorsSet,
              fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
            ),
            terminalTotalDifficulty = terminalTotalDifficulty,
          )
        } else {
          QbftConsensusConfig(
            validatorSet = validatorsSet,
            fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague),
          )
        }
      val initialForkSpec =
        ForkSpec(
          timestampSeconds = 0UL,
          blockTimeSeconds = blockTimeSeconds,
          configuration = initialForkConfig,
        )
      return ForksSchedule(chainId, listOf(initialForkSpec))
    }

    val forkSpecks =
      forks.entries
        .sortedBy { it.key }
        .also {
          require(it.first().key < this.clock.now()) {
            "The first fork timestamp must be in the past: found ${it.first().key}, now is ${this.clock.now()}"
          }
          val elForks =
            it.map { entry ->
              entry.value.elFork.version
                .toInt()
            }
          val sortedElForks = elForks.sorted()
          require(elForks == sortedElForks) {
            "EL forks don't follow the correct order: found ${it.map { entry -> entry.value.elFork }}, expected ${
              sortedElForks.map { version ->
                ElFork.values().first { it.version.toInt() == version }
              }
            }"
          }
        }.mapIndexed { index, (timestamp, chainFork) ->
          val forkConsensusConfig =
            if (index == 0 && terminalTotalDifficulty != null) {
              DifficultyAwareQbftConfig(
                postTtdConfig = QbftConsensusConfig(
                  validatorSet = validatorsSet,
                  fork = chainFork,
                ),
                terminalTotalDifficulty = terminalTotalDifficulty,
              )
            } else {
              QbftConsensusConfig(
                validatorSet = validatorsSet,
                fork = chainFork,
              )
            }
          ForkSpec(
            timestampSeconds = timestamp.epochSeconds.toULong(),
            blockTimeSeconds = blockTimeSeconds,
            configuration = forkConsensusConfig,
          )
        }

    return ForksSchedule(chainId, forkSpecks)
  }
}
