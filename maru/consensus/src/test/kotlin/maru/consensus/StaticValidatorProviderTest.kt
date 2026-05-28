/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import linea.kotlin.decodeHex
import maru.core.Validator
import maru.core.ext.DataGenerators
import org.assertj.core.api.Assertions.assertThat
import kotlin.test.Test

class StaticValidatorProviderTest {
  private val validators = DataGenerators.randomValidators()
  private val staticValidatorProvider = StaticValidatorProvider(validators)

  @Test
  fun `can get validators at after block`() {
    assertThat(staticValidatorProvider.getValidatorsAfterBlock(0U).get()).isEqualTo(validators)
    assertThat(staticValidatorProvider.getValidatorsAfterBlock(1U).get()).isEqualTo(validators)
  }

  @Test
  fun `can get validators for block`() {
    assertThat(staticValidatorProvider.getValidatorsForBlock(0U).get()).isEqualTo(validators)
    assertThat(staticValidatorProvider.getValidatorsForBlock(1U).get()).isEqualTo(validators)
  }

  @Test
  fun `validators are sorted by address`() {
    val validator1 = Validator("0x0000000000000000000000000000000000000001".decodeHex())
    val validator2 = Validator("0x0000000000000000000000000000000000000002".decodeHex())
    val validator3 = Validator("0x0000000000000000000000000000000000000003".decodeHex())

    val unsortedValidators = setOf(validator3, validator1, validator2)
    val provider = StaticValidatorProvider(unsortedValidators)

    assertThat(provider.getValidatorsForBlock(0U).get()).containsExactly(validator1, validator2, validator3)
    assertThat(provider.getValidatorsAfterBlock(0U).get()).containsExactly(validator1, validator2, validator3)
  }
}
