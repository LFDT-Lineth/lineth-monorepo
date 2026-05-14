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
import maru.core.BeaconState
import maru.core.ext.DataGenerators
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.mockito.ArgumentMatchers.eq
import org.mockito.Mockito.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture

class StateTransitionImplTest {
  @Test
  fun `processBlock should return ok`() {
    val newBlock = DataGenerators.randomBeaconBlock(10uL)
    val validators = List(3) { DataGenerators.randomValidator() }.toSortedSet()
    val expectedPostState =
      BeaconState(
        beaconBlockHeader = newBlock.beaconBlockHeader,
        validators = validators,
      )

    val validatorProvider = mock<ValidatorProvider>()
    whenever(validatorProvider.getValidatorsForBlock(eq(newBlock.beaconBlockHeader.number.toLong()).toULong()))
      .thenReturn(SafeFuture.completedFuture(validators))

    val stateTransition = StateTransitionImpl(validatorProvider = validatorProvider)

    val result = stateTransition.processBlock(newBlock).get()
    assertThat(result).isEqualTo(expectedPostState)
  }
}
