/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.RoundExpiryTimeCalculator
import java.time.Duration

/**
 * A [RoundExpiryTimeCalculator] that uses a linear algorithm for increasing the round expiry time.
 * This implementation calculates the round expiry time for a round as:
 * `baseExpiryPeriod + (round * baseExpiryPeriod * coefficient)`.
 *
 * @param baseExpiryPeriod the duration used as the base to be returned for all rounds.
 */
class LinearRoundTimeExpiryCalculator(
  val baseExpiryPeriod: kotlin.time.Duration,
  val coefficient: Double = 1.0,
) : RoundExpiryTimeCalculator {
  override fun calculateRoundExpiry(roundIdentifier: ConsensusRoundIdentifier): Duration {
    val increment = roundIdentifier.roundNumber * baseExpiryPeriod.inWholeMilliseconds * coefficient
    return Duration.ofMillis(baseExpiryPeriod.inWholeMilliseconds + increment.toLong())
  }
}
