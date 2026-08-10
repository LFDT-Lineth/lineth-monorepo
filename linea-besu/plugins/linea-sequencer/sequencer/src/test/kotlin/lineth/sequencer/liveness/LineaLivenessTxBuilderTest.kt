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
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Wei
import org.junit.jupiter.api.Test
import org.web3j.crypto.ECKeyPair
import org.web3j.crypto.Keys
import org.web3j.utils.Numeric
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.io.IOException
import java.math.BigInteger
import java.util.Optional
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

class LineaLivenessTxBuilderTest {
  @Test
  fun `signs the web3j digest and produces transaction from signer`() {
    val keyPair = ECKeyPair.create(BigInteger.ONE)
    val signedDigest = AtomicReference<ByteArray>()
    val signer =
      object : Signer<Secp256k1Signature> {
        override fun publicKey(): ByteArray =
          Numeric.toBytesPadded(keyPair.publicKey, PUBLIC_KEY_SIZE)

        override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> {
          signedDigest.set(bytes.clone())
          val signature = keyPair.sign(bytes)
          return SafeFuture.completedFuture(
            Secp256k1Signature(signature.r, signature.s),
          )
        }
      }
    val config = livenessConfig()

    val transaction =
      LineaLivenessTxBuilder(
        config,
        { Optional.of(Wei.ONE) },
        BigInteger.valueOf(1337),
        signer,
      )
        .buildUptimeTransaction(true, 1234, 0)

    assertThat(signedDigest.get()).hasSize(32)
    assertThat(transaction.sender)
      .isEqualTo(
        Address.fromHexString(
          Numeric.prependHexPrefix(Keys.getAddress(keyPair.publicKey)),
        ),
      )
  }

  @Test
  fun `propagates signer failure without producing transaction`() {
    val signer =
      object : Signer<Secp256k1Signature> {
        override fun publicKey(): ByteArray =
          Numeric.toBytesPadded(
            ECKeyPair.create(BigInteger.ONE).publicKey,
            PUBLIC_KEY_SIZE,
          )

        override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> =
          SafeFuture.failedFuture(IllegalStateException("KMS unavailable"))
      }
    val builder =
      LineaLivenessTxBuilder(
        livenessConfig(),
        { Optional.of(Wei.ONE) },
        BigInteger.valueOf(1337),
        signer,
      )

    assertThatThrownBy { builder.buildUptimeTransaction(true, 1234, 0) }
      .isInstanceOf(IOException::class.java)
      .hasMessageContaining("Failed to sign liveness transaction")
      .hasRootCauseMessage("KMS unavailable")
  }

  @Test
  fun `restores interrupt flag when signing is interrupted`() {
    val signingStarted = CountDownLatch(1)
    val signer =
      object : Signer<Secp256k1Signature> {
        override fun publicKey(): ByteArray =
          Numeric.toBytesPadded(
            ECKeyPair.create(BigInteger.ONE).publicKey,
            PUBLIC_KEY_SIZE,
          )

        override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> {
          signingStarted.countDown()
          return SafeFuture()
        }
      }
    val builder =
      LineaLivenessTxBuilder(
        livenessConfig(),
        { Optional.of(Wei.ONE) },
        BigInteger.valueOf(1337),
        signer,
      )
    val failure = AtomicReference<Throwable>()
    val interrupted = AtomicBoolean()
    val signingThread =
      Thread.ofPlatform().unstarted {
        try {
          builder.buildUptimeTransaction(true, 1234, 0)
        } catch (error: Throwable) {
          failure.set(error)
          interrupted.set(Thread.currentThread().isInterrupted)
        }
      }

    signingThread.start()
    assertThat(signingStarted.await(5, TimeUnit.SECONDS)).isTrue()
    signingThread.interrupt()
    signingThread.join(5_000)

    assertThat(signingThread.isAlive).isFalse()
    assertThat(failure.get())
      .isInstanceOf(IOException::class.java)
      .hasRootCauseInstanceOf(InterruptedException::class.java)
    assertThat(interrupted).isTrue()
  }

  private fun livenessConfig(): LineaLivenessServiceConfiguration =
    LineaLivenessServiceConfiguration.builder()
      .contractAddress(CONTRACT_ADDRESS)
      .gasLimit(100_000)
      .gasPrice(7)
      .build()

  private companion object {
    const val CONTRACT_ADDRESS = "0x1111111111111111111111111111111111111111"
    const val PUBLIC_KEY_SIZE = 64
  }
}
