/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import java.math.BigInteger

/** In-memory EL canonical chain. Blocks are identified by a fork label so reorged blocks get distinct hashes. */
internal class FakeElChain : ElBlockHeaderProvider {
  val canonical = mutableListOf<ElBlockHeader>()
  val requestedRanges = mutableListOf<ULongRange>()

  /** Invoked before serving each range request, to simulate the canonical chain changing mid-sync. */
  var beforeRangeRequest: () -> Unit = {}

  fun extend(
    difficulties: List<Long>,
    fork: String = "a",
  ): FakeElChain {
    difficulties.forEach { difficulty ->
      val number = canonical.size.toULong()
      canonical.add(
        ElBlockHeader(
          number = number,
          hash = "0x$fork-$number",
          parentHash = canonical.lastOrNull()?.hash ?: "0x0",
          difficulty = BigInteger.valueOf(difficulty),
        ),
      )
    }
    return this
  }

  /** Replaces the canonical chain from [fromBlockNumber] with new blocks of the given [difficulties]. */
  fun reorg(
    fromBlockNumber: Int,
    difficulties: List<Long>,
    fork: String,
  ) {
    while (canonical.size > fromBlockNumber) {
      canonical.removeLast()
    }
    extend(difficulties, fork)
  }

  override fun latestHeader(): ElBlockHeader? = canonical.lastOrNull()

  override fun headersByNumber(
    fromBlockNumber: ULong,
    toBlockNumber: ULong,
  ): List<ElBlockHeader> {
    beforeRangeRequest()
    requestedRanges.add(fromBlockNumber..toBlockNumber)
    return canonical.filter { it.number in fromBlockNumber..toBlockNumber }
  }
}
