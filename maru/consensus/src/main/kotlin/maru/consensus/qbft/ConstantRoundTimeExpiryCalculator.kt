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
import kotlin.time.toJavaDuration

/**
 * A [RoundExpiryTimeCalculator] that always returns a constant round expiry time.
 *
 * Note: This can only be used for a single validator network, otherwise it would break the QBFT protocol.
 *
 * @param roundExpiry the constant duration to be returned for all rounds.
 */
class ConstantRoundTimeExpiryCalculator(
  val roundExpiry: kotlin.time.Duration,
) : RoundExpiryTimeCalculator {
  override fun calculateRoundExpiry(roundIdentifier: ConsensusRoundIdentifier): Duration = roundExpiry.toJavaDuration()
}
