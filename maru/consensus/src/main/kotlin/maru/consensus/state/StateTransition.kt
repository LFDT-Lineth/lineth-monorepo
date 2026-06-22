/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.state

import maru.consensus.ValidatorProvider
import maru.core.BeaconBlock
import maru.core.BeaconState
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface StateTransition {
  fun processBlock(block: BeaconBlock): SafeFuture<BeaconState>
}

class StateTransitionImpl(
  private val validatorProvider: ValidatorProvider,
) : StateTransition {
  override fun processBlock(block: BeaconBlock): SafeFuture<BeaconState> {
    val validatorsForBlockFuture = validatorProvider.getValidatorsForBlock(block.beaconBlockHeader.number)
    return validatorsForBlockFuture.thenApply { validatorsForBlock ->
      BeaconState(
        beaconBlockHeader = block.beaconBlockHeader,
        validators = validatorsForBlock,
      )
    }
  }
}
