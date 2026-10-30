/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.methods.response.EthBlock
import java.math.BigInteger

/** The subset of an EL block header needed to determine the terminal block. */
data class ElBlockHeader(
  val number: ULong,
  val difficulty: BigInteger,
)

/** Reads canonical EL block headers. */
interface ElBlockHeaderProvider {
  fun latestHeader(): ElBlockHeader?

  fun headerByNumber(blockNumber: ULong): ElBlockHeader?
}

class Web3jElBlockHeaderProvider(
  private val web3j: Web3j,
) : ElBlockHeaderProvider {
  override fun latestHeader(): ElBlockHeader? = getHeader(DefaultBlockParameter.valueOf("latest"))

  override fun headerByNumber(blockNumber: ULong): ElBlockHeader? =
    getHeader(DefaultBlockParameter.valueOf(BigInteger(blockNumber.toString())))

  private fun getHeader(blockParameter: DefaultBlockParameter): ElBlockHeader? =
    web3j
      .ethGetBlockByNumber(blockParameter, false)
      .send()
      .block
      ?.toElBlockHeader()

  private fun EthBlock.Block.toElBlockHeader(): ElBlockHeader =
    ElBlockHeader(number = number.toLong().toULong(), difficulty = difficulty)
}

/**
 * The first EL block whose total difficulty reaches TTD.
 *
 * @param blockDifficulty difficulty of every block after genesis, or null when TTD is already reached at genesis
 */
data class TerminalBlock(
  val number: ULong,
  val genesisDifficulty: BigInteger,
  val blockDifficulty: BigInteger?,
)

/** Determines the EL terminal block, i.e. the first block whose total difficulty reaches TTD. */
interface TerminalBlockCalculator {
  /** Returns the terminal block, or null if the EL does not have the blocks needed to determine it yet. */
  fun terminalBlock(): TerminalBlock?
}

/**
 * Determines the EL terminal block from TTD, as EL clients no longer return `totalDifficulty` in `eth_getBlockBy*`
 * responses (e.g. Besu >= 26.9.0, Geth >= 1.14.11).
 *
 * Total difficulty is the sum of the difficulties of all blocks from genesis. This assumes every block after genesis
 * has the same difficulty and the pre-merge chain does not reorg, which holds for:
 * - QBFT / IBFT 2.0: every block has difficulty 1 and blocks are final once committed;
 * - Clique with a single, never changing, signer: every block is in-turn with difficulty 2.
 *
 * The genesis difficulty is arbitrary (set in the EL genesis file), so it is read from block 0 and the per-block
 * difficulty from block 1. Total difficulty at block n is then `genesisDifficulty + n * blockDifficulty`, and the
 * terminal block is the first n for which it reaches TTD.
 */
class ConstantDifficultyTerminalBlockCalculator(
  private val headerProvider: ElBlockHeaderProvider,
  private val terminalTotalDifficulty: BigInteger,
) : TerminalBlockCalculator {
  @Volatile
  private var terminalBlock: TerminalBlock? = null

  override fun terminalBlock(): TerminalBlock? {
    terminalBlock?.let { return it }
    val genesisDifficulty = headerProvider.headerByNumber(0UL)?.difficulty ?: return null
    val computed =
      if (genesisDifficulty >= terminalTotalDifficulty) {
        TerminalBlock(number = 0UL, genesisDifficulty = genesisDifficulty, blockDifficulty = null)
      } else {
        val blockDifficulty = headerProvider.headerByNumber(1UL)?.difficulty ?: return null
        check(blockDifficulty.signum() > 0) {
          "EL block 1 has difficulty 0 (post-merge) while genesis difficulty=$genesisDifficulty is below " +
            "terminalTotalDifficulty=$terminalTotalDifficulty: Maru TTD does not match the EL TTD"
        }
        val remainingDifficulty = terminalTotalDifficulty - genesisDifficulty
        val number = (remainingDifficulty + blockDifficulty - BigInteger.ONE) / blockDifficulty
        TerminalBlock(
          number = number.toLong().toULong(),
          genesisDifficulty = genesisDifficulty,
          blockDifficulty = blockDifficulty,
        )
      }
    terminalBlock = computed
    return computed
  }
}
