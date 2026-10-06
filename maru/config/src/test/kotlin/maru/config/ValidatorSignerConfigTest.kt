/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.config

import linea.kotlin.decodeHex
import maru.config.MaruConfigLoader.parseConfig
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class ValidatorSignerConfigTest {
  @Test
  fun `signer defaults to local`() {
    val config =
      parseConfig<QbftOptionsDtoToml>(
        """
        fee-recipient = "0x0000000000000000000000000000000000000001"
        """.trimIndent(),
      )

    assertThat(config.toDomain().validatorSigner).isEqualTo(ValidatorSignerConfig())
  }

  @Test
  fun `custom signer name is parsed`() {
    val config =
      parseConfig<QbftOptionsDtoToml>(
        """
        fee-recipient = "0x0000000000000000000000000000000000000001"
        signer-type = "custom"
        signer-name = "maru-validator"
        """.trimIndent(),
      )

    assertThat(config.toDomain()).isEqualTo(
      QbftConfig(
        feeRecipient = "0x0000000000000000000000000000000000000001".decodeHex(),
        validatorSigner =
        ValidatorSignerConfig(
          type = ValidatorSignerType.CUSTOM,
          name = "maru-validator",
        ),
      ),
    )
  }

  @Test
  fun `custom signer requires a non-blank name`() {
    assertThatThrownBy {
      ValidatorSignerConfig.fromConfig("custom", " ")
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("signer-name must be set and non-blank")
  }

  @Test
  fun `local signer rejects a name`() {
    assertThatThrownBy {
      ValidatorSignerConfig.fromConfig("local", "unused")
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("signer-name must not be set")
  }

  @Test
  fun `unknown signer type is rejected`() {
    assertThatThrownBy {
      ValidatorSignerConfig.fromConfig("aws", null)
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("signer-type must be either local or custom")
  }
}
