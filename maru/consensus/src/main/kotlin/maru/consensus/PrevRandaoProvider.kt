/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus

import maru.core.Hasher
import maru.core.Signer
import maru.extensions.encodeHex
import maru.extensions.xor
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger

fun interface PrevRandaoProvider<T> {
  fun calculateNextPrevRandao(
    signee: T,
    prevRandao: ByteArray,
  ): ByteArray
}

class PrevRandaoProviderImpl<T>(
  private val signer: Signer<T>,
  private val hasher: Hasher,
) : PrevRandaoProvider<T> {
  private val log: Logger = LogManager.getLogger(this.javaClass)

  override fun calculateNextPrevRandao(
    signee: T,
    prevRandao: ByteArray,
  ): ByteArray {
    val signature = signer.sign(signee)
    val signatureHash = hasher.hash(signature)
    val nextPrevRandao = prevRandao.xor(signatureHash)

    log.debug(
      "calculateNextPrevRandao: signee=$signee prevRandao=${prevRandao.encodeHex()} " +
        "signature=${signature.encodeHex()} signatureHash=${signatureHash.encodeHex()} " +
        "nextPrevRandao=${nextPrevRandao.encodeHex()}",
    )
    return nextPrevRandao
  }
}
