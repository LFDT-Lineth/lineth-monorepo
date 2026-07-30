/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.liveness;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.io.IOException;
import java.math.BigInteger;
import java.util.Optional;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicReference;
import linea.crypto.Secp256k1Signature;
import linea.crypto.Signer;
import net.consensys.linea.config.LineaLivenessServiceConfiguration;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.ECKeyPair;
import org.web3j.crypto.Keys;
import org.web3j.utils.Numeric;
import tech.pegasys.teku.infrastructure.async.SafeFuture;

class LineaLivenessTxBuilderTest {
  @Test
  void signsTheWeb3jDigestAndProducesTransactionFromSigner() throws Exception {
    ECKeyPair keyPair = ECKeyPair.create(BigInteger.ONE);
    AtomicReference<byte[]> signedDigest = new AtomicReference<>();
    Signer<Secp256k1Signature> signer =
        new Signer<>() {
          @Override
          public byte[] publicKey() {
            return Numeric.toBytesPadded(keyPair.getPublicKey(), 64);
          }

          @Override
          public SafeFuture<Secp256k1Signature> sign(final byte[] bytes) {
            signedDigest.set(bytes.clone());
            var signature = keyPair.sign(bytes);
            return SafeFuture.completedFuture(new Secp256k1Signature(signature.r, signature.s));
          }
        };
    LineaLivenessServiceConfiguration config =
        LineaLivenessServiceConfiguration.builder()
            .contractAddress("0x1111111111111111111111111111111111111111")
            .gasLimit(100_000)
            .gasPrice(7)
            .build();

    Transaction transaction =
        new LineaLivenessTxBuilder(
                config, () -> Optional.of(Wei.ONE), BigInteger.valueOf(1337), signer)
            .buildUptimeTransaction(true, 1234, 0);

    assertThat(signedDigest.get()).hasSize(32);
    assertThat(transaction.getSender())
        .isEqualTo(
            Address.fromHexString(
                Numeric.prependHexPrefix(Keys.getAddress(keyPair.getPublicKey()))));
  }

  @Test
  void propagatesSignerFailureWithoutProducingTransaction() {
    Signer<Secp256k1Signature> signer =
        new Signer<>() {
          @Override
          public byte[] publicKey() {
            return Numeric.toBytesPadded(ECKeyPair.create(BigInteger.ONE).getPublicKey(), 64);
          }

          @Override
          public SafeFuture<Secp256k1Signature> sign(final byte[] bytes) {
            return SafeFuture.failedFuture(new IllegalStateException("KMS unavailable"));
          }
        };
    LineaLivenessServiceConfiguration config =
        LineaLivenessServiceConfiguration.builder()
            .contractAddress("0x1111111111111111111111111111111111111111")
            .gasLimit(100_000)
            .gasPrice(7)
            .build();
    LineaLivenessTxBuilder builder =
        new LineaLivenessTxBuilder(
            config, () -> Optional.of(Wei.ONE), BigInteger.valueOf(1337), signer);

    assertThatThrownBy(() -> builder.buildUptimeTransaction(true, 1234, 0))
        .isInstanceOf(IOException.class)
        .hasMessageContaining("Failed to sign liveness transaction")
        .hasRootCauseMessage("KMS unavailable");
  }

  @Test
  void restoresInterruptFlagWhenSigningIsInterrupted() throws Exception {
    CountDownLatch signingStarted = new CountDownLatch(1);
    Signer<Secp256k1Signature> signer =
        new Signer<>() {
          @Override
          public byte[] publicKey() {
            return Numeric.toBytesPadded(ECKeyPair.create(BigInteger.ONE).getPublicKey(), 64);
          }

          @Override
          public SafeFuture<Secp256k1Signature> sign(final byte[] bytes) {
            signingStarted.countDown();
            return new SafeFuture<>();
          }
        };
    LineaLivenessServiceConfiguration config =
        LineaLivenessServiceConfiguration.builder()
            .contractAddress("0x1111111111111111111111111111111111111111")
            .gasLimit(100_000)
            .gasPrice(7)
            .build();
    LineaLivenessTxBuilder builder =
        new LineaLivenessTxBuilder(
            config, () -> Optional.of(Wei.ONE), BigInteger.valueOf(1337), signer);
    AtomicReference<Throwable> failure = new AtomicReference<>();
    AtomicBoolean interrupted = new AtomicBoolean();
    Thread signingThread =
        Thread.ofPlatform()
            .unstarted(
                () -> {
                  try {
                    builder.buildUptimeTransaction(true, 1234, 0);
                  } catch (Throwable error) {
                    failure.set(error);
                    interrupted.set(Thread.currentThread().isInterrupted());
                  }
                });

    signingThread.start();
    assertThat(signingStarted.await(5, TimeUnit.SECONDS)).isTrue();
    signingThread.interrupt();
    signingThread.join(5_000);

    assertThat(signingThread.isAlive()).isFalse();
    assertThat(failure.get())
        .isInstanceOf(IOException.class)
        .hasRootCauseInstanceOf(InterruptedException.class);
    assertThat(interrupted).isTrue();
  }
}
