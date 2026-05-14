/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.core.SealedBeaconBlock
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlock
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockHeader

/**
 * Adapter class to convert a SealedBeaconBlock to a QBFT block
 */
class QbftSealedBlockAdapter(
  val sealedBeaconBlock: SealedBeaconBlock,
) : QbftBlock {
  val qbftHeader: QbftBlockHeader = QbftBlockHeaderAdapter(sealedBeaconBlock.beaconBlock.beaconBlockHeader)

  override fun getHeader(): QbftBlockHeader = qbftHeader

  override fun isEmpty(): Boolean =
    sealedBeaconBlock.beaconBlock.beaconBlockBody.executionPayload.transactions
      .isEmpty()

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (other !is QbftSealedBlockAdapter) return false

    if (sealedBeaconBlock != other.sealedBeaconBlock) return false
    if (qbftHeader != other.qbftHeader) return false

    return true
  }

  override fun hashCode(): Int {
    var result = sealedBeaconBlock.hashCode()
    result = 31 * result + qbftHeader.hashCode()
    return result
  }
}
