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
import maru.crypto.LocalValidatorSigner
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class ValidatorSignerFactoryTest {
  private val privateKey = ByteArray(32).also { it[it.lastIndex] = 1 }

  @Test
  fun `default factory rejects a custom signer with its logical name`() {
    assertThatThrownBy {
      DefaultValidatorSignerFactory.create(
        ValidatorSignerConfig(ValidatorSignerType.CUSTOM, "maru-validator"),
      )
    }.isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("maru-validator")
  }

  @Test
  fun `managed signer closes its resource once`() {
    var closeCalls = 0
    val localSigner = LocalValidatorSigner(privateKey)
    val managedSigner = ManagedValidatorSigner(localSigner) { closeCalls++ }

    managedSigner.close()
    managedSigner.close()

    assertThat(closeCalls).isEqualTo(1)
  }
}
