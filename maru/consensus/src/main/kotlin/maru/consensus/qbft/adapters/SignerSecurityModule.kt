/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft.adapters

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import maru.crypto.SecpCrypto
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.crypto.ECPointUtil
import org.hyperledger.besu.cryptoservices.NodeKey
import org.hyperledger.besu.plugin.services.securitymodule.SecurityModule
import org.hyperledger.besu.plugin.services.securitymodule.SecurityModuleException
import org.hyperledger.besu.plugin.services.securitymodule.data.PublicKey
import org.hyperledger.besu.plugin.services.securitymodule.data.Signature
import java.util.concurrent.CompletionException
import java.util.concurrent.ExecutionException

internal class SignerSecurityModule(
  private val signer: Signer<Secp256k1Signature>,
) : SecurityModule {
  private val publicKey: PublicKey

  init {
    val encodedPublicKey = signer.publicKey().copyOf()
    if (encodedPublicKey.size != PUBLIC_KEY_SIZE) {
      throw SecurityModuleException(
        "Validator signer public key must be $PUBLIC_KEY_SIZE bytes, got ${encodedPublicKey.size}",
      )
    }
    try {
      val secpPublicKey = SecpCrypto.signatureAlgorithm.createPublicKey(Bytes.wrap(encodedPublicKey))
      if (!SecpCrypto.signatureAlgorithm.isValidPublicKey(secpPublicKey)) {
        throw SecurityModuleException("Validator signer public key is not a valid secp256k1 point")
      }
      val point =
        ECPointUtil.fromBouncyCastleECPoint(
          SecpCrypto.signatureAlgorithm.publicKeyAsEcPoint(secpPublicKey),
        )
      publicKey = PublicKey { point }
    } catch (error: SecurityModuleException) {
      throw error
    } catch (error: Exception) {
      throw SecurityModuleException("Invalid validator signer public key", error)
    }
  }

  override fun sign(dataHash: Bytes32): Signature {
    val signature =
      try {
        signer.sign(dataHash.toArray()).get()
      } catch (error: InterruptedException) {
        Thread.currentThread().interrupt()
        throw SecurityModuleException("Interrupted while waiting for validator signature", error)
      } catch (error: Exception) {
        throw SecurityModuleException("Validator signing failed", unwrapCompletionError(error))
      }
    return object : Signature {
      override fun getR() = signature.r

      override fun getS() = signature.s
    }
  }

  override fun getPublicKey(): PublicKey = publicKey

  override fun calculateECDHKeyAgreement(partyKey: PublicKey): Bytes32 =
    throw SecurityModuleException("ECDH is not supported by the consensus-only validator signer")

  override fun calculateECDHKeyAgreementCompressed(partyKey: PublicKey): Bytes =
    throw SecurityModuleException("ECDH is not supported by the consensus-only validator signer")

  companion object {
    const val PUBLIC_KEY_SIZE = 64

    private fun unwrapCompletionError(error: Throwable): Throwable {
      var cause = error
      while ((cause is ExecutionException || cause is CompletionException) && cause.cause != null) {
        cause = cause.cause!!
      }
      return cause
    }
  }
}

internal fun Signer<Secp256k1Signature>.toNodeKey(): NodeKey = NodeKey(SignerSecurityModule(this))
