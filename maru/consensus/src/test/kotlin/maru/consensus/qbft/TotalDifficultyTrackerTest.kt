/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.math.BigInteger

class TotalDifficultyTrackerTest {
  private fun totalDifficultyOf(chain: FakeElChain): BigInteger = chain.canonical.sumOf { it.difficulty }

  private fun tracker(
    chain: FakeElChain,
    reorgWindowSize: Int = TotalDifficultyTracker.DEFAULT_REORG_WINDOW_SIZE,
    fetchBatchSize: ULong = TotalDifficultyTracker.DEFAULT_FETCH_BATCH_SIZE,
  ) = TotalDifficultyTracker(chain, reorgWindowSize, fetchBatchSize)

  @Test
  fun `returns null when EL has no blocks`() {
    assertThat(tracker(FakeElChain()).update()).isNull()
  }

  @Test
  fun `sums difficulties from genesis including genesis difficulty`() {
    // QBFT-like chain: genesis difficulty 1, every block difficulty 1 => block N has TD N + 1
    val chain = FakeElChain().extend(List(20) { 1L })

    val head = tracker(chain).update()!!

    assertThat(head.blockNumber).isEqualTo(19UL)
    assertThat(head.blockHash).isEqualTo("0xa-19")
    assertThat(head.totalDifficulty).isEqualTo(BigInteger.valueOf(20))
  }

  @Test
  fun `sums variable difficulties`() {
    // Clique-like chain: in-turn blocks have difficulty 2, out-of-turn ones 1
    val chain = FakeElChain().extend(listOf(1, 2, 2, 1, 2, 1, 1, 2))

    assertThat(tracker(chain).update()!!.totalDifficulty).isEqualTo(BigInteger.valueOf(12))
  }

  @Test
  fun `syncs chains longer than the fetch batch size`() {
    val chain = FakeElChain().extend(List(1000) { (it % 3 + 1).toLong() })

    val head = tracker(chain, fetchBatchSize = 64UL).update()!!

    assertThat(head.blockNumber).isEqualTo(999UL)
    assertThat(head.totalDifficulty).isEqualTo(totalDifficultyOf(chain))
    assertThat(chain.requestedRanges).allMatch { it.last - it.first + 1UL <= 64UL }
  }

  @Test
  fun `only fetches new blocks when the chain is extended`() {
    val chain = FakeElChain().extend(List(10) { 1L })
    val tracker = tracker(chain)
    tracker.update()

    chain.requestedRanges.clear()
    chain.extend(List(5) { 2L })
    val head = tracker.update()!!

    assertThat(head.blockNumber).isEqualTo(14UL)
    assertThat(head.totalDifficulty).isEqualTo(BigInteger.valueOf(20))
    // one canonicity check of the tracked head, then only the 5 new blocks
    assertThat(chain.requestedRanges).containsExactly(9UL..9UL, 10UL..14UL)
  }

  @Test
  fun `does not fetch ranges when the head did not change`() {
    val chain = FakeElChain().extend(List(10) { 1L })
    val tracker = tracker(chain)
    tracker.update()

    chain.requestedRanges.clear()
    val head = tracker.update()!!

    assertThat(head.totalDifficulty).isEqualTo(BigInteger.valueOf(10))
    assertThat(chain.requestedRanges).isEmpty()
  }

  @Test
  fun `recomputes total difficulty after a same-height reorg`() {
    val chain = FakeElChain().extend(listOf(1, 2, 2, 2, 2))
    val tracker = tracker(chain)
    assertThat(tracker.update()!!.totalDifficulty).isEqualTo(BigInteger.valueOf(9))

    // last 2 blocks replaced by out-of-turn blocks
    chain.reorg(fromBlockNumber = 3, difficulties = listOf(1, 1), fork = "b")
    val head = tracker.update()!!

    assertThat(head.blockNumber).isEqualTo(4UL)
    assertThat(head.blockHash).isEqualTo("0xb-4")
    assertThat(head.totalDifficulty).isEqualTo(BigInteger.valueOf(7))
  }

  @Test
  fun `follows a reorg to a shorter but heavier chain`() {
    val chain = FakeElChain().extend(listOf(1, 2, 1, 1, 1, 1))
    val tracker = tracker(chain)
    assertThat(tracker.update()!!.totalDifficulty).isEqualTo(BigInteger.valueOf(7))

    // Clique can reorg to a shorter chain with more in-turn blocks
    chain.reorg(fromBlockNumber = 2, difficulties = listOf(2, 2, 2), fork = "b")
    val head = tracker.update()!!

    assertThat(head.blockNumber).isEqualTo(4UL)
    assertThat(head.blockHash).isEqualTo("0xb-4")
    assertThat(head.totalDifficulty).isEqualTo(BigInteger.valueOf(9))
  }

  @Test
  fun `re-sums from genesis on a reorg deeper than the reorg window`() {
    val chain = FakeElChain().extend(List(50) { 1L })
    val tracker = tracker(chain, reorgWindowSize = 8)
    assertThat(tracker.update()!!.totalDifficulty).isEqualTo(BigInteger.valueOf(50))

    chain.reorg(fromBlockNumber = 20, difficulties = List(40) { 2L }, fork = "b")
    chain.requestedRanges.clear()
    val head = tracker.update()!!

    assertThat(head.blockNumber).isEqualTo(59UL)
    assertThat(head.blockHash).isEqualTo("0xb-59")
    assertThat(head.totalDifficulty).isEqualTo(BigInteger.valueOf(20 + 40 * 2))
    assertThat(chain.requestedRanges).anyMatch { it.first == 0UL }
  }

  @Test
  fun `handles a reorg of the genesis-adjacent blocks`() {
    val chain = FakeElChain().extend(listOf(1, 1, 1))
    val tracker = tracker(chain)
    tracker.update()

    chain.reorg(fromBlockNumber = 1, difficulties = listOf(2, 2, 2), fork = "b")

    assertThat(tracker.update()!!.totalDifficulty).isEqualTo(BigInteger.valueOf(7))
  }

  @Test
  fun `never over-reports total difficulty when the canonical chain changes mid-sync`() {
    val chain = FakeElChain().extend(List(10) { 1L })
    val tracker = tracker(chain, fetchBatchSize = 4UL)
    tracker.update()

    chain.extend(List(10) { 1L })
    chain.requestedRanges.clear()
    var reorged = false
    chain.beforeRangeRequest = {
      // after blocks 10..13 were fetched, blocks from 12 onwards are replaced by a shorter fork
      if (chain.requestedRanges.size == 2 && !reorged) {
        reorged = true
        chain.reorg(fromBlockNumber = 12, difficulties = List(3) { 1L }, fork = "b")
      }
    }
    val partialHead = tracker.update()!!

    // the next fetched block (b-14) does not extend the tracked a-13, so tracking stops at the last consistent block
    assertThat(partialHead.blockNumber).isEqualTo(13UL)
    assertThat(partialHead.blockHash).isEqualTo("0xa-13")
    assertThat(partialHead.totalDifficulty).isLessThanOrEqualTo(totalDifficultyOf(chain))

    chain.beforeRangeRequest = {}
    val head = tracker.update()!!

    assertThat(head.blockNumber).isEqualTo(14UL)
    assertThat(head.blockHash).isEqualTo("0xb-14")
    assertThat(head.totalDifficulty).isEqualTo(totalDifficultyOf(chain))
  }

  @Test
  fun `keeps tracked blocks bounded by the reorg window`() {
    val chain = FakeElChain().extend(List(100) { 1L })
    val tracker = tracker(chain, reorgWindowSize = 10)
    tracker.update()

    // a reorg within the window rewinds without re-summing from genesis
    chain.reorg(fromBlockNumber = 95, difficulties = List(5) { 3L }, fork = "b")
    chain.requestedRanges.clear()
    val head = tracker.update()!!

    assertThat(head.totalDifficulty).isEqualTo(BigInteger.valueOf(95 + 5 * 3))
    assertThat(chain.requestedRanges).noneMatch { it.first == 0UL }
  }
}
