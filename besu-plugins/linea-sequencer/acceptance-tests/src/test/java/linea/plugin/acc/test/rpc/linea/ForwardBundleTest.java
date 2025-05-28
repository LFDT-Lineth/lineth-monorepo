/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea;

import static com.github.tomakehurst.wiremock.client.WireMock.aResponse;
import static com.github.tomakehurst.wiremock.client.WireMock.equalTo;
import static com.github.tomakehurst.wiremock.client.WireMock.exactly;
import static com.github.tomakehurst.wiremock.client.WireMock.getAllServeEvents;
import static com.github.tomakehurst.wiremock.client.WireMock.matchingJsonPath;
import static com.github.tomakehurst.wiremock.client.WireMock.post;
import static com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor;
import static com.github.tomakehurst.wiremock.client.WireMock.request;
import static com.github.tomakehurst.wiremock.client.WireMock.stubFor;
import static com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo;
import static com.github.tomakehurst.wiremock.client.WireMock.verify;
import static java.util.concurrent.TimeUnit.SECONDS;
import static net.consensys.linea.bundles.BundleForwarder.RETRY_COUNT_HEADER;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import java.time.Duration;
import java.util.Arrays;
import java.util.List;
import java.util.stream.Collectors;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.github.tomakehurst.wiremock.http.Fault;
import com.github.tomakehurst.wiremock.http.LoggedResponse;
import com.github.tomakehurst.wiremock.http.Request;
import com.github.tomakehurst.wiremock.junit5.WireMockRuntimeInfo;
import com.github.tomakehurst.wiremock.junit5.WireMockTest;
import com.github.tomakehurst.wiremock.matching.MatchResult;
import com.github.tomakehurst.wiremock.matching.StringValuePattern;
import com.github.tomakehurst.wiremock.stubbing.ServeEvent;
import linea.plugin.acc.test.TestCommandLineOptionsBuilder;
import org.hyperledger.besu.tests.acceptance.dsl.account.Account;
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransaction;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

@WireMockTest
public class ForwardBundleTest extends AbstractSendBundleTest {
  protected static final ObjectMapper OBJECT_MAPPER = new ObjectMapper();
  private static WireMockRuntimeInfo wireMockRuntimeInfo;

  @BeforeAll
  public static void beforeAll(final WireMockRuntimeInfo wireMockRuntimeInfo) {
    ForwardBundleTest.wireMockRuntimeInfo = wireMockRuntimeInfo;
  }

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-bundles-forward-urls=", wireMockRuntimeInfo.getHttpBaseUrl())
        .set(
            "--plugin-linea-bundles-forward-timeout=",
            String.valueOf(Duration.ofSeconds(1).toMillis()))
        .build();
  }

  @Test
  public void bundleIsForwarded() {
    final var bundleParams = sendBundle(1);
    stubSuccessResponseFor(bundleParams, 0);
    verifyRequestForwarded(bundleParams);
  }

  @Test
  public void forwardIsRetriedAfterTimeout() {
    final var bundleParams = sendBundle(2);
    stubSuccessResponseFor(bundleParams, 0, Duration.ofSeconds(2));

    verifyResponseSent(bundleParams);

    verifyRequestForwarded(bundleParams, 1);
  }

  @Test
  public void forwardIsRetriedAfterNetworkFailure() {
    final var bundleParams = sendBundle(3);
    stubFailureFor(bundleParams);
    stubSuccessResponseFor(bundleParams, 1);

    verifyRequestForwarded(bundleParams);
    verifyRequestForwarded(bundleParams, 1);
  }

  private void stubSuccessResponseFor(final BundleParams bundleParams, final int retryCount) {
    stubSuccessResponseFor(bundleParams, retryCount, Duration.ZERO);
  }

  private void stubSuccessResponseFor(
      final BundleParams bundleParams, final int retryCount, final Duration delay) {
    final var requestMatcher = post(urlEqualTo("/"));

    if (retryCount > 0) {
      requestMatcher.withHeader(RETRY_COUNT_HEADER, equalTo(String.valueOf(retryCount)));
    } else {
      requestMatcher.andMatching(this::noRetryCountHeader);
    }

    stubFor(
        requestMatcher
            .withHeader("Content-Type", equalTo("application/json; charset=UTF-8"))
            .withRequestBody(matchingBlockNumber(bundleParams.blockNumber()))
            .willReturn(
                aResponse()
                    .withFixedDelay((int) delay.toMillis())
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
                            }"""
                            .replace("<bundleHash>", "0xb" + bundleParams.blockNumber()))));
  }

  private void stubFailureFor(final BundleParams bundleParams) {
    stubFor(
        post(urlEqualTo("/"))
            .withHeader("Content-Type", equalTo("application/json; charset=UTF-8"))
            .withRequestBody(matchingBlockNumber(bundleParams.blockNumber()))
            .andMatching(this::noRetryCountHeader)
            .willReturn(aResponse().withFault(Fault.CONNECTION_RESET_BY_PEER)));
  }

  private MatchResult noRetryCountHeader(final Request request) {
    return new MatchResult() {
      @Override
      public boolean isExactMatch() {
        return !request.getAllHeaderKeys().contains(RETRY_COUNT_HEADER);
      }

      @Override
      public double getDistance() {
        return 0;
      }
    };
  }

  private static StringValuePattern matchingBlockNumber(final String blockNumber) {
    return matchingJsonPath("$.params[?(@.blockNumber == %s)]".formatted(blockNumber));
  }

  private static void verifyRequestForwarded(final BundleParams bundleParams) {
    verifyRequestForwarded(bundleParams, 0);
  }

  private static void verifyRequestForwarded(
      final BundleParams bundleParams, final int retryCount) {

    final var patternBuilder =
        postRequestedFor(urlEqualTo("/"))
            .withHeader("Content-Type", equalTo("application/json; charset=UTF-8"))
            .withRequestBody(matchingBundleParams(bundleParams));

    if (retryCount > 0) {
      patternBuilder.withHeader(RETRY_COUNT_HEADER, equalTo(String.valueOf(retryCount)));
    } else {
      patternBuilder.withoutHeader(RETRY_COUNT_HEADER);
    }

    await().atMost(2, SECONDS).untilAsserted(() -> verify(exactly(1), patternBuilder));
  }

  private static void verifyResponseSent(final BundleParams bundleParams) {
    await()
        .atMost(2, SECONDS)
        .until(
            () ->
                getAllServeEvents().stream()
                    .map(ServeEvent::getResponse)
                    .map(LoggedResponse::getBodyAsString)
                    .map(
                        body -> {
                          try {
                            return OBJECT_MAPPER.readTree(body);
                          } catch (JsonProcessingException e) {
                            throw new RuntimeException(e);
                          }
                        })
                    .anyMatch(
                        jsonNode ->
                            jsonNode
                                .findPath("bundleHash")
                                .textValue()
                                .equals("0xb" + bundleParams.blockNumber())));
  }

  private static StringValuePattern matchingBundleParams(final BundleParams bundleParams) {
    return matchingJsonPath(
            "$.params[?(@.blockNumber == %s)]".formatted(bundleParams.blockNumber()))
        .and(
            matchingJsonPath(
                "$.params[?(@.txs == [%s])]"
                    .formatted(
                        Arrays.stream(bundleParams.txs()).collect(Collectors.joining(",")))));
  }

  private BundleParams sendBundle(final int blockNumber) {
    final Account sender = accounts.getSecondaryBenefactor();
    final Account recipient = accounts.getPrimaryBenefactor();

    final TransferTransaction tx = accountTransactions.createTransfer(sender, recipient, 1);

    final String bundleRawTx = tx.signedTransactionData();

    final var bundleParams =
        new BundleParams(new String[] {bundleRawTx}, Integer.toHexString(blockNumber));

    final var sendBundleRequest = new SendBundleRequest(bundleParams);
    final var sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests());

    assertThat(sendBundleResponse.hasError()).isFalse();
    assertThat(sendBundleResponse.getResult().bundleHash()).isNotBlank();

    return bundleParams;
  }
}
