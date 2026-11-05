/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import linea.domain.BlockParameter
import linea.domain.BlockWithTxHashes
import linea.ethapi.EthApiBlockClient
import linea.timer.Timer
import linea.timer.TimerFactory
import linea.timer.VertxTimerFactory
import maru.consensus.DifficultyAwareQbftConfig
import maru.consensus.ForkSpec
import maru.consensus.ProtocolFactory
import maru.core.Protocol
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import java.math.BigInteger
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class DifficultyAwareQbftFactory(
  private val ethApiClient: EthApiBlockClient?,
  private val postTtdProtocolFactory: ProtocolFactory,
  private val timerFactory: TimerFactory,
) : ProtocolFactory {
  override fun create(forkSpec: ForkSpec): DifficultyAwareQbft {
    require(ethApiClient != null) {
      "L2 Ethereum API endpoint has to be set if the DifficultyAwareQbftFactory needs to be created"
    }
    return DifficultyAwareQbft(
      ethApiClient = ethApiClient,
      postTtdProtocolFactory = postTtdProtocolFactory,
      forkSpec = forkSpec,
      timerFactory = timerFactory,
    )
  }
}

class DifficultyAwareQbft(
  private val ethApiClient: EthApiBlockClient,
  private val postTtdProtocolFactory: ProtocolFactory,
  private val forkSpec: ForkSpec,
  private val timerFactory: TimerFactory,
  // EL clients no longer return totalDifficulty in eth_getBlockBy* responses, so the terminal block is derived from TTD
  private val terminalBlockCalculator: TerminalBlockCalculator =
    ConstantDifficultyTerminalBlockCalculator(
      ethApiClient = ethApiClient,
      terminalTotalDifficulty =
      BigInteger((forkSpec.configuration as DifficultyAwareQbftConfig).terminalTotalDifficulty.toString()),
    ),
) : Protocol {
  private val log: Logger = LogManager.getLogger(this.javaClass)

  private var poller: Timer? = null
  private var postTtdProtocol: Protocol? = null
  private var unexpectedDifficultyReported = false

  private fun pollTask() {
    val difficultyAwareQbftConfig = forkSpec.configuration as DifficultyAwareQbftConfig
    try {
      if (postTtdProtocol != null) {
        // TTD already reached; poller should have been stopped but may fire one extra time.
        stopPoller()
        return
      }

      val latestBlock = ethApiClient.ethFindBlockByNumberTxHashes(BlockParameter.Tag.LATEST).get()
      if (latestBlock == null) {
        log.warn("Failed to retrieve latest block from EL")
        return
      }

      val terminalBlock = terminalBlockCalculator.terminalBlock()
      if (terminalBlock == null) {
        log.debug(
          "Waiting for the terminal block to be determined: elBlockNumber={}",
          latestBlock.number,
        )
        return
      }
      checkConstantBlockDifficulty(latestBlock, terminalBlock)

      log.debug(
        "Current elBlockNumber={}, terminalBlockNumber={}, terminalTotalDifficulty={}",
        latestBlock.number,
        terminalBlock.number,
        difficultyAwareQbftConfig.terminalTotalDifficulty,
      )
      if (latestBlock.number < terminalBlock.number) {
        return
      }

      log.info(
        "TTD reached at elBlockNumber={} (terminalBlockNumber={}, genesisDifficulty={}, blockDifficulty={}). " +
          "Transitioning to post-TTD protocol.",
        latestBlock.number,
        terminalBlock.number,
        terminalBlock.genesisDifficulty,
        terminalBlock.blockDifficulty,
      )
      val postTtdForkSpec =
        ForkSpec(
          timestampSeconds = forkSpec.timestampSeconds,
          blockTimeSeconds = forkSpec.blockTimeSeconds,
          configuration = difficultyAwareQbftConfig.postTtdConfig,
        )
      transitionToPostTtdProtocol(postTtdForkSpec)
      stopPoller()
    } catch (e: Exception) {
      log.error("Error during EL block polling", e)
    }
  }

  /**
   * The terminal block is computed assuming every block after genesis has the same difficulty as block 1. Report
   * blocks that break that assumption (e.g. an out-of-turn Clique block when there is more than one signer), as the
   * computed terminal block may then not match the EL's. Post-merge blocks have difficulty 0 and are expected.
   */
  private fun checkConstantBlockDifficulty(
    latestBlock: BlockWithTxHashes,
    terminalBlock: TerminalBlock,
  ) {
    val expectedDifficulty = terminalBlock.blockDifficulty ?: return
    val latestDifficulty = BigInteger(latestBlock.difficulty.toString())
    if (latestBlock.number == 0UL ||
      latestDifficulty.signum() == 0 ||
      latestDifficulty == expectedDifficulty ||
      unexpectedDifficultyReported
    ) {
      return
    }
    log.error(
      "EL block {} has difficulty {} while every block after genesis is expected to have difficulty {} (as block 1). " +
        "The terminal block {} may not match the EL's: only QBFT/IBFT 2.0 and single-signer Clique pre-merge " +
        "chains are supported.",
      latestBlock.number,
      latestDifficulty,
      expectedDifficulty,
      terminalBlock.number,
    )
    unexpectedDifficultyReported = true
  }

  @Synchronized
  private fun transitionToPostTtdProtocol(postTtdForkSpec: ForkSpec) {
    if (postTtdProtocol != null) {
      throw IllegalStateException("This protocol is supposed to be stopped after reaching TTD")
    }

    try {
      log.info("Creating post-TTD protocol forkSpec={}", postTtdForkSpec)
      postTtdProtocol = postTtdProtocolFactory.create(postTtdForkSpec)
      postTtdProtocol?.start()
      log.info("Post-TTD protocol started successfully")
    } catch (e: Exception) {
      log.error("Failed to start post-TTD protocol", e)
      throw e
    }
  }

  override fun start() {
    synchronized(this) {
      if (poller != null) {
        return
      }
      log.debug("Starting DifficultyAwareQbft with pollingInterval={} seconds", forkSpec.blockTimeSeconds)
      poller = timerFactory.createTimer(
        name = "DifficultyAwareQbft",
        period = forkSpec.blockTimeSeconds.toInt().seconds,
        initialDelay = if (timerFactory is VertxTimerFactory) 1.milliseconds else Duration.ZERO,
        timerSchedule = linea.timer.TimerSchedule.FIXED_RATE,
        errorHandler = { e -> log.warn("DifficultyAwareQbft poll task exception", e) },
        task = Runnable { pollTask() },
      )
      poller?.start()
      postTtdProtocol?.start()
    }
  }

  override fun pause() {
    synchronized(this) {
      if (poller != null) {
        stopPoller()
      }

      if (postTtdProtocol != null) {
        log.debug("Stopping post-TTD protocol")
        try {
          postTtdProtocol?.pause()
        } catch (e: Exception) {
          log.warn("Error stopping post-TTD protocol", e)
        }
      }
    }
  }

  override fun close() {
    synchronized(this) {
      pause()
      postTtdProtocol?.close()
    }
  }

  private fun stopPoller() {
    log.debug("Stopping DifficultyAwareQbft poller")
    poller?.stop()
    poller = null
  }
}
