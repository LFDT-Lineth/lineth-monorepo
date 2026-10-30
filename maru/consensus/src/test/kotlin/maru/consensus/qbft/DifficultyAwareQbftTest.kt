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
import java.math.BigInteger

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
      headerProvider = chain,
    ).also { it.start() }

  private fun poll() = timerFactory.getTimer("DifficultyAwareQbft")!!.runNextTask()

  @Test
  fun `does not transition before the terminal block`() {
    // QBFT: genesis 0x1 + 8 blocks of difficulty 1 => TD 9 where TTD is 10 (terminal block 9)
    chain.extend(1).extendRepeated(count = 8, difficulty = 1)

    poll()

    verify(postTtdProtocol, never()).start()
  }

  @Test
  fun `transitions at the terminal block`() {
    // QBFT: TD(9) = 1 + 9 = 10 = TTD
    chain.extend(1).extendRepeated(count = 9, difficulty = 1)

    poll()

    verify(postTtdProtocol).start()
  }

  @Test
  fun `transitions when the chain grows to the terminal block across polls`() {
    chain.extend(1).extendRepeated(count = 4, difficulty = 1)
    poll()
    verify(postTtdProtocol, never()).start()

    chain.extendRepeated(count = 5, difficulty = 1)
    poll()
    verify(postTtdProtocol).start()

    // further polls do not start the post-TTD protocol again
    chain.extendRepeated(count = 5, difficulty = 0)
    poll()
    verify(postTtdProtocol, times(1)).start()
  }

  @Test
  fun `waits for block 1 to determine the terminal block`() {
    chain.extend(1)
    poll()
    verify(postTtdProtocol, never()).start()

    chain.extendRepeated(count = 9, difficulty = 1)
    poll()
    verify(postTtdProtocol).start()
  }

  @Test
  fun `transitions at the terminal block of a single-signer Clique chain`() {
    // Clique: genesis 0x1, in-turn blocks of difficulty 2 => TD(4) = 9 < 10, TD(5) = 11 >= 10
    chain.extend(1).extendRepeated(count = 4, difficulty = 2)
    poll()
    verify(postTtdProtocol, never()).start()

    chain.extend(2)
    poll()
    verify(postTtdProtocol).start()
  }

  @Test
  fun `transitions immediately when restarted after the merge`() {
    // pre-merge blocks up to the terminal block 9, then post-merge blocks with difficulty 0
    chain.extend(1).extendRepeated(count = 9, difficulty = 1).extendRepeated(count = 6, difficulty = 0)

    poll()

    verify(postTtdProtocol).start()
  }

  @Test
  fun `keeps using the computed terminal block when a block has an unexpected difficulty`() {
    // block 1 sets the expected difficulty to 1; block 5 has difficulty 2, which is reported but not fatal
    chain.extend(1).extendRepeated(count = 4, difficulty = 1).extend(2)
    poll()
    verify(postTtdProtocol, never()).start()

    chain.extendRepeated(count = 4, difficulty = 1)
    poll()
    verify(postTtdProtocol).start()
  }

  @Test
  fun `switches on the terminal block from an injected calculator`() {
    val stubCalculator =
      object : TerminalBlockCalculator {
        override fun terminalBlock() = TerminalBlock(
          number = 3UL,
          genesisDifficulty = BigInteger.ONE,
          blockDifficulty = null,
        )
      }
    val timerFactory = TestablePeriodicTimerFactory()
    DifficultyAwareQbft(
      ethereumJsonRpcClient = mock(),
      postTtdProtocolFactory =
      object : ProtocolFactory {
        override fun create(forkSpec: ForkSpec): Protocol = postTtdProtocol
      },
      forkSpec = forkSpec,
      timerFactory = timerFactory,
      headerProvider = chain,
      terminalBlockCalculator = stubCalculator,
    ).start()
    val poll = { timerFactory.getTimer("DifficultyAwareQbft")!!.runNextTask() }

    // TTD in the fork config would put the terminal block at 9, but the injected calculator says 3
    chain.extend(1).extendRepeated(count = 2, difficulty = 1)
    poll()
    verify(postTtdProtocol, never()).start()

    chain.extend(1)
    poll()
    verify(postTtdProtocol).start()
  }
}
