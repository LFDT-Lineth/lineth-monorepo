/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.validation

import maru.consensus.qbft.ProposerSelector
import maru.consensus.state.StateTransition
import maru.core.BeaconBlockHeader
import maru.database.BeaconChain
import maru.executionlayer.manager.ExecutionLayerManager

fun interface BeaconBlockValidatorFactory {
  fun createValidatorForBlock(beaconBlockHeader: BeaconBlockHeader): BlockValidator
}

class BeaconBlockValidatorFactoryImpl(
  val beaconChain: BeaconChain,
  proposerSelector: ProposerSelector,
  stateTransition: StateTransition,
  executionLayerManager: ExecutionLayerManager?,
  val allowEmptyBlocks: Boolean,
) : BeaconBlockValidatorFactory {
  private val stateRootValidator = StateRootValidator(stateTransition)
  private val bodyRootValidator = BodyRootValidator()
  private val executionPayloadValidator =
    if (executionLayerManager != null) ExecutionPayloadValidator(executionLayerManager) else null
  private val emptyBlockValidator = if (!allowEmptyBlocks) EmptyBlockValidator else null

  private val proposerValidator = ProposerValidator(proposerSelector, beaconChain)

  override fun createValidatorForBlock(beaconBlockHeader: BeaconBlockHeader): BlockValidator {
    val parentBeaconBlockNumber = beaconBlockHeader.number - 1UL
    val parentBlock = beaconChain.getSealedBeaconBlock(parentBeaconBlockNumber)
    if (parentBlock == null) {
      throw IllegalArgumentException("Expected block for header number=$parentBeaconBlockNumber isn't found!")
    } else {
      val parentHeader = parentBlock.beaconBlock.beaconBlockHeader
      return CompositeBlockValidator(
        blockValidators = listOfNotNull(
          stateRootValidator,
          BlockNumberValidator(parentHeader),
          ExecutionPayloadBlockNumberValidator(parentBlock.beaconBlock.beaconBlockBody.executionPayload),
          TimestampValidator(parentHeader),
          proposerValidator,
          ParentRootValidator(parentHeader),
          bodyRootValidator,
          executionPayloadValidator,
          emptyBlockValidator,
        ),
      )
    }
  }
}
