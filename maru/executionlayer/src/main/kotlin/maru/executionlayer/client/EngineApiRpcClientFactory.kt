/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import com.github.michaelbull.result.Result
import io.vertx.core.Vertx
import net.consensys.linea.jsonrpc.client.JsonRpcV2Client
import net.consensys.linea.jsonrpc.client.RequestRetryConfig
import net.consensys.linea.jsonrpc.client.VertxHttpJsonRpcClientFactory
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.Logger
import java.net.URL
import java.nio.file.Path
import java.time.Clock
import java.util.function.Predicate
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

/**
 * Builds the JSON-RPC transport for one execution-layer Engine API endpoint, to be shared by the [JsonRpcEngineApiClient]
 * of every fork. Requests are not retried, so callers keep control of retry policy. The underlying HTTP client is
 * released when [vertx] is closed.
 */
object EngineApiRpcClientFactory {
  fun create(
    vertx: Vertx,
    metricsFacade: MetricsFacade,
    endpoint: URL,
    jwtSecretPath: Path?,
    requestTimeouts: (method: String) -> Duration,
    log: Logger,
    clock: Clock = Clock.systemUTC(),
  ): JsonRpcV2Client {
    val jwtTokenProvider = jwtSecretPath?.let { EngineApiJwtTokenProvider.fromSecretFile(it, clock) }
    return VertxHttpJsonRpcClientFactory(vertx, metricsFacade).createJsonRpcV2Client(
      endpoints = listOf(endpoint.toURI()),
      retryConfig = RequestRetryConfig(backoffDelay = 1.seconds),
      requestObjectMapper = engineApiObjectMapper,
      shallRetryRequestsClientBasePredicate = Predicate<Result<Any?, Throwable>> { false },
      log = log,
      requestHeadersProvider = { jwtTokenProvider?.authorizationHeader() ?: emptyMap() },
      requestOverallTimeoutProvider = requestTimeouts,
    )
  }
}
