/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import linea.domain.Block
import linea.domain.BlockParameter
import linea.domain.BlockWithTxHashes
import linea.domain.createBlock
import linea.domain.toBlockWithRandomTxHashes
import linea.ethapi.EthApiBlockClient
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

/** In-memory EL chain: block n has the n-th difficulty added via [extend]. */
internal class FakeElChain : EthApiBlockClient {
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

  override fun ethBlockNumber(): SafeFuture<ULong> = SafeFuture.completedFuture(difficulties.lastIndex.toULong())

  override fun ethFindBlockByNumberFullTxs(blockParameter: BlockParameter): SafeFuture<Block?> =
    SafeFuture.completedFuture(findBlock(blockParameter))

  override fun ethFindBlockByNumberTxHashes(blockParameter: BlockParameter): SafeFuture<BlockWithTxHashes?> =
    SafeFuture.completedFuture(findBlock(blockParameter)?.toBlockWithRandomTxHashes())

  private fun findBlock(blockParameter: BlockParameter): Block? {
    val blockNumber =
      when (blockParameter) {
        BlockParameter.Tag.LATEST -> difficulties.lastIndex.takeIf { it >= 0 }?.toULong() ?: return null
        is BlockParameter.BlockNumber -> blockParameter.number.also { requestedBlockNumbers.add(it) }
        else -> throw UnsupportedOperationException("Unsupported blockParameter=$blockParameter")
      }
    val difficulty = difficulties.getOrNull(blockNumber.toInt()) ?: return null
    return createBlock(number = blockNumber, difficulty = difficulty.toLong().toULong())
  }
}
