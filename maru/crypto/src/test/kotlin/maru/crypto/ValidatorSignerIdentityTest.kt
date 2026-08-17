/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.crypto

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ValidatorSignerIdentityTest {
  @Test
  fun `derives validator identity from signer public key`() {
    val privateKey = ByteArray(32).also { it[it.lastIndex] = 1 }
    val signer = LocalValidatorSigner(privateKey)

    assertThat(signer.toValidator()).isEqualTo(SecpCrypto.privateKeyToValidator(privateKey))
  }
}
