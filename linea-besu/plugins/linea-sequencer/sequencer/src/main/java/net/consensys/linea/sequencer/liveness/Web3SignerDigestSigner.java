/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.liveness;

import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.FileInputStream;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.security.KeyStore;
import java.time.Duration;
import java.util.Arrays;
import java.util.Map;
import javax.net.ssl.KeyManagerFactory;
import javax.net.ssl.SSLContext;
import javax.net.ssl.TrustManagerFactory;
import linea.crypto.Secp256k1Signature;
import linea.crypto.Signer;
import net.consensys.linea.config.LineaLivenessServiceConfiguration;
import org.web3j.utils.Numeric;
import tech.pegasys.teku.infrastructure.async.SafeFuture;

/** Web3Signer client that signs a caller-supplied digest without hashing it again. */
public class Web3SignerDigestSigner implements Signer<Secp256k1Signature> {
  private static final int DIGEST_SIZE = 32;
  private static final int SIGNATURE_SIZE = Secp256k1Signature.SIZE_BYTES + 1;
  private final ObjectMapper objectMapper = new ObjectMapper();
  private final HttpClient httpClient;
  private final URI endpoint;
  private final byte[] publicKey;

  public Web3SignerDigestSigner(final LineaLivenessServiceConfiguration config) {
    endpoint = URI.create(config.signerUrl() + "/api/v1/eth1/sign/" + config.signerKeyId());
    publicKey = Numeric.hexStringToByteArray(config.signerKeyId());
    HttpClient.Builder builder = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(30));
    if (config.tlsEnabled()) {
      builder.sslContext(buildSslContext(config));
    }
    httpClient = builder.build();
  }

  @Override
  public byte[] publicKey() {
    return publicKey.clone();
  }

  @Override
  public SafeFuture<Secp256k1Signature> sign(final byte[] digest) {
    if (digest.length != DIGEST_SIZE) {
      return SafeFuture.failedFuture(
          new IllegalArgumentException(
              "Web3Signer requires a 32-byte digest, got " + digest.length + " bytes"));
    }
    try {
      String body =
          objectMapper.writeValueAsString(
              Map.of("data", Numeric.toHexString(digest), "applyHash", false));
      HttpRequest request =
          HttpRequest.newBuilder()
              .uri(endpoint)
              .header("Content-Type", "application/json")
              .timeout(Duration.ofSeconds(30))
              .POST(HttpRequest.BodyPublishers.ofString(body))
              .build();
      return SafeFuture.of(
          httpClient
              .sendAsync(request, HttpResponse.BodyHandlers.ofString())
              .thenApply(this::parseResponse));
    } catch (Exception e) {
      return SafeFuture.failedFuture(e);
    }
  }

  private Secp256k1Signature parseResponse(final HttpResponse<String> response) {
    if (response.statusCode() != 200) {
      throw new IllegalStateException(
          "Web3Signer request failed with status " + response.statusCode());
    }
    byte[] signature = Numeric.hexStringToByteArray(response.body().replace("\"", "").trim());
    if (signature.length != SIGNATURE_SIZE) {
      throw new IllegalArgumentException(
          "Web3Signer returned "
              + signature.length
              + " bytes; expected "
              + SIGNATURE_SIZE
              + " bytes (r || s || v)");
    }
    return Secp256k1Signature.Companion.fromRSBytes(
        Arrays.copyOf(signature, Secp256k1Signature.SIZE_BYTES));
  }

  private static SSLContext buildSslContext(final LineaLivenessServiceConfiguration config) {
    try (FileInputStream keyStoreInput = new FileInputStream(config.tlsKeyStorePath().toFile());
        FileInputStream trustStoreInput =
            new FileInputStream(config.tlsTrustStorePath().toFile())) {
      KeyStore keyStore = KeyStore.getInstance("PKCS12");
      keyStore.load(keyStoreInput, config.tlsKeyStorePassword().toCharArray());
      KeyManagerFactory keyManagers =
          KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm());
      keyManagers.init(keyStore, config.tlsKeyStorePassword().toCharArray());

      KeyStore trustStore = KeyStore.getInstance("PKCS12");
      trustStore.load(trustStoreInput, config.tlsTrustStorePassword().toCharArray());
      TrustManagerFactory trustManagers =
          TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm());
      trustManagers.init(trustStore);

      SSLContext sslContext = SSLContext.getInstance("TLS");
      sslContext.init(keyManagers.getKeyManagers(), trustManagers.getTrustManagers(), null);
      return sslContext;
    } catch (Exception e) {
      throw new IllegalStateException("Failed to initialize Web3Signer TLS", e);
    }
  }
}
