/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.serialization.rlp.RLPSerializers.BeaconBlockSerializer
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlock
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockCodec
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

/**
 * Adapter for QBFT block codec, this provides a way to serialize QBFT blocks
 */
object QbftBlockCodecAdapter : QbftBlockCodec {
  override fun readFrom(rlpInput: RLPInput): QbftBlock = QbftBlockAdapter(BeaconBlockSerializer.readFrom(rlpInput))

  override fun writeTo(
    qbftBlock: QbftBlock,
    rlpOutput: RLPOutput,
  ) {
    qbftBlock.toBeaconBlock().let {
      BeaconBlockSerializer.writeTo(it, rlpOutput)
    }
  }
}
