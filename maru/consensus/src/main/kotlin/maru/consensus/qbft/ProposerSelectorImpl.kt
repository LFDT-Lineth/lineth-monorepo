/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.core.BeaconState
import maru.core.Validator
import org.apache.logging.log4j.LogManager
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.blockcreation.BftProposerSelector
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.collections.map

fun interface ProposerSelector {
  fun getProposerForBlock(
    parentBeaconState: BeaconState,
    roundIdentifier: ConsensusRoundIdentifier,
  ): SafeFuture<Validator>
}

object ProposerSelectorImpl : ProposerSelector {
  private val log = LogManager.getLogger(this.javaClass)

  override fun getProposerForBlock(
    parentBeaconState: BeaconState,
    roundIdentifier: ConsensusRoundIdentifier,
  ): SafeFuture<Validator> {
    log.trace("Get proposer for {}", roundIdentifier)
    val prevBlockProposerAddress = parentBeaconState.beaconBlockHeader.proposer.toAddress()
    val validatorsForRound =
      parentBeaconState.validators.map { it.toAddress() }
    val proposer =
      BftProposerSelector.selectProposerForRound(
        /* roundIdentifier = */
        roundIdentifier,
        /* prevBlockProposer = */
        prevBlockProposerAddress,
        /* validatorsForRound = */
        validatorsForRound,
        /* changeEachBlock = */
        true,
      )
    return SafeFuture.completedFuture(Validator(proposer.bytes.toArray()))
  }
}
