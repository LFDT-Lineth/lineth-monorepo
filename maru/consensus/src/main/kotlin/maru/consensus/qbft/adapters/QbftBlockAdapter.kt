/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.core.BeaconBlock
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlock
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockHeader

/**
 * Adapter class to convert a BeaconBlock to a QBFT block
 */
class QbftBlockAdapter(
  val beaconBlock: BeaconBlock,
) : QbftBlock {
  val qbftHeader: QbftBlockHeader = QbftBlockHeaderAdapter(beaconBlock.beaconBlockHeader)

  override fun getHeader(): QbftBlockHeader = qbftHeader

  override fun isEmpty(): Boolean =
    beaconBlock.beaconBlockBody.executionPayload.transactions
      .isEmpty()

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (other !is QbftBlockAdapter) return false

    if (beaconBlock != other.beaconBlock) return false
    if (qbftHeader != other.qbftHeader) return false

    return true
  }

  override fun hashCode(): Int {
    var result = beaconBlock.hashCode()
    result = 31 * result + qbftHeader.hashCode()
    return result
  }
}
