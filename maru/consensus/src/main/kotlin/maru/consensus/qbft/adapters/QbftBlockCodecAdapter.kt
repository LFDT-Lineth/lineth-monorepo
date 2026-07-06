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
import maru.serialization.rlp.RLPSerDe
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlock
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockCodec
import org.hyperledger.besu.ethereum.rlp.RLPInput
import org.hyperledger.besu.ethereum.rlp.RLPOutput

/**
 * Adapter for QBFT block codec, this provides a way to serialize QBFT blocks.
 *
 * [beaconBlockSerializer] must be the node's fork-aware serializer (from `ForkAwareBlockHashing`) so that
 * blocks decoded from QBFT Proposal/RoundChange messages carry the fork-appropriate chain-identity hash
 * function. Otherwise a non-proposer validator would recover a round-inclusive identity and persist the
 * block under a different key than the proposer from QBFT_PHASE1 onward.
 */
class QbftBlockCodecAdapter(
  private val beaconBlockSerializer: RLPSerDe<BeaconBlock>,
) : QbftBlockCodec {
  override fun readFrom(rlpInput: RLPInput): QbftBlock = QbftBlockAdapter(beaconBlockSerializer.readFrom(rlpInput))

  override fun writeTo(
    qbftBlock: QbftBlock,
    rlpOutput: RLPOutput,
  ) {
    beaconBlockSerializer.writeTo(qbftBlock.toBeaconBlock(), rlpOutput)
  }
}
