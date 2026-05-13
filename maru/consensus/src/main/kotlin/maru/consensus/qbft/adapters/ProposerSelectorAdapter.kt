/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.consensus.qbft.toAddress
import maru.database.BeaconChain
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.blockcreation.ProposerSelector
import org.hyperledger.besu.datatypes.Address
import maru.consensus.qbft.ProposerSelector as MaruProposerSelector

class ProposerSelectorAdapter(
  private val beaconChain: BeaconChain,
  private val proposerSelector: MaruProposerSelector,
) : ProposerSelector {
  override fun selectProposerForRound(roundIdentifier: ConsensusRoundIdentifier): Address {
    val prevBlockNumber = roundIdentifier.sequenceNumber - 1
    val parentBeaconState =
      beaconChain.getBeaconState(prevBlockNumber.toULong()) ?: throw IllegalStateException(
        "Parent block not found. parentBlockNumber=$prevBlockNumber",
      )
    return proposerSelector
      .getProposerForBlock(parentBeaconState, roundIdentifier)
      .get()
      .toAddress()
  }
}
