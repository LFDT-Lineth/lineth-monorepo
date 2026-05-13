/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package testutils

import org.assertj.core.api.Assertions
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.methods.response.EthBlock
import testutils.besu.BesuFactory
import java.math.BigInteger
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

/**
 * Metadata for block comparison containing essential block identity.
 */
data class BlockMetadata(
  val number: BigInteger,
  val hash: String,
)

object Checks {
  fun BesuNode.assertMinedBlocks(blocksMined: Int) {
    await
      .pollDelay(1.seconds.toJavaDuration())
      .timeout(30.seconds.toJavaDuration())
      .untilAsserted {
        val minedBlocks = this.getMinedBlocks(blocksMined)
        assertThat(minedBlocks).hasSize(blocksMined)
      }
  }

  fun BesuNode.getMinedBlocks(blocksMined: Int): List<EthBlock.Block> {
    // Fast-exit: one cheap eth_blockNumber call before issuing N block-fetch requests.
    // This keeps TCP connection usage low when the node is still catching up, since most
    // poll iterations during the await window will return here after a single HTTP round-trip
    // instead of opening N parallel connections each time.
    val currentBlockNum =
      this
        .nodeRequests()
        .eth()
        .ethBlockNumber()
        .send()
        ?.blockNumber ?: BigInteger.ZERO
    if (currentBlockNum < BigInteger.valueOf(blocksMined.toLong())) return emptyList()

    return (1..blocksMined)
      .map {
        this
          .nodeRequests()
          .eth()
          .ethGetBlockByNumber(
            DefaultBlockParameter.valueOf(BigInteger.valueOf(it.toLong())),
            false,
          ).sendAsync()
      }.mapNotNull { it?.get()?.block }
  }

  fun BesuNode.getBlockNumber(): BigInteger =
    this
      .nodeRequests()
      .eth()
      .ethBlockNumber()
      .send()
      .blockNumber

  /**
   * Converts blocks to metadata for comparison.
   * This ensures we're comparing the essential block identity without full content comparison.
   */
  fun blocksToMetadata(blocks: List<EthBlock.Block>): List<BlockMetadata> =
    blocks.map { block ->
      BlockMetadata(number = block.number, hash = block.hash)
    }

  /**
   * Helper method to check that all provided Besu nodes have produced the same blocks.
   * Uses block metadata (number and hash) for comparison.
   */
  fun checkAllNodesHaveSameBlocks(
    expectedBlockCount: Int,
    vararg besuNodes: BesuNode,
    timeout: kotlin.time.Duration = 30.seconds,
  ) {
    await
      .pollDelay(1.seconds.toJavaDuration())
      .timeout(timeout.toJavaDuration())
      .untilAsserted {
        require(besuNodes.isNotEmpty()) { "At least one node must be provided" }

        // Get blocks from the first node as reference
        val referenceBlocks = blocksToMetadata(besuNodes.first().getMinedBlocks(expectedBlockCount))
        assertThat(referenceBlocks)
          .withFailMessage("Reference node should have $expectedBlockCount blocks")
          .hasSize(expectedBlockCount)

        // Compare all other nodes against the reference
        besuNodes.drop(1).forEach { node ->
          val checkedBlocks = blocksToMetadata(node.getMinedBlocks(expectedBlockCount))
          assertThat(checkedBlocks)
            .withFailMessage(
              "Node ${node.name} should have $expectedBlockCount blocks matching reference",
            ).hasSize(expectedBlockCount)
            .isEqualTo(referenceBlocks)
        }
      }
  }

  // Checks that all block times in the list are exactly BesuFactory.MIN_BLOCK_TIME (1 second)
  // First block is skipped, because after startup block time is sometimes floating above 1 second
  fun List<EthBlock.Block>.verifyBlockTime() {
    val timestampsSeconds = this.subList(1, this.size - 1).map { it.timestamp.toLong() }
    timestampsSeconds.reduceIndexed { index, prevTimestamp, timestamp ->
      Assertions.assertThat(prevTimestamp).isLessThan(timestamp)
      val actualBlockTime = timestamp - prevTimestamp
      Assertions
        .assertThat(actualBlockTime)
        .withFailMessage(
          "invalid block time: expected=${BesuFactory.MIN_BLOCK_TIME} actual=$actualBlockTime " +
            "blocks timestamps=$timestampsSeconds",
        ).isEqualTo(BesuFactory.MIN_BLOCK_TIME)
      timestamp
    }
  }

  // Checks that all block times in the list are exactly BesuFactory.MIN_BLOCK_TIME (1 second)
  fun List<EthBlock.Block>.verifyBlockTimeWithAGapOn(gappedBlockNumber: Int) {
    val blocksPreGap = this.subList(1, gappedBlockNumber)
    // Skipping the first block after gap as well, because first block after startup can be missed for unclear reasons
    val blocksPostGap = this.subList(gappedBlockNumber, this.size - 1)
    // Verify block time is consistent before and after the switch
    blocksPreGap.verifyBlockTime()
    blocksPostGap.verifyBlockTime()
  }
}
