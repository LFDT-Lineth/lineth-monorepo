/*
 * Copyright Consensys Software Inc.
 *
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
import org.mockito.kotlin.any
import org.mockito.kotlin.doReturn
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.methods.response.EthBlock

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

  private fun web3jReturning(block: EthBlock.Block): Web3j {
    val ethBlock = EthBlock().apply { result = block }
    val request = mock<Request<*, EthBlock>>()
    whenever(request.send()).thenReturn(ethBlock)
    val web3j = mock<Web3j>()
    doReturn(request).whenever(web3j).ethGetBlockByNumber(any(), any())
    return web3j
  }

  private fun block(
    number: Long,
    totalDifficulty: Long?,
  ): EthBlock.Block =
    EthBlock.Block().apply {
      setNumber("0x" + number.toString(16))
      setTotalDifficulty(totalDifficulty?.let { "0x" + it.toString(16) })
    }

  private fun createAndPoll(
    block: EthBlock.Block,
    forkSpec: ForkSpec = this.forkSpec,
  ): Protocol {
    val postTtdProtocol = mock<Protocol>()
    val postTtdProtocolFactory =
      object : ProtocolFactory {
        override fun create(forkSpec: ForkSpec): Protocol = postTtdProtocol
      }
    val timerFactory = TestablePeriodicTimerFactory()
    val protocol =
      DifficultyAwareQbft(
        ethereumJsonRpcClient = web3jReturning(block),
        postTtdProtocolFactory = postTtdProtocolFactory,
        forkSpec = forkSpec,
        timerFactory = timerFactory,
      )
    protocol.start()
    timerFactory.getTimer("DifficultyAwareQbft")!!.runNextTask()
    return postTtdProtocol
  }

  @Test
  fun `transitions to post-TTD protocol when totalDifficulty reaches TTD`() {
    val postTtdProtocol = createAndPoll(block(number = 9, totalDifficulty = terminalTotalDifficulty.toLong()))

    verify(postTtdProtocol).start()
  }

  @Test
  fun `does not transition when totalDifficulty is below TTD`() {
    val postTtdProtocol = createAndPoll(block(number = 5, totalDifficulty = 6))

    verify(postTtdProtocol, never()).start()
  }

  @Test
  fun `does not transition when EL omits totalDifficulty`() {
    val postTtdProtocol = createAndPoll(block(number = 100, totalDifficulty = null))

    verify(postTtdProtocol, never()).start()
  }

  private fun forkSpecWithTerminalBlockNumber(terminalBlockNumber: ULong): ForkSpec =
    forkSpec.copy(
      configuration =
      DifficultyAwareQbftConfig(postTtdConfig, terminalTotalDifficulty, terminalBlockNumber = terminalBlockNumber),
    )

  @Test
  fun `transitions when terminalBlockNumber is reached even if EL omits totalDifficulty`() {
    val postTtdProtocol =
      createAndPoll(block(number = 9, totalDifficulty = null), forkSpecWithTerminalBlockNumber(9UL))

    verify(postTtdProtocol).start()
  }

  @Test
  fun `does not transition when terminalBlockNumber is not reached`() {
    val postTtdProtocol =
      createAndPoll(block(number = 8, totalDifficulty = null), forkSpecWithTerminalBlockNumber(9UL))

    verify(postTtdProtocol, never()).start()
  }

  @Test
  fun `terminalBlockNumber takes precedence over totalDifficulty`() {
    val postTtdProtocol =
      createAndPoll(
        block(number = 8, totalDifficulty = terminalTotalDifficulty.toLong()),
        forkSpecWithTerminalBlockNumber(9UL),
      )

    verify(postTtdProtocol, never()).start()
  }
}
