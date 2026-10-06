/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.teku

import linea.web3j.okhttp.okHttpClientBuilder
import okhttp3.Call
import okhttp3.EventListener
import okhttp3.OkHttpClient
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.ethereum.executionclient.ExecutionEngineClientFactory
import tech.pegasys.teku.ethereum.executionclient.auth.JwtAuthHttpInterceptor
import tech.pegasys.teku.ethereum.executionclient.auth.JwtConfig
import tech.pegasys.teku.infrastructure.logging.EventLogger
import tech.pegasys.teku.infrastructure.time.SystemTimeProvider
import java.net.URL
import java.util.Optional
import java.util.UUID
import java.util.concurrent.TimeUnit
import kotlin.io.path.Path
import kotlin.time.Duration
import kotlin.time.Duration.Companion.minutes
import kotlin.time.toJavaDuration

object JwtHelper {
  fun loadOrGenerate(jwtPath: String): JwtConfig {
    val jwtConfigPath = Optional.ofNullable(jwtPath)
    return JwtConfig
      .createIfNeeded(
        /* needed = */
        true,
        jwtConfigPath,
        Optional.of(UUID.randomUUID().toString()),
        Path("/dev/null"), // Teku's API limitation. Would be good to clean it
      ).get()
  }
}

object TekuWeb3JClientFactory {
  val defaultRequestResponseLogLevel: Level = Level.TRACE
  val defaultFailedRequestResponseLogLevel: Level = Level.DEBUG
  val eventLogger: EventLogger = EventLogger.EVENT_LOG // "teku-event-log"

  fun create(
    endpoint: URL,
    jwtPath: String? = null,
    timeout: Duration = 1.minutes,
    log: Logger = LogManager.getLogger("clients.web3j"),
    requestResponseLogLevel: Level = defaultRequestResponseLogLevel,
    failuresLogLevel: Level = defaultFailedRequestResponseLogLevel,
  ): Web3JClient {
    val okHttpClient: OkHttpClient =
      okHttpClientBuilder(
        logger = log,
        requestResponseLogLevel = requestResponseLogLevel,
        failuresLogLevel = failuresLogLevel,
      ).callTimeout(timeout.toJavaDuration())
        .readTimeout(timeout.toJavaDuration())
        .eventListener(object : EventListener() {
          override fun callStart(call: Call) {
            // Set the limit before Teku's asynchronous call enters OkHttp's timeout watchdog.
            if (timeout.isPositive()) {
              val callTimeout = call.timeout()
              callTimeout.timeout(minOf(callTimeout.timeoutNanos(), timeout.inWholeNanoseconds), TimeUnit.NANOSECONDS)
            }
          }
        })
        .apply {
          jwtPath?.let {
            addInterceptor(
              JwtAuthHttpInterceptor(
                /* jwtConfig = */
                JwtHelper.loadOrGenerate(jwtPath),
                /* timeProvider = */
                SystemTimeProvider.SYSTEM_TIME_PROVIDER,
              ),
            )
          }
        }.build()

    val engineClient = ExecutionEngineClientFactory.create(
      endpoint.toString(),
      SystemTimeProvider.SYSTEM_TIME_PROVIDER,
      eventLogger,
      { elIsUp -> log.info("client {} is {}", endpoint, if (elIsUp) "up" else "down") },
      { okHttpClient },
      { throw UnsupportedOperationException("IPC transport is not supported by this HTTP client factory") },
    )
    return Web3JClient(endpoint.toString(), okHttpClient, engineClient)
  }
}
