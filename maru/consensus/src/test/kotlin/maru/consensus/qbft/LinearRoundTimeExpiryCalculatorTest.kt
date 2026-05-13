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
import kotlin.time.Duration.Companion.seconds

class LinearRoundTimeExpiryCalculatorTest {
  @Test
  fun `calculates expiry for round number`() {
    val baseExpiry = 1.seconds
    val calculator = LinearRoundTimeExpiryCalculator(baseExpiry)

    val round0 = calculator.calculateRoundExpiry(ConsensusRoundIdentifier(1L, 0))
    val round1 = calculator.calculateRoundExpiry(ConsensusRoundIdentifier(1L, 1))
    val round2 = calculator.calculateRoundExpiry(ConsensusRoundIdentifier(1L, 2))
    val round4 = calculator.calculateRoundExpiry(ConsensusRoundIdentifier(1L, 4))

    assertEquals(Duration.ofMillis(1000), round0) // 1000
    assertEquals(Duration.ofMillis(2000), round1) // 1000 + 1000
    assertEquals(Duration.ofMillis(3000), round2) // 1000 + 2000
    assertEquals(Duration.ofMillis(5000), round4) // 1000 + 4000
  }

  @Test
  fun `sequence number does not affect calculation`() {
    val baseExpiry = 2.seconds
    val calculator = LinearRoundTimeExpiryCalculator(baseExpiry)

    val rid1 = ConsensusRoundIdentifier(1L, 3)
    val rid2 = ConsensusRoundIdentifier(42L, 3)
    val rid3 = ConsensusRoundIdentifier(999L, 3)

    // All should have same expiry for round 3: 2000 + (3 * 2000 * 1.0) = 2000 + 6000 = 8000ms
    val expected = Duration.ofMillis(8000)

    assertEquals(expected, calculator.calculateRoundExpiry(rid1))
    assertEquals(expected, calculator.calculateRoundExpiry(rid2))
    assertEquals(expected, calculator.calculateRoundExpiry(rid3))
  }

  @Test
  fun `calculates correctly with different coefficients`() {
    val baseExpiry = 1.seconds

    val calculator1 = LinearRoundTimeExpiryCalculator(baseExpiry, 0.5)
    val calculator2 = LinearRoundTimeExpiryCalculator(baseExpiry, 1.0)
    val calculator3 = LinearRoundTimeExpiryCalculator(baseExpiry, 2.0)

    val rid = ConsensusRoundIdentifier(1L, 3)

    // Round 3 with different coefficients:
    val result1 = calculator1.calculateRoundExpiry(rid) // 1000 + (3 * 1000 * 0.5) = 2500ms
    val result2 = calculator2.calculateRoundExpiry(rid) // 1000 + (3 * 1000 * 1.0) = 4000ms
    val result3 = calculator3.calculateRoundExpiry(rid) // 1000 + (3 * 1000 * 2.0) = 7000ms

    assertEquals(Duration.ofMillis(2500), result1)
    assertEquals(Duration.ofMillis(4000), result2)
    assertEquals(Duration.ofMillis(7000), result3)
  }
}
