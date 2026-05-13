/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import maru.consensus.ValidatorProvider
import maru.consensus.qbft.toAddress
import maru.core.ext.DataGenerators
import org.assertj.core.api.Assertions.assertThat
import org.mockito.Mockito
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture.completedFuture
import kotlin.test.Test

class QbftValidatorProviderAdaptorTest {
  @Test
  fun `can get validators after block`() {
    val validatorProvider = Mockito.mock(ValidatorProvider::class.java)
    val validators1 = DataGenerators.randomValidators()
    val validators2 = DataGenerators.randomValidators()
    val header1 = QbftBlockHeaderAdapter(DataGenerators.randomBeaconBlockHeader(10U))
    val header2 = QbftBlockHeaderAdapter(DataGenerators.randomBeaconBlockHeader(11U))
    whenever(
      validatorProvider.getValidatorsAfterBlock(header1.beaconBlockHeader.number),
    ).thenReturn(completedFuture(validators1))
    whenever(
      validatorProvider.getValidatorsAfterBlock(header2.beaconBlockHeader.number),
    ).thenReturn(completedFuture(validators2))

    val qbftValidatorProviderAdapter = QbftValidatorProviderAdapter(validatorProvider)

    assertThat(
      qbftValidatorProviderAdapter.getValidatorsAfterBlock(header1),
    ).containsAll(validators1.map { it.toAddress() })
    assertThat(
      qbftValidatorProviderAdapter.getValidatorsAfterBlock(header2),
    ).containsAll(validators2.map { it.toAddress() })
  }

  @Test
  fun `can get validators for block`() {
    val validatorProvider = Mockito.mock(ValidatorProvider::class.java)
    val validators1 = DataGenerators.randomValidators()
    val validators2 = DataGenerators.randomValidators()
    val header1 = QbftBlockHeaderAdapter(DataGenerators.randomBeaconBlockHeader(10U))
    val header2 = QbftBlockHeaderAdapter(DataGenerators.randomBeaconBlockHeader(11U))
    whenever(
      validatorProvider.getValidatorsForBlock(header1.beaconBlockHeader.number),
    ).thenReturn(completedFuture(validators1))
    whenever(
      validatorProvider.getValidatorsForBlock(header2.beaconBlockHeader.number),
    ).thenReturn(completedFuture(validators2))

    val qbftValidatorProviderAdapter = QbftValidatorProviderAdapter(validatorProvider)
    assertThat(
      qbftValidatorProviderAdapter.getValidatorsForBlock(header1),
    ).containsAll(validators1.map { it.toAddress() })
    assertThat(
      qbftValidatorProviderAdapter.getValidatorsForBlock(header2),
    ).containsAll(validators2.map { it.toAddress() })
  }
}
