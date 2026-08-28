/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.sequencer.liveness

import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import lineth.config.LineaLivenessServiceConfiguration
import lineth.config.LineaLivenessServiceConfiguration.SignerType
import lineth.signing.SignerService
import org.hyperledger.besu.plugin.ServiceManager
import org.web3j.crypto.Keys
import org.web3j.utils.Numeric
import java.math.BigInteger

/** Resolves and validates the signer configured for liveness transactions. */
class LivenessSignerResolver(private val serviceManager: ServiceManager) {
  fun resolve(
    configuration: LineaLivenessServiceConfiguration,
  ): Signer<Secp256k1Signature> {
    val signer =
      when (configuration.signerType()) {
        SignerType.WEB3SIGNER -> Web3SignerDigestSigner(configuration)
        SignerType.CUSTOM ->
          serviceManager
            .getService(SignerService::class.java)
            .orElseThrow {
              IllegalStateException(
                "No SignerService is registered for CUSTOM liveness signing",
              )
            }
      }

    validateSigner(configuration, signer)
    return signer
  }

  private fun validateSigner(
    configuration: LineaLivenessServiceConfiguration,
    signer: Signer<Secp256k1Signature>,
  ) {
    val publicKey = signer.publicKey()
    require(publicKey.size == PUBLIC_KEY_SIZE) {
      "${configuration.signerType()} signer public key must be 64-byte secp256k1 coordinates " +
        "(x || y), got ${publicKey.size} bytes"
    }

    val derivedAddress = Numeric.prependHexPrefix(Keys.getAddress(BigInteger(1, publicKey)))
    require(derivedAddress.equals(configuration.signerAddress(), ignoreCase = true)) {
      val signerIdentifier = configuration.signerKeyId()?.let { " '$it'" }.orEmpty()
      "Configured liveness signer address does not match " +
        "${configuration.signerType()} signer$signerIdentifier"
    }
  }

  private companion object {
    const val PUBLIC_KEY_SIZE = 64
  }
}
