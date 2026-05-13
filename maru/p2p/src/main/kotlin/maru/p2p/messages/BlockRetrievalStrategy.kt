/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.p2p.messages

import maru.core.SealedBeaconBlock
import maru.database.BeaconChain
import maru.serialization.MAX_COMPRESSED_MESSAGE_SIZE
import maru.serialization.rlp.MaruCompressorRLPSerDe
import maru.serialization.rlp.RLPSerDe
import maru.serialization.rlp.RLPSerializers

/**
 * Strategy interface for block retrieval logic.
 */
fun interface BlockRetrievalStrategy {
  fun getBlocks(
    beaconChain: BeaconChain,
    request: BeaconBlocksByRangeRequest,
    maxBlocks: ULong,
  ): List<SealedBeaconBlock>
}

/**
 * Default implementation that retrieves blocks from the beacon chain.
 */
class DefaultBlockRetrievalStrategy : BlockRetrievalStrategy {
  override fun getBlocks(
    beaconChain: BeaconChain,
    request: BeaconBlocksByRangeRequest,
    maxBlocks: ULong,
  ): List<SealedBeaconBlock> =
    beaconChain.getSealedBeaconBlocks(
      startBlockNumber = request.startBlockNumber,
      count = maxBlocks,
    )
}

/**
 * Implementation that retrieves blocks from the beacon chain and ensures their compressed serialized size
 * would not exceed the given size limit
 */
class SizeLimitBlockRetrievalStrategy(
  private val sealedBeaconBlockSerDe: RLPSerDe<SealedBeaconBlock> =
    MaruCompressorRLPSerDe(RLPSerializers.SealedBeaconBlockSerializer),
  private val sizeLimit: Int = MAX_COMPRESSED_MESSAGE_SIZE - 4, // first 4 bytes of message is for length prefix
) : BlockRetrievalStrategy {
  override fun getBlocks(
    beaconChain: BeaconChain,
    request: BeaconBlocksByRangeRequest,
    maxBlocks: ULong,
  ): List<SealedBeaconBlock> {
    val sealedBeaconBlocks = mutableListOf<SealedBeaconBlock>()
    var sumOfSerializedBlockSize = 0
    var blockNumber = request.startBlockNumber
    val blockRange = request.startBlockNumber..request.startBlockNumber + maxBlocks - 1U

    while (sumOfSerializedBlockSize < sizeLimit && blockRange.contains(blockNumber)) {
      val sealedBlock =
        beaconChain.getSealedBeaconBlock(blockNumber)
          ?: throw IllegalStateException("Missing sealed beacon block $blockNumber")

      sumOfSerializedBlockSize += sealedBeaconBlockSerDe.serialize(sealedBlock).size

      if (sumOfSerializedBlockSize <= sizeLimit) {
        sealedBeaconBlocks.add(sealedBlock)
      }

      blockNumber++
    }

    return sealedBeaconBlocks
  }
}
