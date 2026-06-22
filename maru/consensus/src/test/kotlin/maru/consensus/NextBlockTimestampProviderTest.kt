/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import org.assertj.core.api.Assertions.assertThat
import java.time.Clock
import java.time.Instant
import java.time.ZoneId
import kotlin.test.Test

class NextBlockTimestampProviderTest {
  private val chainId = 1337u
  private val forksSchedule =
    ForksSchedule(
      chainId,
      listOf(
        ForkSpec(0UL, 1u, QbftConsensusConfig(validatorSet = emptySet(), ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague))),
        ForkSpec(
          10UL,
          2u,
          QbftConsensusConfig(validatorSet = emptySet(), ChainFork(ClFork.QBFT_PHASE0, ElFork.Prague)),
        ),
      ),
    )
  private val baseLastBlockTimestamp = 9UL

  private fun createCLockForTimestamp(timestamp: Long): Clock =
    Clock.fixed(Instant.ofEpochMilli(timestamp), ZoneId.of("UTC"))

  @Test
  fun `nextBlockTimestampProvider targets next planned block timestamp`() {
    val nextBlockTimestampProvider =
      NextBlockTimestampProviderImpl(
        createCLockForTimestamp(9999L),
        forksSchedule,
      )

    val result = nextBlockTimestampProvider.nextTargetBlockUnixTimestamp(baseLastBlockTimestamp)

    assertThat(result).isEqualTo(10UL)
  }

  @Test
  fun `if current time is overdue it targets next integer second`() {
    val nextBlockTimestampProvider =
      NextBlockTimestampProviderImpl(
        createCLockForTimestamp(11123),
        forksSchedule,
      )

    val result = nextBlockTimestampProvider.nextTargetBlockUnixTimestamp(baseLastBlockTimestamp)

    assertThat(result).isEqualTo(12UL)
  }

  @Test
  fun `nextBlockTimestampProvider takes into account forks schedule`() {
    val nextBlockTimestampProvider =
      NextBlockTimestampProviderImpl(
        createCLockForTimestamp(11123),
        forksSchedule,
      )
    val result = nextBlockTimestampProvider.nextTargetBlockUnixTimestamp(10UL)

    assertThat(result).isEqualTo(12UL)
  }
}
