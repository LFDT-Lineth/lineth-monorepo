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

package net.consensys.linea.bundles;

import static com.github.tomakehurst.wiremock.client.WireMock.aResponse;
import static com.github.tomakehurst.wiremock.client.WireMock.equalTo;
import static com.github.tomakehurst.wiremock.client.WireMock.equalToJson;
import static com.github.tomakehurst.wiremock.client.WireMock.exactly;
import static com.github.tomakehurst.wiremock.client.WireMock.getAllServeEvents;
import static com.github.tomakehurst.wiremock.client.WireMock.matchingJsonPath;
import static com.github.tomakehurst.wiremock.client.WireMock.post;
import static com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor;
import static com.github.tomakehurst.wiremock.client.WireMock.stubFor;
import static com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo;
import static com.github.tomakehurst.wiremock.client.WireMock.verify;
import static java.util.concurrent.TimeUnit.SECONDS;
import static net.consensys.linea.bundles.BundleForwarder.RETRY_COUNT_HEADER;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.fail;
import static org.awaitility.Awaitility.await;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Answers.RETURNS_DEEP_STUBS;
import static org.mockito.Mockito.lenient;

import java.io.IOException;
import java.io.InterruptedIOException;
import java.net.MalformedURLException;
import java.net.URI;
import java.time.Duration;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Executors;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.github.tomakehurst.wiremock.junit5.WireMockRuntimeInfo;
import com.github.tomakehurst.wiremock.junit5.WireMockTest;
import com.github.tomakehurst.wiremock.matching.StringValuePattern;
import net.consensys.linea.bundles.BundleForwarder.SendBundleResponse;
import net.consensys.linea.config.LineaBundleConfiguration;
import net.consensys.linea.utils.PriorityThreadPoolExecutor;
import net.consensys.linea.utils.TestablePriorityThreadPoolExecutor;
import okhttp3.OkHttpClient;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@WireMockTest
@ExtendWith(MockitoExtension.class)
class BundleForwarderTest extends AbstractBundleTest {
  private static final AtomicLong REQ_ID_COUNT = new AtomicLong(0);
  private static final Duration RPC_CALL_TIMEOUT = Duration.ofSeconds(2);
  private static final long CHAIN_HEAD_BLOCK_NUMBER = 5L;
  private LineaBundleConfiguration bundleConfiguration;
  private BundleForwarder bundleForwarder;
  private TestablePriorityThreadPoolExecutor executor;
  private ResponseCollector responseCollector;

  @Mock(answer = RETURNS_DEEP_STUBS)
  private BlockchainService blockchainService;

  @BeforeEach
  void init(final WireMockRuntimeInfo wmInfo) throws MalformedURLException {
    REQ_ID_COUNT.set(0);
    lenient()
        .when(blockchainService.getChainHeadHeader().getNumber())
        .thenReturn(CHAIN_HEAD_BLOCK_NUMBER);
    responseCollector = new ResponseCollector();
    executor =
        new TestablePriorityThreadPoolExecutor(0, 1, 1, SECONDS, Thread.ofVirtual().factory());
    executor.addAfterExecuteListener(responseCollector);
    bundleConfiguration =
        LineaBundleConfiguration.builder()
            .forwardUrls(Set.of(URI.create(wmInfo.getHttpBaseUrl()).toURL()))
            .timeoutMillis(5000)
            .retryDelayMillis(100)
            .build();

    bundleForwarder =
        new BundleForwarder(
            bundleConfiguration,
            executor,
            Executors.newSingleThreadScheduledExecutor(),
            blockchainService,
            new OkHttpClient.Builder().callTimeout(RPC_CALL_TIMEOUT).build(),
            URI.create(wmInfo.getHttpBaseUrl()).toURL());
  }

  @AfterEach
  void cleanup() {
    executor.shutdown();
  }

  @Test
  void bundleIsForwarded() throws IOException {
    final var bundle =
        createBundle(
            Hash.fromHexStringLenient("0x1234"), CHAIN_HEAD_BLOCK_NUMBER + 1, List.of(TX1, TX2));
    stubSuccessResponseFor(bundle, 0);

    bundleForwarder.onTransactionBundleAdded(bundle);

    verifyRequestSentFor(bundle, 0);
    responseCollector.assertSuccessResponse(0);
  }

  @Test
  void timeoutRequestIsRetried() throws IOException {
    final var bundle =
        createBundle(
            Hash.fromHexStringLenient("0x1234"), CHAIN_HEAD_BLOCK_NUMBER + 1, List.of(TX1, TX2));
    stubTimeoutResponseFor(bundle, 0);

    bundleForwarder.onTransactionBundleAdded(bundle);

    executor.addAfterExecuteListener(
        (r, t) ->
            // now change the response to success at the second attempt
            stubSuccessResponseFor(bundle, 1));

    verifyRequestSentFor(bundle, 0);
    responseCollector.assertFailedResponse(0, InterruptedIOException.class);
    verifyRequestSentFor(bundle, 1, 1);
    responseCollector.assertSuccessResponse(1);
  }

  @Test
  void bundleWithLowerBlockNumberIsSentFirst()
      throws IOException, ExecutionException, InterruptedException {
    final var bundle_bn1 =
        createBundle(
            Hash.fromHexStringLenient("0x9abc"), CHAIN_HEAD_BLOCK_NUMBER + 1, List.of(TX1, TX3));
    final var bundle_bn2 =
        createBundle(
            Hash.fromHexStringLenient("0x5678"), CHAIN_HEAD_BLOCK_NUMBER + 2, List.of(TX2, TX3));

    stubSuccessResponseFor(bundle_bn1, 0);
    stubSuccessResponseFor(bundle_bn2, 1);

    // since we want to form a queue of bundle to form, we just wait before executing the send
    // bundle task that all the expected tasks are queued
    executor.waitForQueueTaskCount(2, true);

    // we submit bundle_bn2, bundle_bn1 in this order
    // we expect they are sent in this order: bundle_bn1, bundle_bn2
    // with lower block number bundles sent first
    bundleForwarder.onTransactionBundleAdded(bundle_bn2);
    bundleForwarder.onTransactionBundleAdded(bundle_bn1);

    // reqId ensures the correct order
    verifyRequestSentFor(bundle_bn1, 0);
    responseCollector.assertSuccessResponse(0);
    verifyRequestSentFor(bundle_bn2, 1);
    responseCollector.assertSuccessResponse(1);
  }

  @Test
  void requestForAlreadyImportedBlockIsSkipped() {
    final var bundle =
        createBundle(
            Hash.fromHexStringLenient("0x1234"), CHAIN_HEAD_BLOCK_NUMBER, List.of(TX1, TX2));
    bundleForwarder.onTransactionBundleAdded(bundle);

    responseCollector.assertSkippedBundle(bundle);

    // ensure no rpc call was sent
    assertThat(getAllServeEvents()).isEmpty();
  }

  @Test
  void failedRequestIsRetriedAfterBundlesForTheNextBlock()
      throws IOException, ExecutionException, InterruptedException {
    final var bundle_bn1 =
        createBundle(
            Hash.fromHexStringLenient("0x9abc"), CHAIN_HEAD_BLOCK_NUMBER + 1, List.of(TX1, TX3));
    final var bundle_bn2 =
        createBundle(
            Hash.fromHexStringLenient("0x5678"), CHAIN_HEAD_BLOCK_NUMBER + 2, List.of(TX2, TX3));

    stubTimeoutResponseFor(bundle_bn1, 0);
    stubSuccessResponseFor(bundle_bn2, 1);

    // since we want to form a queue of bundle to form, we just wait before executing the send
    // bundle task that all the expected tasks are queued
    executor.waitForQueueTaskCount(2, true);

    // we submit bundle_bn2, bundle_bn1 in this order
    // we expect they are sent in this order: bundle_bn1, bundle_bn2
    // but bundle_bn1 timeout at first attempt and is retried after bundle_bn2
    bundleForwarder.onTransactionBundleAdded(bundle_bn2);
    bundleForwarder.onTransactionBundleAdded(bundle_bn1);

    final var isFirstExecution = new AtomicBoolean(false);
    executor.addAfterExecuteListener(
        (r, t) -> {
          // we expect bundle_bn1 to be processed first and retried so we can wait for the task
          // queue to be filled again, but this time bundle_bn2 should have priority over bundle_bn1
          if (isFirstExecution.compareAndSet(false, true)) {
            // change the response to success at the second attempt
            stubSuccessResponseFor(bundle_bn1, 2);

            try {
              executor.waitForQueueTaskCount(2, false);
            } catch (ExecutionException | InterruptedException e) {
              throw new RuntimeException(e);
            }
          }
        });

    final var allEvents = getAllServeEvents();

    allEvents.forEach(event -> System.out.println(event));

    // reqId ensures the correct order
    verifyRequestSentFor(bundle_bn1, 0);
    responseCollector.assertFailedResponse(0, InterruptedIOException.class);
    verifyRequestSentFor(bundle_bn2, 1);
    responseCollector.assertSuccessResponse(1);
    verifyRequestSentFor(bundle_bn1, 2, 1);
    responseCollector.assertSuccessResponse(2);
  }

  @Test
  void cancelledBundleIsNotForwarded() throws InterruptedException {
    final var bundle =
        createBundle(
            Hash.fromHexStringLenient("0x9abc"), CHAIN_HEAD_BLOCK_NUMBER + 1, List.of(TX1, TX3));

    // since we want that the bundle is not forwarded before we have a chance to remove it
    // we pause the execution
    final var semaphore = executor.pauseExecution();

    bundleForwarder.onTransactionBundleAdded(bundle);
    assertThat(executor.getQueue()).map(this::extractBundle).containsExactly(bundle);

    bundleForwarder.onTransactionBundleRemoved(bundle);
    assertThat(executor.getQueue()).isEmpty();

    // resume execution
    semaphore.release();

    executor.executeSomething();

    // expect that both internal tasks are completed
    await().until(() -> executor.getCompletedTaskCount() == 2);

    // ensure no rpc call was sent
    assertThat(getAllServeEvents()).isEmpty();
  }

  private static String getExpectedRequest(final TransactionBundle bundle, final long reqId)
      throws JsonProcessingException {
    final var expectedRequest =
        """
        {
          "jsonrpc": "2.0",
          "id": <reqId>,
          "method": "linea_sendBundle",
          "params": [<params>]
        }
        """
            .replace("<params>", OBJECT_MAPPER.writeValueAsString(bundle.toBundleParameter(false)))
            .replace("<reqId>", String.valueOf(reqId));
    return expectedRequest;
  }

  private static void stubSuccessResponseFor(final TransactionBundle bundle, final long reqId) {
    stubFor(
        post(urlEqualTo("/"))
            .withHeader("Content-Type", equalTo("application/json; charset=UTF-8"))
            .withRequestBody(matchingBlockNumberAndReqId(bundle.blockNumber(), reqId))
            .willReturn(
                aResponse()
                    .withStatus(200)
                    .withHeader("Content-Type", "application/json")
                    .withBody(
                        """
                            {
                              "jsonrpc": "2.0",
                              "result": {
                                "bundleHash": "<bundleHash>"
                              },
                              "id": <reqId>
                            }"""
                            .replace("<bundleHash>", bundle.bundleIdentifier().toHexString())
                            .replace("<reqId>", String.valueOf(reqId)))));
  }

  private static void stubTimeoutResponseFor(final TransactionBundle bundle, final long reqId) {
    stubFor(
        post(urlEqualTo("/"))
            .withHeader("Content-Type", equalTo("application/json; charset=UTF-8"))
            .withRequestBody(matchingBlockNumberAndReqId(bundle.blockNumber(), reqId))
            .willReturn(aResponse().withFixedDelay(10_000)));
  }

  private static StringValuePattern matchingBlockNumberAndReqId(
      final long blockNumber, final long reqId) {
    return matchingJsonPath("$.params[?(@.blockNumber == %d)]".formatted(blockNumber))
        .and(matchingJsonPath("$[?(@.id == %d)]".formatted(reqId)));
  }

  private static void verifyRequestSentFor(final TransactionBundle bundle, final long reqId)
      throws JsonProcessingException {
    verifyRequestSentFor(bundle, reqId, 0);
  }

  private static void verifyRequestSentFor(
      final TransactionBundle bundle, final long reqId, final int retryCount)
      throws JsonProcessingException {
    final var expectedRequest = getExpectedRequest(bundle, reqId);

    final var patterBuilder =
        postRequestedFor(urlEqualTo("/"))
            .withHeader("Content-Type", equalTo("application/json; charset=UTF-8"))
            .withRequestBody(equalToJson(expectedRequest));

    if (retryCount > 0) {
      patterBuilder.withHeader(RETRY_COUNT_HEADER, equalTo(String.valueOf(retryCount)));
    } else {
      patterBuilder.withoutHeader(RETRY_COUNT_HEADER);
    }

    await().atMost(2, SECONDS).untilAsserted(() -> verify(exactly(1), patterBuilder));
  }

  @SuppressWarnings("unchecked")
  private PriorityThreadPoolExecutor.PriorityFuture<SendBundleResponse> extractPriorityFuture(
      final Runnable r) {
    return (PriorityThreadPoolExecutor.PriorityFuture<SendBundleResponse>) r;
  }

  private BundleForwarder.SendBundleTask extractSendBundleTask(final Runnable r) {
    return (BundleForwarder.SendBundleTask) extractPriorityFuture(r).getSourceTask();
  }

  private TransactionBundle extractBundle(final Runnable r) {
    return extractSendBundleTask(r).getBundle();
  }

  private class ResponseCollector
      implements TestablePriorityThreadPoolExecutor.AfterExecuteListener {
    private final Map<Long, SendBundleResponse> responses = new HashMap<>();
    private final Map<Long, Throwable> failures = new HashMap<>();
    private final Set<TransactionBundle> skipped = new HashSet<>();

    @Override
    public void onAfterExecute(final Runnable r, final Throwable t) {
      try {
        final var response = extractPriorityFuture(r).get();
        responses.put(response.reqId(), response);
      } catch (InterruptedException | ExecutionException e) {
        if (e.getCause() instanceof BundleForwarder.BundleForwarderException bfe) {
          bfe.reqId()
              .ifPresentOrElse(reqId -> failures.put(reqId, e), () -> skipped.add(bfe.bundle()));
        } else {
          fail(e);
        }
      }
    }

    public void assertSuccessResponse(final long reqId) throws IOException {
      await().until(() -> responses.containsKey(reqId) || failures.containsKey(reqId));
      if (failures.containsKey(reqId)) {
        fail(failures.get(reqId));
      }

      final var response = responses.get(reqId);
      assertTrue(response.response().isSuccessful());
      assertThat(getReqId(response)).isEqualTo(reqId);
      assertThat(getBundleHash(response))
          .isEqualTo(response.bundle().bundleIdentifier().toHexString());
    }

    public void assertFailedResponse(
        final long reqId, final Class<? extends Throwable> expectedException) {
      await().until(() -> responses.containsKey(reqId) || failures.containsKey(reqId));

      if (responses.containsKey(reqId)) {
        assertFalse(responses.get(reqId).response().isSuccessful());
      }

      assertThat(failures.get(reqId).getCause().getCause()).isInstanceOf(expectedException);
    }

    public void assertSkippedBundle(final TransactionBundle bundle) {
      await().until(() -> skipped.contains(bundle));
    }

    private String getBundleHash(final SendBundleResponse response) throws JsonProcessingException {
      return OBJECT_MAPPER.readTree(response.body()).findValue("bundleHash").asText();
    }

    private long getReqId(final SendBundleResponse response) throws JsonProcessingException {
      return OBJECT_MAPPER.readTree(response.body()).get("id").asLong();
    }
  }
}
