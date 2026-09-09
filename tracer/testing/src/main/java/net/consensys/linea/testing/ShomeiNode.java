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
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.testing;

import com.fasterxml.jackson.databind.node.ArrayNode;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.io.UncheckedIOException;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.concurrent.Executors;
import lombok.extern.slf4j.Slf4j;

/**
 * A minimal in-process stand-in for a Shomei JSON-RPC node, used so the empty-block Besu tests do
 * not need the real {@code io.consensys.protocols.shomei:shomei-server} (whose Vert.x 4-era HTTP
 * server fails to start under the Vert.x 5 runtime Besu 26.8.1 brings).
 *
 * <p>These tests only ever call shomei to <em>record</em> an execution-proof request — the merkle
 * proof content is written to the traces dir and never read back or asserted — and the Besu node
 * forwards trielogs via {@code rollup_sendRawTrieLog}. Neither response is content-checked, so a
 * stub that binds the port and returns well-formed JSON-RPC results is observably equivalent.
 */
@Slf4j
public class ShomeiNode implements AutoCloseable, Runnable {

  public record MerkelProofResponse(
      String zkParentStateRootHash,
      String zkEndStateRootHash,
      ArrayNode zkStateMerkleProof,
      String zkStateManagerVersion) {}

  private static final String PROOF_RESULT =
      """
      {
        "zkParentStateRootHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "zkEndStateRootHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "zkStateMerkleProof": [],
        "zkStateManagerVersion": "test"
      }
      """;

  private final int jsonRpcPort;
  private final String jsonRpcUrl;
  private HttpServer server;

  public ShomeiNode(int jsonRpcPort) {
    this.jsonRpcPort = jsonRpcPort;
    this.jsonRpcUrl = "http://127.0.0.1:" + jsonRpcPort;
  }

  public String getJsonRpcUrl() {
    return this.jsonRpcUrl;
  }

  @Override
  public void run() {
    try {
      server = HttpServer.create(new InetSocketAddress("127.0.0.1", jsonRpcPort), 0);
      server.createContext("/", this::handleJsonRpc);
      server.setExecutor(Executors.newSingleThreadExecutor());
      server.start();
      log.info("Shomei stub listening on {}", jsonRpcUrl);
    } catch (IOException e) {
      throw new UncheckedIOException("Failed to start Shomei stub on port " + jsonRpcPort, e);
    }
  }

  private void handleJsonRpc(final HttpExchange exchange) throws IOException {
    final String request =
        new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
    final String result = request.contains("rollup_sendRawTrieLog") ? "true" : PROOF_RESULT;
    final byte[] body =
        ("{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":" + result + "}")
            .getBytes(StandardCharsets.UTF_8);
    exchange.getResponseHeaders().set("Content-Type", "application/json");
    exchange.sendResponseHeaders(200, body.length);
    try (var os = exchange.getResponseBody()) {
      os.write(body);
    }
  }

  @Override
  public void close() {
    if (server != null) {
      server.stop(0);
    }
  }

  public static class Builder {
    private Integer jsonRpcPort;

    public Builder setDataStoragePath(Path dataStoragePath) {
      return this; // no-op: the stub keeps no state
    }

    public Builder setJsonRpcPort(Integer port) {
      this.jsonRpcPort = port;
      return this;
    }

    public Builder setBesuRpcPort(Integer port) {
      return this; // no-op: the stub never dials Besu
    }

    public ShomeiNode build() {
      if (jsonRpcPort == null) {
        throw new IllegalStateException("jsonRpcPort is required");
      }
      return new ShomeiNode(jsonRpcPort);
    }
  }

  public record GetZkEVMStateMerkleProofResponse(
      ArrayNode zkStateMerkleProof,
      byte[] zkParentStateRootHash,
      byte[] zkEndStateRootHash,
      String zkStateManagerVersion) {
    public static GetZkEVMStateMerkleProofResponse fromJson(String json) {
      return null; // Placeholder, unused by the tests
    }
  }

  /**
   * Parameters for {@code rollup_getZkEVMStateMerkleProofV0}, local to the test harness so it does
   * not need {@code shomei-server}'s {@code
   * net.consensys.shomei.rpc.server.model.RollupGetZkEvmStateV0Parameter} (three string fields).
   */
  public record RollupGetZkEvmStateV0Parameter(
      String startBlockNumber, String endBlockNumber, String zkStateManagerVersion) {}
}
