/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.crypto

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import linea.kotlin.toBytes32
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger

class SigningTest {
  @Test
  fun `ulong signer forwards the digest and returns r-s encoding`() {
    val value = 42uL
    val signature = Secp256k1Signature(BigInteger.ONE, BigInteger.TWO)
    val signer = RecordingSigner(signature)

    val encodedSignature = Signing.ULongSigner(signer).sign(value)

    assertThat(signer.received).containsExactly(*value.toBytes32())
    assertThat(encodedSignature).containsExactly(*signature.toRSBytes())
  }

  private class RecordingSigner(
    private val signature: Secp256k1Signature,
  ) : Signer<Secp256k1Signature> {
    var received: ByteArray? = null
      private set

    override fun publicKey(): ByteArray = ByteArray(64)

    override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> {
      received = bytes.copyOf()
      return SafeFuture.completedFuture(signature)
    }
  }
}
