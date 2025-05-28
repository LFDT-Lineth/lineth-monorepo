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
package net.consensys.linea.jsonrpc;

import static com.github.tomakehurst.wiremock.client.WireMock.aResponse;
import static com.github.tomakehurst.wiremock.client.WireMock.exactly;
import static com.github.tomakehurst.wiremock.client.WireMock.post;
import static com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor;
import static com.github.tomakehurst.wiremock.client.WireMock.stubFor;
import static com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo;
import static com.github.tomakehurst.wiremock.client.WireMock.verify;
import static java.util.concurrent.TimeUnit.SECONDS;
import static org.awaitility.Awaitility.await;
import static org.mockito.Mockito.when;

import java.io.IOException;
import java.net.URI;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.List;
import java.util.Optional;

import com.github.tomakehurst.wiremock.junit5.WireMockRuntimeInfo;
import com.github.tomakehurst.wiremock.junit5.WireMockTest;
import net.consensys.linea.config.LineaNodeType;
import net.consensys.linea.config.LineaRejectedTxReportingConfiguration;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.data.TransactionSelectionResult;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.api.io.TempDir;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

@WireMockTest
@ExtendWith(MockitoExtension.class)
public class JsonRpcManagerStartTest {
  @TempDir private Path tempDataDir;
  private JsonRpcManager jsonRpcManager;
  private final Bytes randomEncodedBytes = Bytes.random(32);
  @Mock private Transaction transaction;
  static final String PLUGIN_IDENTIFIER = "linea-start-test-plugin";

  @BeforeEach
  void init(final WireMockRuntimeInfo wmInfo) throws IOException {
    // create temp directories
    final Path jsonRpcDir =
        tempDataDir.resolve(JsonRpcManager.JSON_RPC_DIR).resolve(PLUGIN_IDENTIFIER);
    Files.createDirectories(jsonRpcDir);

    // mock stubbing
    when(transaction.encoded()).thenReturn(randomEncodedBytes);

    // save rejected transaction in tempDataDir so that they are processed by the
    // JsonRpcManager.start
    for (int i = 0; i < 3; i++) {
      final TransactionSelectionResult result = TransactionSelectionResult.invalid("test" + i);
      final Instant timestamp = Instant.now();
      final String jsonRpcCall =
          JsonRpcRequestBuilder.generateSaveRejectedTxJsonRpc(
              LineaNodeType.SEQUENCER,
              transaction,
              timestamp,
              Optional.of(1L),
              result.toString(),
              List.of());

      JsonRpcManager.saveJsonToDir(jsonRpcCall, jsonRpcDir);
    }

    final LineaRejectedTxReportingConfiguration config =
        LineaRejectedTxReportingConfiguration.builder()
            .rejectedTxEndpoint(URI.create(wmInfo.getHttpBaseUrl()).toURL())
            .lineaNodeType(LineaNodeType.SEQUENCER)
            .build();
    jsonRpcManager = new JsonRpcManager(PLUGIN_IDENTIFIER, tempDataDir, config);
  }

  @AfterEach
  void cleanup() {
    jsonRpcManager.shutdown();
  }

  @Test
  void existingJsonRpcFilesAreProcessedOnStart() {
    stubFor(
        post(urlEqualTo("/"))
            .willReturn(
                aResponse()
                    .withStatus(200)
                    .withHeader("Content-Type", "application/json")
                    .withBody(
                        "{\"jsonrpc\":\"2.0\",\"result\":{ \"status\": \"SAVED\"},\"id\":1}")));
    // method under test
    jsonRpcManager.start();

    // Use Awaitility to wait for the condition to be met
    await()
        .atMost(2, SECONDS)
        .untilAsserted(() -> verify(exactly(3), postRequestedFor(urlEqualTo("/"))));
  }
}
