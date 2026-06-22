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
import org.hyperledger.besu.consensus.common.bft.BftHelpers
import org.hyperledger.besu.consensus.common.bft.BlockTimer
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.RoundTimer
import org.hyperledger.besu.consensus.common.bft.blockcreation.ProposerSelector
import org.hyperledger.besu.consensus.common.bft.network.ValidatorMulticaster
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockCreatorFactory
import org.hyperledger.besu.consensus.qbft.core.types.QbftFinalState
import org.hyperledger.besu.cryptoservices.NodeKey
import org.hyperledger.besu.datatypes.Address
import java.time.Clock

class QbftFinalStateAdapter(
  private val localAddress: Address,
  private val nodeKey: NodeKey,
  private val proposerSelector: ProposerSelector,
  private val validatorMulticaster: ValidatorMulticaster,
  private val roundTimer: RoundTimer,
  private val blockTimer: BlockTimer,
  private val blockCreatorFactory: QbftBlockCreatorFactory,
  private val clock: Clock,
  private val beaconChain: BeaconChain,
) : QbftFinalState {
  override fun getValidatorMulticaster(): ValidatorMulticaster = validatorMulticaster

  override fun getNodeKey(): NodeKey = nodeKey

  override fun getRoundTimer(): RoundTimer = roundTimer

  override fun isLocalNodeValidator(): Boolean = getValidators().contains(localAddress)

  override fun getValidators(): Collection<Address> =
    beaconChain.getLatestBeaconState().validators.map { it.toAddress() }

  override fun getLocalAddress(): Address = localAddress

  override fun getClock(): Clock = clock

  override fun getBlockCreatorFactory(): QbftBlockCreatorFactory = blockCreatorFactory

  override fun getQuorum(): Int = BftHelpers.calculateRequiredValidatorQuorum(getValidators().size)

  override fun getBlockTimer(): BlockTimer = blockTimer

  override fun isLocalNodeProposerForRound(roundIdentifier: ConsensusRoundIdentifier): Boolean =
    proposerSelector.selectProposerForRound(roundIdentifier) == localAddress
}
