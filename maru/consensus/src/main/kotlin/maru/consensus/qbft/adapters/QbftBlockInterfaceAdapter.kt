/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.consensus.state.StateTransition
import maru.core.BeaconBlock
import maru.core.EMPTY_HASH
import maru.core.HashUtil
import maru.core.Validator
import maru.serialization.rlp.stateRoot
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlock
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockInterface
import org.hyperledger.besu.datatypes.Address

/**
 * Adapter class for QBFT block interface, this provides a way to replace the round number in a block
 */
class QbftBlockInterfaceAdapter(
  private val stateTransition: StateTransition,
) : QbftBlockInterface {
  override fun replaceRoundForCommitBlock(
    proposalBlock: QbftBlock,
    roundNumber: Int,
  ): QbftBlock {
    val beaconBlockHeader = proposalBlock.header.toBeaconBlockHeader()
    val replacedBeaconBlockHeader =
      beaconBlockHeader.copy(
        round = roundNumber.toUInt(),
      )
    return processBlockWithUpdatedHeader(proposalBlock, replacedBeaconBlockHeader)
  }

  override fun replaceRoundAndProposerForProposalBlock(
    proposalBlock: QbftBlock,
    roundNumber: Int,
    proposer: Address,
  ): QbftBlock {
    val beaconBlockHeader = proposalBlock.header.toBeaconBlockHeader()
    val replacedBeaconBlockHeader =
      beaconBlockHeader.copy(
        proposer = Validator(proposer.bytes.toArrayUnsafe()),
        round = roundNumber.toUInt(),
      )
    return processBlockWithUpdatedHeader(proposalBlock, replacedBeaconBlockHeader)
  }

  private fun processBlockWithUpdatedHeader(
    proposalBlock: QbftBlock,
    updatedHeader: maru.core.BeaconBlockHeader,
  ): QbftBlock {
    val beaconBlockBody = proposalBlock.toBeaconBlock().beaconBlockBody
    val postState = stateTransition.processBlock(BeaconBlock(updatedHeader, beaconBlockBody)).get()
    val stateRootHeader =
      postState.beaconBlockHeader.copy(
        stateRoot = EMPTY_HASH,
      )
    val stateRoot = HashUtil.stateRoot(postState.copy(beaconBlockHeader = stateRootHeader))
    val finalBlockHeader = stateRootHeader.copy(stateRoot = stateRoot)
    return QbftBlockAdapter(BeaconBlock(finalBlockHeader, beaconBlockBody))
  }
}
