/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import linea.timer.TestablePeriodicTimerFactory
import maru.consensus.ChainFork
import maru.consensus.ClFork
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ElFork
import maru.consensus.ForkSpec
import maru.consensus.ProtocolFactory
import maru.consensus.QbftConsensusConfig
import maru.core.Protocol
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.times
import org.mockito.kotlin.verify

class DifficultyAwareQbftTest {
  private val terminalTotalDifficulty = 10UL
  private val postTtdConfig =
    QbftConsensusConfig(validatorSet = emptySet(), fork = ChainFork(ClFork.QBFT_PHASE0, ElFork.Paris))
  private val forkSpec =
    ForkSpec(
      timestampSeconds = 0UL,
      blockTimeSeconds = 1u,
      configuration = DifficultyAwareQbftConfig(postTtdConfig, terminalTotalDifficulty),
    )

  private val chain = FakeElChain()
  private val postTtdProtocol = mock<Protocol>()
  private val timerFactory = TestablePeriodicTimerFactory()

  private val protocol =
    DifficultyAwareQbft(
      ethereumJsonRpcClient = mock(),
      postTtdProtocolFactory =
      object : ProtocolFactory {
        override fun create(forkSpec: ForkSpec): Protocol = postTtdProtocol
      },
      forkSpec = forkSpec,
      timerFactory = timerFactory,
      totalDifficultyTracker = TotalDifficultyTracker(chain),
    ).also { it.start() }

  private fun poll() = timerFactory.getTimer("DifficultyAwareQbft")!!.runNextTask()

  @Test
  fun `does not transition while total difficulty is below TTD`() {
    // genesis + 8 blocks of difficulty 1 => TD 9 where TTD is 10
    chain.extend(List(9) { 1L })

    poll()

    verify(postTtdProtocol, never()).start()
  }

  @Test
  fun `transitions once summed block difficulties reach TTD`() {
    // genesis + 9 blocks of difficulty 1 => TD 10 at block 9
    chain.extend(List(10) { 1L })

    poll()

    verify(postTtdProtocol).start()
  }

  @Test
  fun `transitions when the chain grows to TTD across polls`() {
    chain.extend(List(5) { 1L })
    poll()
    verify(postTtdProtocol, never()).start()

    chain.extend(List(5) { 1L })
    poll()
    verify(postTtdProtocol).start()

    // further polls do not start the post-TTD protocol again
    chain.extend(List(5) { 1L })
    poll()
    verify(postTtdProtocol, times(1)).start()
  }

  @Test
  fun `transitions after a reorg to a heavier chain reaches TTD`() {
    // Clique-like: TD 8 on fork a
    chain.extend(listOf(1, 1, 1, 1, 1, 1, 1, 1))
    poll()
    verify(postTtdProtocol, never()).start()

    // reorg of the last 3 blocks to in-turn blocks of difficulty 2 => TD 5 + 6 = 11
    chain.reorg(fromBlockNumber = 5, difficulties = listOf(2, 2, 2), fork = "b")
    poll()

    verify(postTtdProtocol).start()
  }

  @Test
  fun `does not transition when a reorg lowers total difficulty below TTD`() {
    // TD 9 on fork a
    chain.extend(listOf(1, 2, 2, 2, 2))
    poll()

    // reorg to out-of-turn blocks => TD 1 + 2 + 1 + 1 + 1 + 1 + 1 + 1 = 9: still below TTD
    chain.reorg(fromBlockNumber = 2, difficulties = listOf(1, 1, 1, 1, 1, 1), fork = "b")
    poll()

    verify(postTtdProtocol, never()).start()
  }
}
