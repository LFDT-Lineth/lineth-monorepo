/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.liveness;

import static com.github.tomakehurst.wiremock.client.WireMock.aResponse;
import static com.github.tomakehurst.wiremock.client.WireMock.equalToJson;
import static com.github.tomakehurst.wiremock.client.WireMock.get;
import static com.github.tomakehurst.wiremock.client.WireMock.getRequestedFor;
import static com.github.tomakehurst.wiremock.client.WireMock.post;
import static com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor;
import static com.github.tomakehurst.wiremock.client.WireMock.stubFor;
import static com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo;
import static com.github.tomakehurst.wiremock.client.WireMock.verify;
import static java.nio.charset.StandardCharsets.UTF_8;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.github.tomakehurst.wiremock.junit5.WireMockRuntimeInfo;
import com.github.tomakehurst.wiremock.junit5.WireMockTest;
import java.math.BigInteger;
import linea.crypto.Secp256k1Signature;
import net.consensys.linea.config.LineaLivenessServiceConfiguration;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.ECKeyPair;
import org.web3j.crypto.Hash;
import org.web3j.crypto.Keys;
import org.web3j.crypto.Sign;
import org.web3j.utils.Numeric;

@WireMockTest
class Web3SignerDigestSignerTest {
  private static final String SIGN_PATH = "/api/v1/eth1/sign/";
  private static final String PUBLIC_KEYS_PATH = "/api/v1/eth1/publicKeys";
  private final ECKeyPair keyPair = ECKeyPair.create(BigInteger.ONE);
  private final byte[] publicKey = Numeric.toBytesPadded(keyPair.getPublicKey(), 64);
  private final String publicKeyHex = Numeric.toHexStringNoPrefix(publicKey);
  private Web3SignerDigestSigner signer;

  @BeforeEach
  void setUp(final WireMockRuntimeInfo wireMock) {
    signer =
        new Web3SignerDigestSigner(
            LineaLivenessServiceConfiguration.builder()
                .signerUrl(wireMock.getHttpBaseUrl())
                .signerKeyId(publicKeyHex)
                .tlsEnabled(false)
                .build());
  }

  @Test
  void signsDigestWithoutApplyingAnotherHash() {
    final byte[] digest = Hash.sha3("liveness transaction".getBytes(UTF_8));
    final Sign.SignatureData signature = Sign.signMessage(digest, keyPair, false);
    final byte[] response = Bytes.concat(signature.getR(), signature.getS(), signature.getV());

    stubFor(
        post(urlEqualTo(SIGN_PATH + publicKeyHex))
            .withRequestBody(
                equalToJson(
                    "{\"data\":\"" + Numeric.toHexString(digest) + "\",\"applyHash\":false}"))
            .willReturn(aResponse().withStatus(200).withBody(Numeric.toHexString(response))));

    final Secp256k1Signature result = signer.sign(digest).join();

    assertThat(result.toRSBytes())
        .containsExactly(Bytes.concat(signature.getR(), signature.getS()));
    assertThat(signer.publicKey()).containsExactly(publicKey);
    verify(1, postRequestedFor(urlEqualTo(SIGN_PATH + publicKeyHex)));
  }

  @Test
  void resolvesPublicKeyWhenKeyIdentifierIsAnAlias(final WireMockRuntimeInfo wireMock) {
    final String keyAlias = "liveness-key";
    final String signerAddress =
        Numeric.prependHexPrefix(Keys.getAddress(new BigInteger(1, publicKey)));
    final byte[] digest = Hash.sha3("aliased liveness key".getBytes(UTF_8));
    final Sign.SignatureData signature = Sign.signMessage(digest, keyPair, false);
    stubFor(
        get(urlEqualTo(PUBLIC_KEYS_PATH))
            .willReturn(
                aResponse()
                    .withStatus(200)
                    .withHeader("Content-Type", "application/json")
                    .withBody("[\"" + Numeric.toHexString(publicKey) + "\"]")));
    stubFor(
        post(urlEqualTo(SIGN_PATH + keyAlias))
            .willReturn(
                aResponse()
                    .withStatus(200)
                    .withBody(
                        Numeric.toHexString(
                            Bytes.concat(signature.getR(), signature.getS(), signature.getV())))));

    final Web3SignerDigestSigner aliasSigner =
        new Web3SignerDigestSigner(
            LineaLivenessServiceConfiguration.builder()
                .signerUrl(wireMock.getHttpBaseUrl())
                .signerKeyId(keyAlias)
                .signerAddress(signerAddress)
                .tlsEnabled(false)
                .build());

    assertThat(aliasSigner.publicKey()).containsExactly(publicKey);
    assertThat(aliasSigner.sign(digest).join().toRSBytes())
        .containsExactly(Bytes.concat(signature.getR(), signature.getS()));
    verify(1, getRequestedFor(urlEqualTo(PUBLIC_KEYS_PATH)));
    verify(1, postRequestedFor(urlEqualTo(SIGN_PATH + keyAlias)));
  }

  @Test
  void rejectsInputThatIsNotA32ByteDigestWithoutCallingWeb3Signer() {
    assertThatThrownBy(() -> signer.sign(new byte[31]).join())
        .hasRootCauseInstanceOf(IllegalArgumentException.class)
        .hasRootCauseMessage("Web3Signer requires a 32-byte digest, got 31 bytes");

    verify(0, postRequestedFor(urlEqualTo(SIGN_PATH + publicKeyHex)));
  }

  @Test
  void rejectsMalformedSignatureResponse() {
    stubFor(
        post(urlEqualTo(SIGN_PATH + publicKeyHex))
            .willReturn(aResponse().withStatus(200).withBody("0x01")));

    assertThatThrownBy(() -> signer.sign(new byte[32]).join())
        .hasRootCauseInstanceOf(IllegalArgumentException.class)
        .hasRootCauseMessage("Web3Signer returned 1 bytes; expected 65 bytes (r || s || v)");
  }

  @Test
  void reportsHttpErrorStatus() {
    stubFor(
        post(urlEqualTo(SIGN_PATH + publicKeyHex))
            .willReturn(aResponse().withStatus(404).withBody("key not found")));

    assertThatThrownBy(() -> signer.sign(new byte[32]).join())
        .hasRootCauseInstanceOf(IllegalStateException.class)
        .hasRootCauseMessage("Web3Signer request failed with status 404");
  }

  private static final class Bytes {
    private Bytes() {}

    private static byte[] concat(final byte[]... values) {
      int length = 0;
      for (byte[] value : values) {
        length += value.length;
      }
      final byte[] result = new byte[length];
      int offset = 0;
      for (byte[] value : values) {
        System.arraycopy(value, 0, result, offset, value.length);
        offset += value.length;
      }
      return result;
    }
  }
}
