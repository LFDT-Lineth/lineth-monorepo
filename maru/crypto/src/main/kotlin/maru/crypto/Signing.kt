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
import linea.kotlin.toBytes32
import maru.core.Signer
import linea.crypto.Signer as AsyncSigner

object Signing {
  class ULongSigner(
    private val signer: AsyncSigner<Secp256k1Signature>,
  ) : Signer<ULong> {
    override fun sign(signee: ULong): ByteArray =
      try {
        signer
          .sign(signee.toBytes32())
          .get()
          .toRSBytes()
      } catch (error: InterruptedException) {
        Thread.currentThread().interrupt()
        throw error
      }
  }
}
