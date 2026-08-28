/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.app

import maru.config.ValidatorSignerConfig
import maru.config.ValidatorSignerType
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class CustomValidatorSignerFactoryTest {
  @Test
  fun `missing custom signer factory reports the logical name`() {
    val config = ValidatorSignerConfig(ValidatorSignerType.CUSTOM, "maru-validator")

    assertThatThrownBy {
      MissingCustomValidatorSignerFactory.create(config)
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("maru-validator")
  }
}
