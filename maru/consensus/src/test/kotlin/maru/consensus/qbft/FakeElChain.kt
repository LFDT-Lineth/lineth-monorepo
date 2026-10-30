/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import java.math.BigInteger

/** In-memory EL chain: block n has the n-th difficulty added via [extend]. */
internal class FakeElChain : ElBlockHeaderProvider {
  val difficulties = mutableListOf<BigInteger>()
  val requestedBlockNumbers = mutableListOf<ULong>()

  fun extend(vararg difficulties: Long): FakeElChain {
    difficulties.forEach { this.difficulties.add(BigInteger.valueOf(it)) }
    return this
  }

  fun extendRepeated(
    count: Int,
    difficulty: Long,
  ): FakeElChain = extend(*LongArray(count) { difficulty })

  override fun latestHeader(): ElBlockHeader? =
    difficulties.lastIndex.takeIf { it >= 0 }?.let { ElBlockHeader(it.toULong(), difficulties[it]) }

  override fun headerByNumber(blockNumber: ULong): ElBlockHeader? {
    requestedBlockNumbers.add(blockNumber)
    return difficulties.getOrNull(blockNumber.toInt())?.let { ElBlockHeader(blockNumber, it) }
  }
}
