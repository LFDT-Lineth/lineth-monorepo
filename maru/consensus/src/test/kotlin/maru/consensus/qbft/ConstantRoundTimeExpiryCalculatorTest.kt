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
import org.junit.jupiter.api.Assertions.assertEquals
import org.junit.jupiter.api.Test
import java.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class ConstantRoundTimeExpiryCalculatorTest {
  @Test
  fun `returns same expiry for different rounds`() {
    val calculator = ConstantRoundTimeExpiryCalculator(2.seconds)

    val rid1 = ConsensusRoundIdentifier(1L, 0)
    val rid2 = ConsensusRoundIdentifier(42L, 7)

    val expected = Duration.ofSeconds(2)

    assertEquals(expected, calculator.calculateRoundExpiry(rid1))
    assertEquals(expected, calculator.calculateRoundExpiry(rid2))
  }

  @Test
  fun `preserves sub-second precision`() {
    val calculator = ConstantRoundTimeExpiryCalculator(1500.milliseconds)

    val rid = ConsensusRoundIdentifier(10L, 3)

    val expected = Duration.ofMillis(1500)

    assertEquals(expected, calculator.calculateRoundExpiry(rid))
  }
}
