/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea

import com.fasterxml.jackson.core.JsonProcessingException
import com.fasterxml.jackson.databind.ObjectMapper
import com.github.tomakehurst.wiremock.client.WireMock.aResponse
import com.github.tomakehurst.wiremock.client.WireMock.equalTo
import com.github.tomakehurst.wiremock.client.WireMock.exactly
import com.github.tomakehurst.wiremock.client.WireMock.getAllServeEvents
import com.github.tomakehurst.wiremock.client.WireMock.matchingJsonPath
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor
import com.github.tomakehurst.wiremock.client.WireMock.stubFor
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.client.WireMock.verify
import com.github.tomakehurst.wiremock.http.Fault
import com.github.tomakehurst.wiremock.http.Request
import com.github.tomakehurst.wiremock.junit5.WireMockRuntimeInfo
import com.github.tomakehurst.wiremock.junit5.WireMockTest
import com.github.tomakehurst.wiremock.matching.MatchResult
import com.github.tomakehurst.wiremock.matching.StringValuePattern
import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import linea.plugin.acc.test.rpc.SendBundleRequest
import net.consensys.linea.bundles.BundleForwarder.RETRY_COUNT_HEADER
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeAll
import org.junit.jupiter.api.Test
import java.time.Duration
import java.util.concurrent.TimeUnit.SECONDS

@WireMockTest
class ForwardBundleTest : AbstractSendBundleTest() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-bundles-forward-urls=", wireMockRuntimeInfo!!.httpBaseUrl)
      .set(
        "--plugin-linea-bundles-forward-timeout=",
        Duration.ofSeconds(1).toMillis().toString(),
      )
      /**
       * If we use default retry delay of 1000ms, we get forwardIsRetriedAfterTimeout flakiness
       * because of the following race condition: t = 0s -> bundle sent; t = 1s -> bundle
       * forwarder timeout; t = 2s -> bundle retry + verifyRequestForwarded timeout
       *
       * The race condition is between bundle retry and verifyRequestForwarded timeout. If
       * verifyRequestForwarded times out before the bundle retry is completed, the test will fail
       *
       * We set retry delay to 900ms to avoid this race condition such that t = 1.9s -> bundle
       * retry; t = 2.0s -> verifyRequestForwarded timeout
       */
      .set(
        "--plugin-linea-bundles-forward-retry-delay=",
        Duration.ofMillis(900).toMillis().toString(),
      )
      .build()
  }

  @Test
  fun bundleIsForwarded() {
    val bundleParams = sendBundle(1)
    stubSuccessResponseFor(bundleParams, 0)
    verifyRequestForwarded(bundleParams)
  }

  @Test
  fun forwardIsRetriedAfterTimeout() {
    val bundleParams = sendBundle(2)
    stubSuccessResponseFor(bundleParams, 0, Duration.ofSeconds(2))

    verifyResponseSent(bundleParams)

    verifyRequestForwarded(bundleParams, 1)
  }

  @Test
  fun forwardIsRetriedAfterNetworkFailure() {
    val bundleParams = sendBundle(3)
    stubFailureFor(bundleParams)
    stubSuccessResponseFor(bundleParams, 1)

    verifyRequestForwarded(bundleParams)
    verifyRequestForwarded(bundleParams, 1)
  }

  private fun stubSuccessResponseFor(bundleParams: BundleParams, retryCount: Int) {
    stubSuccessResponseFor(bundleParams, retryCount, Duration.ZERO)
  }

  private fun stubSuccessResponseFor(
    bundleParams: BundleParams,
    retryCount: Int,
    delay: Duration,
  ) {
    val requestMatcher = post(urlEqualTo("/"))

    if (retryCount > 0) {
      requestMatcher.withHeader(RETRY_COUNT_HEADER, equalTo(retryCount.toString()))
    } else {
      requestMatcher.andMatching(::noRetryCountHeader)
    }

    stubFor(
      requestMatcher
        .withHeader("Content-Type", equalTo("application/json; charset=UTF-8"))
        .withRequestBody(matchingBlockNumber(bundleParams.blockNumber))
        .willReturn(
          aResponse()
            .withFixedDelay(delay.toMillis().toInt())
            .withTransformers("response-template")
            .withStatus(200)
            .withHeader("Content-Type", "application/json")
            .withBody(
              """
                {
                  "jsonrpc": "2.0",
                  "result": {
                    "bundleHash": "<bundleHash>"
                  },
                  "id": {{jsonPath request.body '$.id'}}
                }""".replace("<bundleHash>", "0xb${bundleParams.blockNumber}"),
            ),
        ),
    )
  }

  private fun stubFailureFor(bundleParams: BundleParams) {
    stubFor(
      post(urlEqualTo("/"))
        .withHeader("Content-Type", equalTo("application/json; charset=UTF-8"))
        .withRequestBody(matchingBlockNumber(bundleParams.blockNumber))
        .andMatching(::noRetryCountHeader)
        .willReturn(aResponse().withFault(Fault.CONNECTION_RESET_BY_PEER)),
    )
  }

  private fun noRetryCountHeader(request: Request): MatchResult {
    return object : MatchResult() {
      override fun isExactMatch(): Boolean {
        return !request.allHeaderKeys.contains(RETRY_COUNT_HEADER)
      }

      override fun getDistance(): Double {
        return 0.0
      }
    }
  }

  private fun sendBundle(blockNumber: Int): BundleParams {
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.primaryBenefactor

    val tx = accountTransactions.createTransfer(sender, recipient, 1)

    val bundleRawTx = tx.signedTransactionData()

    val bundleParams =
      BundleParams(arrayOf(bundleRawTx), Integer.toHexString(blockNumber))

    val sendBundleRequest = SendBundleRequest(bundleParams)
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isFalse()
    assertThat(sendBundleResponse.result.bundleHash).isNotBlank()

    return bundleParams
  }

  companion object {
    private val OBJECT_MAPPER = ObjectMapper()
    private var wireMockRuntimeInfo: WireMockRuntimeInfo? = null

    @JvmStatic
    @BeforeAll
    fun beforeAll(wireMockRuntimeInfo: WireMockRuntimeInfo) {
      ForwardBundleTest.wireMockRuntimeInfo = wireMockRuntimeInfo
    }

    private fun matchingBlockNumber(blockNumber: String): StringValuePattern {
      return matchingJsonPath("$.params[?(@.blockNumber == $blockNumber)]")
    }

    private fun verifyRequestForwarded(bundleParams: BundleParams) {
      verifyRequestForwarded(bundleParams, 0)
    }

    private fun verifyRequestForwarded(bundleParams: BundleParams, retryCount: Int) {
      val patternBuilder = postRequestedFor(urlEqualTo("/"))
        .withHeader("Content-Type", equalTo("application/json; charset=UTF-8"))
        .withRequestBody(matchingBundleParams(bundleParams))

      if (retryCount > 0) {
        patternBuilder.withHeader(RETRY_COUNT_HEADER, equalTo(retryCount.toString()))
      } else {
        patternBuilder.withoutHeader(RETRY_COUNT_HEADER)
      }

      await().atMost(2, SECONDS).untilAsserted { verify(exactly(1), patternBuilder) }
    }

    private fun verifyResponseSent(bundleParams: BundleParams) {
      await()
        .atMost(2, SECONDS)
        .until {
          getAllServeEvents()
            .map { it.response }
            .map { it.bodyAsString }
            .map { body ->
              try {
                OBJECT_MAPPER.readTree(body)
              } catch (e: JsonProcessingException) {
                throw RuntimeException(e)
              }
            }
            .any { jsonNode ->
              jsonNode
                .findPath("bundleHash")
                .textValue() == "0xb${bundleParams.blockNumber}"
            }
        }
    }

    private fun matchingBundleParams(bundleParams: BundleParams): StringValuePattern {
      return matchingJsonPath(
        "$.params[?(@.blockNumber == ${bundleParams.blockNumber})]",
      ).and(
        matchingJsonPath(
          "$.params[?(@.txs == [${bundleParams.txs.joinToString(",")}])]",
        ),
      )
    }
  }
}
