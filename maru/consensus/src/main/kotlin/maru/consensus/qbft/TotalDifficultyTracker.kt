/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.methods.response.EthBlock
import java.math.BigInteger

/** The subset of an EL block header needed to compute total difficulty. */
data class ElBlockHeader(
  val number: ULong,
  val hash: String,
  val parentHash: String,
  val difficulty: BigInteger,
)

/** Total difficulty of the EL chain at a given block, i.e. the sum of the difficulties of all blocks up to it. */
data class ElChainTotalDifficulty(
  val blockNumber: ULong,
  val blockHash: String,
  val totalDifficulty: BigInteger,
)

/** Reads canonical EL block headers. */
interface ElBlockHeaderProvider {
  fun latestHeader(): ElBlockHeader?

  /**
   * Returns the canonical headers for the inclusive range [fromBlockNumber, toBlockNumber], in ascending order. The
   * result stops early, possibly empty, at the first block the EL does not have.
   */
  fun headersByNumber(
    fromBlockNumber: ULong,
    toBlockNumber: ULong,
  ): List<ElBlockHeader>
}

class Web3jElBlockHeaderProvider(
  private val web3j: Web3j,
) : ElBlockHeaderProvider {
  override fun latestHeader(): ElBlockHeader? =
    web3j
      .ethGetBlockByNumber(DefaultBlockParameter.valueOf("latest"), false)
      .send()
      .block
      ?.toElBlockHeader()

  override fun headersByNumber(
    fromBlockNumber: ULong,
    toBlockNumber: ULong,
  ): List<ElBlockHeader> {
    if (fromBlockNumber > toBlockNumber) {
      return emptyList()
    }
    val batch = web3j.newBatch()
    var blockNumber = fromBlockNumber
    while (blockNumber <= toBlockNumber) {
      batch.add(
        web3j.ethGetBlockByNumber(DefaultBlockParameter.valueOf(BigInteger(blockNumber.toString())), false),
      )
      blockNumber++
    }
    val headers = mutableListOf<ElBlockHeader>()
    for (response in batch.send().responses) {
      val ethBlock = response as EthBlock
      if (ethBlock.hasError()) {
        throw IllegalStateException(
          "Failed to get block from EL: code=${ethBlock.error.code} message=${ethBlock.error.message}",
        )
      }
      val block = ethBlock.block ?: break
      headers.add(block.toElBlockHeader())
    }
    return headers
  }

  private fun EthBlock.Block.toElBlockHeader(): ElBlockHeader =
    ElBlockHeader(
      number = number.toLong().toULong(),
      hash = hash.lowercase(),
      parentHash = parentHash.lowercase(),
      difficulty = difficulty,
    )
}

/**
 * Tracks the total difficulty of the EL canonical chain head by summing the `difficulty` of every block from genesis,
 * the same way EL clients compute it. Needed because EL clients no longer return `totalDifficulty` in
 * `eth_getBlockBy*` responses (e.g. Besu >= 26.9.0, Geth >= 1.14.11).
 *
 * Blocks are linked by parent hash, so the tracked chain always matches the EL canonical chain:
 * - when the EL head moved forward on the same chain, only the new blocks are fetched;
 * - on a reorg, the tracked chain is rewound to the common ancestor with the canonical chain and re-extended;
 * - a reorg deeper than [reorgWindowSize] blocks, or past genesis, resets the tracker and re-sums from genesis.
 *
 * If the canonical chain changes while blocks are being fetched, [update] stops at the last consistent block and
 * reports a lower total difficulty; the next [update] resolves it. Total difficulty is therefore never over-reported.
 *
 * Not thread-safe: [update] must be called from a single thread.
 */
class TotalDifficultyTracker(
  private val headerProvider: ElBlockHeaderProvider,
  private val reorgWindowSize: Int = DEFAULT_REORG_WINDOW_SIZE,
  private val fetchBatchSize: ULong = DEFAULT_FETCH_BATCH_SIZE,
  private val log: Logger = LogManager.getLogger(TotalDifficultyTracker::class.java),
) {
  companion object {
    const val DEFAULT_REORG_WINDOW_SIZE = 1024
    const val DEFAULT_FETCH_BATCH_SIZE = 200UL
  }

  init {
    require(reorgWindowSize > 0) { "reorgWindowSize must be greater than 0" }
    require(fetchBatchSize > 0UL) { "fetchBatchSize must be greater than 0" }
  }

  private data class TrackedBlock(
    val number: ULong,
    val hash: String,
    val totalDifficulty: BigInteger,
  )

  /** Contiguous canonical blocks in ascending order, the last one being the tracked head. */
  private val trackedBlocks = ArrayDeque<TrackedBlock>()

  /**
   * Brings the tracked chain up to date with the EL canonical head and returns its total difficulty, or null if the EL
   * returned no blocks yet.
   */
  fun update(): ElChainTotalDifficulty? {
    val latestHeader = headerProvider.latestHeader() ?: return head()
    rewindToCanonicalChain(latestHeader)
    extendTo(latestHeader.number)
    return head()
  }

  fun head(): ElChainTotalDifficulty? =
    trackedBlocks.lastOrNull()?.let { ElChainTotalDifficulty(it.number, it.hash, it.totalDifficulty) }

  private fun rewindToCanonicalChain(latestHeader: ElBlockHeader) {
    // The canonical chain may now be shorter than the tracked one
    while (trackedBlocks.isNotEmpty() && trackedBlocks.last().number > latestHeader.number) {
      trackedBlocks.removeLast()
    }
    var reorgDepth = 0
    while (trackedBlocks.isNotEmpty()) {
      val trackedHead = trackedBlocks.last()
      val canonicalHash =
        if (trackedHead.number == latestHeader.number) {
          latestHeader.hash
        } else {
          headerProvider.headersByNumber(trackedHead.number, trackedHead.number).firstOrNull()?.hash
        }
      if (canonicalHash == trackedHead.hash) {
        // Tracked blocks are linked by parent hash, so all of them are canonical too
        if (reorgDepth > 0) {
          log.info(
            "EL reorg detected: rewound total difficulty tracking by {} blocks to common ancestor elBlockNumber={}",
            reorgDepth,
            trackedHead.number,
          )
        }
        return
      }
      trackedBlocks.removeLast()
      reorgDepth++
    }
    if (reorgDepth > 0) {
      log.warn(
        "EL reorg deeper than the {} tracked blocks: re-summing total difficulty from genesis",
        reorgDepth,
      )
    }
  }

  private fun extendTo(targetBlockNumber: ULong) {
    var nextBlockNumber = trackedBlocks.lastOrNull()?.let { it.number + 1UL } ?: 0UL
    while (nextBlockNumber <= targetBlockNumber) {
      val toBlockNumber = minOf(targetBlockNumber, nextBlockNumber + fetchBatchSize - 1UL)
      val requestedCount = toBlockNumber - nextBlockNumber + 1UL
      val headers = headerProvider.headersByNumber(nextBlockNumber, toBlockNumber)
      for (header in headers) {
        if (!append(header, expectedBlockNumber = nextBlockNumber)) {
          return
        }
        nextBlockNumber++
      }
      if (headers.size.toULong() < requestedCount) {
        // The EL does not have all requested blocks (anymore); retry on the next update
        return
      }
    }
  }

  /** Returns false if [header] does not extend the tracked chain, e.g. the canonical chain changed meanwhile. */
  private fun append(
    header: ElBlockHeader,
    expectedBlockNumber: ULong,
  ): Boolean {
    if (header.number != expectedBlockNumber) {
      log.warn("EL returned elBlockNumber={} while elBlockNumber={} was expected", header.number, expectedBlockNumber)
      return false
    }
    val parent = trackedBlocks.lastOrNull()
    if (parent != null && header.parentHash != parent.hash) {
      log.debug(
        "EL canonical chain changed while syncing total difficulty: elBlockNumber={} parentHash={} != {}",
        header.number,
        header.parentHash,
        parent.hash,
      )
      return false
    }
    val totalDifficulty = (parent?.totalDifficulty ?: BigInteger.ZERO) + header.difficulty
    trackedBlocks.addLast(TrackedBlock(header.number, header.hash, totalDifficulty))
    while (trackedBlocks.size > reorgWindowSize) {
      trackedBlocks.removeFirst()
    }
    return true
  }
}
