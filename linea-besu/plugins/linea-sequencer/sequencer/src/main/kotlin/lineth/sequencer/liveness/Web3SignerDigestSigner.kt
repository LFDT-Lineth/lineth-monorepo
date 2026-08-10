/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.sequencer.liveness

import com.fasterxml.jackson.databind.ObjectMapper
import linea.crypto.Secp256k1Signature
import linea.crypto.Signer
import lineth.config.LineaLivenessServiceConfiguration
import org.web3j.crypto.Keys
import org.web3j.utils.Numeric
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.io.FileInputStream
import java.io.IOException
import java.math.BigInteger
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.security.KeyStore
import java.time.Duration
import java.util.HexFormat
import javax.net.ssl.KeyManagerFactory
import javax.net.ssl.SSLContext
import javax.net.ssl.TrustManagerFactory

/** Web3Signer client that signs a caller-supplied digest without hashing it again. */
class Web3SignerDigestSigner(
  config: LineaLivenessServiceConfiguration,
) : Signer<Secp256k1Signature> {
  private val objectMapper = ObjectMapper()
  private val endpoint =
    URI.create("${config.signerUrl()}/api/v1/eth1/sign/${config.signerKeyId()}")
  private val httpClient =
    HttpClient.newBuilder()
      .connectTimeout(REQUEST_TIMEOUT)
      .apply {
        if (config.tlsEnabled()) {
          sslContext(buildSslContext(config))
        }
      }
      .build()
  private val publicKey = resolvePublicKey(config)

  override fun publicKey(): ByteArray = publicKey.clone()

  override fun sign(bytes: ByteArray): SafeFuture<Secp256k1Signature> {
    if (bytes.size != DIGEST_SIZE) {
      return SafeFuture.failedFuture(
        IllegalArgumentException(
          "Web3Signer requires a 32-byte digest, got ${bytes.size} bytes",
        ),
      )
    }

    return try {
      val body =
        objectMapper.writeValueAsString(
          mapOf("data" to Numeric.toHexString(bytes), "applyHash" to false),
        )
      val request =
        HttpRequest.newBuilder()
          .uri(endpoint)
          .header("Content-Type", "application/json")
          .timeout(REQUEST_TIMEOUT)
          .POST(HttpRequest.BodyPublishers.ofString(body))
          .build()
      SafeFuture.of(
        httpClient
          .sendAsync(request, HttpResponse.BodyHandlers.ofString())
          .thenApply(::parseResponse),
      )
    } catch (error: Exception) {
      SafeFuture.failedFuture(error)
    }
  }

  private fun parseResponse(response: HttpResponse<String>): Secp256k1Signature {
    check(response.statusCode() == 200) {
      "Web3Signer request failed with status ${response.statusCode()}"
    }
    val signature =
      Numeric.hexStringToByteArray(response.body().replace("\"", "").trim())
    require(signature.size == SIGNATURE_SIZE) {
      "Web3Signer returned ${signature.size} bytes; expected $SIGNATURE_SIZE bytes (r || s || v)"
    }
    return Secp256k1Signature.fromRSBytes(
      signature.copyOf(Secp256k1Signature.SIZE_BYTES),
    )
  }

  private fun resolvePublicKey(config: LineaLivenessServiceConfiguration): ByteArray {
    val keyId = Numeric.cleanHexPrefix(config.signerKeyId())
    if (PUBLIC_KEY_PATTERN.matches(keyId)) {
      return HexFormat.of().parseHex(keyId)
    }

    val publicKeysEndpoint = URI.create("${config.signerUrl()}/api/v1/eth1/publicKeys")
    val request =
      HttpRequest.newBuilder()
        .uri(publicKeysEndpoint)
        .timeout(REQUEST_TIMEOUT)
        .GET()
        .build()
    try {
      val response = httpClient.send(request, HttpResponse.BodyHandlers.ofString())
      check(response.statusCode() == 200) {
        "Web3Signer public key request failed with status ${response.statusCode()}"
      }
      objectMapper.readValue(response.body(), Array<String>::class.java).forEach { candidate ->
        val candidateBytes = Numeric.hexStringToByteArray(candidate)
        if (
          candidateBytes.size == PUBLIC_KEY_SIZE &&
          addressOf(candidateBytes).equals(config.signerAddress(), ignoreCase = true)
        ) {
          return candidateBytes
        }
      }
      throw IllegalStateException(
        "Web3Signer did not return a public key for configured signer address " +
          config.signerAddress(),
      )
    } catch (error: InterruptedException) {
      Thread.currentThread().interrupt()
      throw IllegalStateException("Web3Signer public key request was interrupted", error)
    } catch (error: IOException) {
      throw IllegalStateException("Failed to resolve Web3Signer public key", error)
    }
  }

  private companion object {
    const val DIGEST_SIZE = 32
    const val PUBLIC_KEY_SIZE = 64
    const val SIGNATURE_SIZE = Secp256k1Signature.SIZE_BYTES + 1
    val REQUEST_TIMEOUT: Duration = Duration.ofSeconds(30)
    val PUBLIC_KEY_PATTERN = Regex("[0-9a-fA-F]{128}")

    fun addressOf(publicKey: ByteArray): String =
      Numeric.prependHexPrefix(Keys.getAddress(BigInteger(1, publicKey)))

    fun buildSslContext(config: LineaLivenessServiceConfiguration): SSLContext {
      try {
        FileInputStream(config.tlsKeyStorePath().toFile()).use { keyStoreInput ->
          FileInputStream(config.tlsTrustStorePath().toFile()).use { trustStoreInput ->
            val keyStore = KeyStore.getInstance("PKCS12")
            keyStore.load(keyStoreInput, config.tlsKeyStorePassword().toCharArray())
            val keyManagers =
              KeyManagerFactory.getInstance(KeyManagerFactory.getDefaultAlgorithm())
            keyManagers.init(keyStore, config.tlsKeyStorePassword().toCharArray())

            val trustStore = KeyStore.getInstance("PKCS12")
            trustStore.load(trustStoreInput, config.tlsTrustStorePassword().toCharArray())
            val trustManagers =
              TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
            trustManagers.init(trustStore)

            return SSLContext.getInstance("TLS").apply {
              init(keyManagers.keyManagers, trustManagers.trustManagers, null)
            }
          }
        }
      } catch (error: Exception) {
        throw IllegalStateException("Failed to initialize Web3Signer TLS", error)
      }
    }
  }
}
