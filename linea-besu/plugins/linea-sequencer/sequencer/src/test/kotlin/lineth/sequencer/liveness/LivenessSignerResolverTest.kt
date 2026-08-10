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
import lineth.signing.NamedSignerProviderService
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.hyperledger.besu.plugin.ServiceManager
import org.hyperledger.besu.plugin.services.BesuService
import org.junit.jupiter.api.Test
import org.web3j.crypto.ECKeyPair
import org.web3j.crypto.Keys
import org.web3j.utils.Numeric
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.Optional

class LivenessSignerResolverTest {
  @Test
  fun `rejects missing custom signer provider`() {
    val serviceManager = FakeServiceManager()

    assertThatThrownBy {
      LivenessSignerResolver(serviceManager).resolve(customConfig(SIGNER_ADDRESS))
    }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("No NamedSignerProviderService")
      .hasMessageContaining(SIGNER_NAME)
  }

  @Test
  fun `propagates unknown signer name from provider`() {
    val serviceManager =
      FakeServiceManager()
        .withService(
          NamedSignerProviderService::class.java,
          NamedSignerProviderService { name ->
            throw IllegalArgumentException("Unknown AWS KMS signer: $name")
          },
        )

    assertThatThrownBy {
      LivenessSignerResolver(serviceManager).resolve(customConfig(SIGNER_ADDRESS))
    }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessage("Unknown AWS KMS signer: $SIGNER_NAME")
  }

  @Test
  fun `rejects signer address mismatch`() {
    val serviceManager = serviceManagerReturning(signer(PUBLIC_KEY))

    assertThatThrownBy {
      LivenessSignerResolver(serviceManager)
        .resolve(customConfig("0x0000000000000000000000000000000000000000"))
    }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("Configured liveness signer address does not match CUSTOM signer")
  }

  @Test
  fun `rejects non-canonical public key encoding`() {
    val serviceManager = serviceManagerReturning(signer(ByteArray(65)))

    assertThatThrownBy {
      LivenessSignerResolver(serviceManager).resolve(customConfig(SIGNER_ADDRESS))
    }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("must be 64-byte secp256k1 coordinates")
      .hasMessageContaining("got 65 bytes")
  }

  @Test
  fun `resolves custom signer when address matches public key`() {
    val expectedSigner = signer(PUBLIC_KEY)
    val serviceManager = serviceManagerReturning(expectedSigner)

    assertThat(
      LivenessSignerResolver(serviceManager).resolve(customConfig(SIGNER_ADDRESS)),
    )
      .isSameAs(expectedSigner)
  }

  private fun customConfig(address: String): LineaLivenessServiceConfiguration =
    LineaLivenessServiceConfiguration.builder()
      .signerType(SignerType.CUSTOM)
      .signerName(SIGNER_NAME)
      .signerAddress(address)
      .build()

  private fun serviceManagerReturning(
    signer: Signer<Secp256k1Signature>,
  ): ServiceManager =
    FakeServiceManager()
      .withService(
        NamedSignerProviderService::class.java,
        NamedSignerProviderService { signer },
      )

  private fun signer(publicKey: ByteArray): Signer<Secp256k1Signature> =
    object : Signer<Secp256k1Signature> {
      override fun publicKey(): ByteArray = publicKey.clone()

      override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> =
        SafeFuture.failedFuture(UnsupportedOperationException())
    }

  private class FakeServiceManager : ServiceManager {
    private val services = mutableMapOf<Class<*>, BesuService>()

    fun <T : BesuService> withService(
      serviceType: Class<T>,
      service: T,
    ): FakeServiceManager = apply { services[serviceType] = service }

    override fun <T : BesuService> addService(serviceType: Class<T>, service: T) {
      services[serviceType] = service
    }

    override fun <T : BesuService> getService(serviceType: Class<T>): Optional<T> =
      Optional.ofNullable(services[serviceType]).map(serviceType::cast)
  }

  private companion object {
    const val SIGNER_NAME = "liveness"
    val PUBLIC_KEY: ByteArray =
      Numeric.toBytesPadded(ECKeyPair.create(BigInteger.ONE).publicKey, 64)
    val SIGNER_ADDRESS: String =
      Numeric.prependHexPrefix(Keys.getAddress(BigInteger(1, PUBLIC_KEY)))
  }
}
