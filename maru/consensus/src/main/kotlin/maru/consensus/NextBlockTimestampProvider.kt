/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import java.time.Clock
import kotlin.math.ceil
import kotlin.math.max

fun interface NextBlockTimestampProvider {
  fun nextTargetBlockUnixTimestamp(lastBlockTimestamp: ULong): ULong
}

class NextBlockTimestampProviderImpl(
  private val clock: Clock,
  private val forksSchedule: ForksSchedule,
) : NextBlockTimestampProvider {
  override fun nextTargetBlockUnixTimestamp(lastBlockTimestamp: ULong): ULong {
    val currentBlockTime = forksSchedule.getForkByTimestamp(lastBlockTimestamp).blockTimeSeconds

    val nextIntegerSecond = ceil(clock.millis() / 1000.0).toULong()
    return max(lastBlockTimestamp + currentBlockTime, nextIntegerSecond)
  }
}
