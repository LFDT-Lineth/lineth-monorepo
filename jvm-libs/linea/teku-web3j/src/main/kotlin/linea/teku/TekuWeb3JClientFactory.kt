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
import okhttp3.OkHttpClient
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.web3j.protocol.Web3j
import org.web3j.protocol.Web3jService
import org.web3j.protocol.http.HttpService
import tech.pegasys.teku.ethereum.executionclient.auth.JwtAuthHttpInterceptor
import tech.pegasys.teku.ethereum.executionclient.auth.JwtConfig
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient
import tech.pegasys.teku.infrastructure.logging.EventLogger
import tech.pegasys.teku.infrastructure.time.SystemTimeProvider
import java.net.URL
import java.util.Optional
import java.util.UUID
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
    nonCriticalMethods: Set<String> = emptySet(),
  ): Web3JClient {
    val okHttpClient: OkHttpClient =
      okHttpClientBuilder(
        logger = log,
        requestResponseLogLevel = requestResponseLogLevel,
        failuresLogLevel = failuresLogLevel,
      ).callTimeout(timeout.toJavaDuration())
        .readTimeout(timeout.toJavaDuration())
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

    val httpService: Web3jService = Jackson2HttpService(endpoint.toString(), okHttpClient)
    // Same transport/auth (shares okHttpClient) as httpService above, but web3j's own vanilla
    // HttpService, so its default Jackson 3 mapper resolves web3j-native response types (e.g.
    // EthBlock) correctly. See Web3jClient's doc for why httpService alone can't do this.
    val eth1Web3j: Web3j = Web3j.build(HttpService(endpoint.toString(), okHttpClient))
    val web3jClient =
      Web3jClient(
        eventLogger,
        web3jService = httpService,
        timeProvider = SystemTimeProvider.SYSTEM_TIME_PROVIDER,
        executionClientEventsPublisher = { elIsUp ->
          log.info("client {} is {}", endpoint, if (elIsUp) "up" else "down")
        },
        eth1Web3j = eth1Web3j,
        nonCriticalMethods = nonCriticalMethods,
      )
    return web3jClient
  }
}
