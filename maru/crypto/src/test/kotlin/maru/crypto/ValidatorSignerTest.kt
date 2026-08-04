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
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.hyperledger.besu.cryptoservices.KeyPairSecurityModule
import org.hyperledger.besu.cryptoservices.NodeKey
import org.hyperledger.besu.ethereum.core.Util
import org.hyperledger.besu.plugin.services.securitymodule.SecurityModuleException
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.TimeUnit

class ValidatorSignerTest {
  private val privateKey = ByteArray(32).also { it[it.lastIndex] = 1 }
  private val digest = Bytes32.fromHexString("0x01").toArray()

  @Test
  fun `local signer is wire compatible with key pair node key`() {
    val signer = LocalValidatorSigner(privateKey)
    val adaptedNodeKey = signer.toNodeKey()
    val privateKeyObject = SecpCrypto.signatureAlgorithm.createPrivateKey(Bytes32.wrap(privateKey))
    val keyPair = SecpCrypto.signatureAlgorithm.createKeyPair(privateKeyObject)
    val originalNodeKey = NodeKey(KeyPairSecurityModule(keyPair))

    assertThat(adaptedNodeKey.publicKey).isEqualTo(originalNodeKey.publicKey)
    assertThat(Util.publicKeyToAddress(adaptedNodeKey.publicKey))
      .isEqualTo(Util.publicKeyToAddress(originalNodeKey.publicKey))
    assertThat(adaptedNodeKey.sign(Bytes32.wrap(digest)).encodedBytes())
      .isEqualTo(originalNodeKey.sign(Bytes32.wrap(digest)).encodedBytes())
  }

  @Test
  fun `adapter forwards exactly the supplied digest`() {
    val localSigner = LocalValidatorSigner(privateKey)
    var received: ByteArray? = null
    val recordingSigner =
      FakeValidatorSigner(localSigner.publicKey()) { bytes ->
        received = bytes.copyOf()
        localSigner.sign(bytes)
      }

    recordingSigner.toNodeKey().sign(Bytes32.wrap(digest))

    assertThat(received).containsExactly(*digest)
  }

  @Test
  fun `adapter rejects malformed public keys`() {
    val signer =
      FakeValidatorSigner(ByteArray(63)) {
        error("must not be called")
      }

    assertThatThrownBy { SignerSecurityModule(signer) }
      .isInstanceOf(SecurityModuleException::class.java)
      .hasMessageContaining("must be 64 bytes")
  }

  @Test
  fun `adapter rejects invalid secp256k1 points`() {
    val signer =
      FakeValidatorSigner(ByteArray(SignerSecurityModule.PUBLIC_KEY_SIZE)) {
        error("must not be called")
      }

    assertThatThrownBy { SignerSecurityModule(signer) }
      .isInstanceOf(SecurityModuleException::class.java)
  }

  @Test
  fun `adapter wraps provider failures`() {
    val localSigner = LocalValidatorSigner(privateKey)
    val providerError = IllegalStateException("provider unavailable")
    val signer =
      FakeValidatorSigner(localSigner.publicKey()) {
        SafeFuture.failedFuture(providerError)
      }

    assertThatThrownBy { SignerSecurityModule(signer).sign(Bytes32.wrap(digest)) }
      .isInstanceOf(SecurityModuleException::class.java)
      .hasMessageContaining("Validator signing failed")
      .hasCause(providerError)
  }

  @Test
  fun `adapter preserves interruption while waiting for a signature`() {
    val localSigner = LocalValidatorSigner(privateKey)
    val signingStarted = SafeFuture<Unit>()
    val pendingSignature = SafeFuture<Secp256k1Signature>()
    val signer =
      FakeValidatorSigner(localSigner.publicKey()) {
        signingStarted.complete(Unit)
        pendingSignature
      }
    var signingError: Throwable? = null
    var interruptPreserved = false
    val thread =
      Thread {
        try {
          SignerSecurityModule(signer).sign(Bytes32.wrap(digest))
        } catch (caught: Throwable) {
          signingError = caught
          interruptPreserved = Thread.currentThread().isInterrupted
        }
      }.apply { isDaemon = true }

    thread.start()
    signingStarted.get(5, TimeUnit.SECONDS)
    thread.interrupt()
    thread.join(TimeUnit.SECONDS.toMillis(5))

    assertThat(thread.isAlive).isFalse()
    assertThat(signingError).isInstanceOf(SecurityModuleException::class.java)
    assertThat(interruptPreserved).isTrue()
  }

  private class FakeValidatorSigner(
    private val publicKey: ByteArray,
    private val signAction: (ByteArray) -> SafeFuture<Secp256k1Signature>,
  ) : Signer<Secp256k1Signature> {
    override fun publicKey(): ByteArray = publicKey

    override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> = signAction(bytes)
  }
}
