/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import maru.consensus.PrevRandaoProvider
import maru.consensus.ValidatorProvider
import maru.consensus.state.FinalizationProvider
import maru.database.BeaconChain
import maru.executionlayer.manager.ExecutionLayerManager
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.hyperledger.besu.consensus.common.bft.blockcreation.ProposerSelector
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockCreatorFactory
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockCreator as BesuQbftBlockCreator

class QbftBlockCreatorFactory(
  private val manager: ExecutionLayerManager,
  private val proposerSelector: ProposerSelector,
  private val validatorProvider: ValidatorProvider,
  private val beaconChain: BeaconChain,
  private val finalizationStateProvider: FinalizationProvider,
  private val prevRandaoProvider: PrevRandaoProvider<ULong>,
  private val feeRecipient: ByteArray,
  private val eagerQbftBlockCreatorConfig: EagerQbftBlockCreator.Config,
) : QbftBlockCreatorFactory {
  private val log: Logger = LogManager.getLogger(this.javaClass)
  private var hasCreatedFirstBlockCreator = false

  override fun create(round: Int): BesuQbftBlockCreator {
    val delayedQbftBlockCreator =
      DelayedQbftBlockCreator(
        manager = manager,
        proposerSelector = proposerSelector,
        validatorProvider = validatorProvider,
        beaconChain = beaconChain,
        round = round,
      )
    val blockNumber = beaconChain.getLatestBeaconState().beaconBlockHeader.number + 1u
    val blockCreator = createBlockCreator(round, blockNumber, delayedQbftBlockCreator)
    hasCreatedFirstBlockCreator = true
    return blockCreator
  }

  private fun createBlockCreator(
    round: Int,
    blockNumber: ULong,
    delayedQbftBlockCreator: DelayedQbftBlockCreator,
  ): BesuQbftBlockCreator =
    if (round == 0 && hasCreatedFirstBlockCreator) {
      log.debug("Using delayed block creator: clBlockNumber={}, round={}", blockNumber, round)
      delayedQbftBlockCreator
    } else {
      log.debug("Using eager block creator: clBlockNumber={}, round={} ", blockNumber, round)
      EagerQbftBlockCreator(
        manager = manager,
        delegate = delayedQbftBlockCreator,
        finalizationStateProvider = finalizationStateProvider,
        prevRandaoProvider = prevRandaoProvider,
        feeRecipient = feeRecipient,
        beaconChain = beaconChain,
        config = eagerQbftBlockCreatorConfig,
      )
    }
}
