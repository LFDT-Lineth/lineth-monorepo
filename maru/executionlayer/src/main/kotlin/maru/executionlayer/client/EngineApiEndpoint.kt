/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import io.vertx.core.Vertx
import maru.consensus.ElFork
import net.consensys.linea.jsonrpc.client.JsonRpcV2Client
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.Logger
import java.net.URL
import java.nio.file.Path
import java.util.concurrent.ConcurrentHashMap
import kotlin.time.Duration

/** One execution-layer Engine API endpoint. The [EngineApiClient] of every fork shares its JSON-RPC transport. */
class EngineApiEndpoint(
  val endpoint: URL,
  private val rpcClient: JsonRpcV2Client,
  private val metricsFacade: MetricsFacade,
) {
  private val clients = ConcurrentHashMap<ElFork, EngineApiClient>()

  fun clientFor(fork: ElFork): EngineApiClient =
    clients.computeIfAbsent(fork) { JsonRpcEngineApiClient(rpcClient, it, endpoint, metricsFacade) }

  companion object {
    /** Creates the endpoint; its HTTP client is released when [vertx] is closed. */
    fun create(
      vertx: Vertx,
      metricsFacade: MetricsFacade,
      endpoint: URL,
      jwtSecretPath: Path?,
      timeoutOverride: Duration?,
      log: Logger,
    ): EngineApiEndpoint =
      EngineApiEndpoint(
        endpoint = endpoint,
        rpcClient = EngineApiRpcClientFactory.create(
          vertx,
          metricsFacade,
          endpoint,
          jwtSecretPath,
          timeoutOverride,
          log,
        ),
        metricsFacade = metricsFacade,
      )
  }
}
