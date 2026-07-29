/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txselection;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.math.BigInteger;
import java.util.HashMap;
import java.util.Map;
import java.util.Optional;
import linea.crypto.Secp256k1Signature;
import linea.crypto.Signer;
import linea.signing.NamedSignerProviderService;
import net.consensys.linea.config.LineaLivenessServiceConfiguration;
import net.consensys.linea.config.LineaLivenessServiceConfiguration.SignerType;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.BesuService;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.ECKeyPair;
import org.web3j.crypto.Keys;
import org.web3j.utils.Numeric;
import tech.pegasys.teku.infrastructure.async.SafeFuture;

class LineaTransactionSelectorPluginSignerTest {
  private static final String SIGNER_NAME = "liveness";
  private static final byte[] PUBLIC_KEY =
      Numeric.toBytesPadded(ECKeyPair.create(BigInteger.ONE).getPublicKey(), 64);
  private static final String SIGNER_ADDRESS =
      Numeric.prependHexPrefix(Keys.getAddress(new BigInteger(1, PUBLIC_KEY)));

  @Test
  void rejectsMissingCustomSignerProvider() {
    final ServiceManager serviceManager = new FakeServiceManager();

    assertThatThrownBy(
            () ->
                LineaTransactionSelectorPlugin.resolveLivenessSigner(
                    customConfig(SIGNER_ADDRESS), serviceManager))
        .isInstanceOf(IllegalStateException.class)
        .hasMessageContaining("No NamedSignerProviderService")
        .hasMessageContaining(SIGNER_NAME);
  }

  @Test
  void propagatesUnknownSignerNameFromProvider() {
    final ServiceManager serviceManager =
        new FakeServiceManager()
            .withService(
                NamedSignerProviderService.class,
                name -> {
                  throw new IllegalArgumentException("Unknown AWS KMS signer: " + name);
                });

    assertThatThrownBy(
            () ->
                LineaTransactionSelectorPlugin.resolveLivenessSigner(
                    customConfig(SIGNER_ADDRESS), serviceManager))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessage("Unknown AWS KMS signer: " + SIGNER_NAME);
  }

  @Test
  void rejectsSignerAddressMismatch() {
    final ServiceManager serviceManager = serviceManagerReturning(signer(PUBLIC_KEY));

    assertThatThrownBy(
            () ->
                LineaTransactionSelectorPlugin.resolveLivenessSigner(
                    customConfig("0x0000000000000000000000000000000000000000"), serviceManager))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("Configured liveness signer address does not match CUSTOM signer");
  }

  @Test
  void rejectsNonCanonicalPublicKeyEncoding() {
    final ServiceManager serviceManager = serviceManagerReturning(signer(new byte[65]));

    assertThatThrownBy(
            () ->
                LineaTransactionSelectorPlugin.resolveLivenessSigner(
                    customConfig(SIGNER_ADDRESS), serviceManager))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("must be 64-byte secp256k1 coordinates")
        .hasMessageContaining("got 65 bytes");
  }

  @Test
  void resolvesCustomSignerWhenAddressMatchesPublicKey() {
    final Signer<Secp256k1Signature> expectedSigner = signer(PUBLIC_KEY);
    final ServiceManager serviceManager = serviceManagerReturning(expectedSigner);

    assertThat(
            LineaTransactionSelectorPlugin.resolveLivenessSigner(
                customConfig(SIGNER_ADDRESS), serviceManager))
        .isSameAs(expectedSigner);
  }

  private static LineaLivenessServiceConfiguration customConfig(final String address) {
    return LineaLivenessServiceConfiguration.builder()
        .signerType(SignerType.CUSTOM)
        .signerName(SIGNER_NAME)
        .signerAddress(address)
        .build();
  }

  private static ServiceManager serviceManagerReturning(final Signer<Secp256k1Signature> signer) {
    return new FakeServiceManager().withService(NamedSignerProviderService.class, name -> signer);
  }

  private static Signer<Secp256k1Signature> signer(final byte[] publicKey) {
    return new Signer<>() {
      @Override
      public byte[] publicKey() {
        return publicKey.clone();
      }

      @Override
      public SafeFuture<Secp256k1Signature> sign(final byte[] bytes) {
        return SafeFuture.failedFuture(new UnsupportedOperationException());
      }
    };
  }

  private static final class FakeServiceManager implements ServiceManager {
    private final Map<Class<?>, BesuService> services = new HashMap<>();

    <T extends BesuService> FakeServiceManager withService(
        final Class<T> serviceType, final T service) {
      services.put(serviceType, service);
      return this;
    }

    @Override
    public <T extends BesuService> void addService(final Class<T> serviceType, final T service) {
      services.put(serviceType, service);
    }

    @Override
    public <T extends BesuService> Optional<T> getService(final Class<T> serviceType) {
      return Optional.ofNullable(services.get(serviceType)).map(serviceType::cast);
    }
  }
}
