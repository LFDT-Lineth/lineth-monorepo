/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import maru.core.Validator

data class QbftConsensusConfig(
  val validatorSet: Set<Validator>,
  override val fork: ChainFork,
) : ConsensusConfig {
  val elFork: ElFork = fork.elFork

  override fun toString(): String =
    "QbftConsensusConfig(fork=${fork.clFork}/${fork.elFork}, validatorSet=$validatorSet)"
}

data class DifficultyAwareQbftConfig(
  val postTtdConfig: QbftConsensusConfig,
  val terminalTotalDifficulty: ULong,
  /**
   * When set, the transition to [postTtdConfig] happens once the latest EL block number is >= this value,
   * instead of relying on the EL block's totalDifficulty (which some EL clients, e.g. Besu >= 26.9.0, no longer
   * return in eth_getBlockBy* responses). It should be the number of the block at which TTD is reached.
   */
  val terminalBlockNumber: ULong? = null,
) : ConsensusConfig {
  override val fork: ChainFork = postTtdConfig.fork

  override fun toString(): String =
    "DifficultyAwareQbftConfig(terminalTotalDifficulty=$terminalTotalDifficulty, " +
      "terminalBlockNumber=$terminalBlockNumber, postTtdConfig=$postTtdConfig)"
}
