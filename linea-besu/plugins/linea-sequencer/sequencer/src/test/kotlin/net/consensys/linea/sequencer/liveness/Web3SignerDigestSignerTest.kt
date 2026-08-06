/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.liveness

import com.github.tomakehurst.wiremock.client.WireMock.aResponse
import com.github.tomakehurst.wiremock.client.WireMock.equalToJson
import com.github.tomakehurst.wiremock.client.WireMock.get
import com.github.tomakehurst.wiremock.client.WireMock.getRequestedFor
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor
import com.github.tomakehurst.wiremock.client.WireMock.stubFor
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.client.WireMock.verify
import com.github.tomakehurst.wiremock.junit5.WireMockRuntimeInfo
import com.github.tomakehurst.wiremock.junit5.WireMockTest
import net.consensys.linea.config.LineaLivenessServiceConfiguration
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.crypto.ECKeyPair
import org.web3j.crypto.Hash
import org.web3j.crypto.Keys
import org.web3j.crypto.Sign
import org.web3j.utils.Numeric
import java.math.BigInteger
import java.nio.charset.StandardCharsets.UTF_8

@WireMockTest
class Web3SignerDigestSignerTest {
  private val keyPair = ECKeyPair.create(BigInteger.ONE)
  private val publicKey = Numeric.toBytesPadded(keyPair.publicKey, 64)
  private val publicKeyHex = Numeric.toHexStringNoPrefix(publicKey)
  private lateinit var signer: Web3SignerDigestSigner

  @BeforeEach
  fun setUp(wireMock: WireMockRuntimeInfo) {
    signer =
      Web3SignerDigestSigner(
        LineaLivenessServiceConfiguration.builder()
          .signerUrl(wireMock.httpBaseUrl)
          .signerKeyId(publicKeyHex)
          .tlsEnabled(false)
          .build(),
      )
  }

  @Test
  fun `signs digest without applying another hash`() {
    val digest = Hash.sha3("liveness transaction".toByteArray(UTF_8))
    val signature = Sign.signMessage(digest, keyPair, false)
    val response = signature.bytes()

    stubFor(
      post(urlEqualTo(SIGN_PATH + publicKeyHex))
        .withRequestBody(
          equalToJson(
            """{"data":"${Numeric.toHexString(digest)}","applyHash":false}""",
          ),
        )
        .willReturn(aResponse().withStatus(200).withBody(Numeric.toHexString(response))),
    )

    val result = signer.sign(digest).join()

    assertThat(result.toRSBytes()).containsExactly(*(signature.r + signature.s))
    assertThat(signer.publicKey()).containsExactly(*publicKey)
    verify(1, postRequestedFor(urlEqualTo(SIGN_PATH + publicKeyHex)))
  }

  @Test
  fun `resolves public key when key identifier is an alias`(
    wireMock: WireMockRuntimeInfo,
  ) {
    val keyAlias = "liveness-key"
    val signerAddress =
      Numeric.prependHexPrefix(Keys.getAddress(BigInteger(1, publicKey)))
    val digest = Hash.sha3("aliased liveness key".toByteArray(UTF_8))
    val signature = Sign.signMessage(digest, keyPair, false)
    stubFor(
      get(urlEqualTo(PUBLIC_KEYS_PATH))
        .willReturn(
          aResponse()
            .withStatus(200)
            .withHeader("Content-Type", "application/json")
            .withBody("""["${Numeric.toHexString(publicKey)}"]"""),
        ),
    )
    stubFor(
      post(urlEqualTo(SIGN_PATH + keyAlias))
        .willReturn(
          aResponse()
            .withStatus(200)
            .withBody(Numeric.toHexString(signature.bytes())),
        ),
    )

    val aliasSigner =
      Web3SignerDigestSigner(
        LineaLivenessServiceConfiguration.builder()
          .signerUrl(wireMock.httpBaseUrl)
          .signerKeyId(keyAlias)
          .signerAddress(signerAddress)
          .tlsEnabled(false)
          .build(),
      )

    assertThat(aliasSigner.publicKey()).containsExactly(*publicKey)
    assertThat(aliasSigner.sign(digest).join().toRSBytes())
      .containsExactly(*(signature.r + signature.s))
    verify(1, getRequestedFor(urlEqualTo(PUBLIC_KEYS_PATH)))
    verify(1, postRequestedFor(urlEqualTo(SIGN_PATH + keyAlias)))
  }

  @Test
  fun `rejects input that is not a 32-byte digest without calling Web3Signer`() {
    assertThatThrownBy { signer.sign(ByteArray(31)).join() }
      .hasRootCauseInstanceOf(IllegalArgumentException::class.java)
      .hasRootCauseMessage("Web3Signer requires a 32-byte digest, got 31 bytes")

    verify(0, postRequestedFor(urlEqualTo(SIGN_PATH + publicKeyHex)))
  }

  @Test
  fun `rejects malformed signature response`() {
    stubFor(
      post(urlEqualTo(SIGN_PATH + publicKeyHex))
        .willReturn(aResponse().withStatus(200).withBody("0x01")),
    )

    assertThatThrownBy { signer.sign(ByteArray(32)).join() }
      .hasRootCauseInstanceOf(IllegalArgumentException::class.java)
      .hasRootCauseMessage("Web3Signer returned 1 bytes; expected 65 bytes (r || s || v)")
  }

  @Test
  fun `reports HTTP error status`() {
    stubFor(
      post(urlEqualTo(SIGN_PATH + publicKeyHex))
        .willReturn(aResponse().withStatus(404).withBody("key not found")),
    )

    assertThatThrownBy { signer.sign(ByteArray(32)).join() }
      .hasRootCauseInstanceOf(IllegalStateException::class.java)
      .hasRootCauseMessage("Web3Signer request failed with status 404")
  }

  private fun Sign.SignatureData.bytes(): ByteArray = r + s + v

  private companion object {
    const val SIGN_PATH = "/api/v1/eth1/sign/"
    const val PUBLIC_KEYS_PATH = "/api/v1/eth1/publicKeys"
  }
}
