/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.e2e

import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import maru.config.ApiEndpointConfig
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.web3j.protocol.Web3j
import tech.pegasys.teku.ethereum.executionclient.auth.JwtConfig
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3jClientBuilder
import tech.pegasys.teku.infrastructure.time.SystemTimeProvider
import java.net.URI
import java.util.Optional
import java.util.UUID
import kotlin.io.path.Path
import kotlin.time.Duration.Companion.minutes
import kotlin.time.toJavaDuration

object TestEnvironment {
  const val JWT_CONFIG_PATH = "../docker/jwt"
  val testMetricsFacade =
    MicrometerMetricsFacade(
      registry = SimpleMeterRegistry(),
      metricsPrefix = "maru.e2e",
    )
  private val jwtConfig: Optional<JwtConfig> =
    JwtConfig.createIfNeeded(
      true,
      Optional.of(JWT_CONFIG_PATH),
      Optional.of(UUID.randomUUID().toString()),
      Path("/tmp"),
    )
  val sequencerL2Client: Web3j = buildWeb3Client("http://localhost:8545")

  // The switch doesn't work for Geth 1.15 yet
  private val geth1L2Client: Web3j = buildWeb3Client("http://localhost:8555")
  private val geth2L2Client: Web3j = buildWeb3Client("http://localhost:8565")
  private val besuFollowerL2Client: Web3j = buildWeb3Client("http://localhost:9545")

  // The switch doesn't work for nethermind yet
  private val nethermindFollowerL2Client: Web3j = buildWeb3Client("http://localhost:10545", jwtConfig)
  private val erigonFollowerL2Client: Web3j = buildWeb3Client("http://localhost:11545")
  val preMergeFollowerClients =
    mapOf(
      "follower-geth-2" to geth2L2Client,
      "follower-besu" to besuFollowerL2Client,
      "follower-erigon" to erigonFollowerL2Client,
      "follower-nethermind" to nethermindFollowerL2Client,
    )
  val clientsSyncablePreMergeAndPostMerge =
    mapOf(
      "follower-besu" to besuFollowerL2Client,
      "follower-erigon" to erigonFollowerL2Client,
      "follower-nethermind" to nethermindFollowerL2Client,
    )
  val followerClientsPostMerge = preMergeFollowerClients - "follower-geth-2" + ("follower-geth" to geth1L2Client)
  val allClients = preMergeFollowerClients + followerClientsPostMerge + ("sequencer" to sequencerL2Client)

  private val besuFollowerExecutionEngineClient = createExecutionClientConfig("http://localhost:9550")
  private val nethermindFollowerExecutionEngineClient =
    createExecutionClientConfig("http://localhost:10550", JWT_CONFIG_PATH)
  private val erigonFollowerExecutionEngineClient =
    createExecutionClientConfig("http://localhost:11551", JWT_CONFIG_PATH)
  private val geth1ExecutionEngineClient = createExecutionClientConfig("http://localhost:8561", JWT_CONFIG_PATH)
  private val geth2ExecutionEngineClient = createExecutionClientConfig("http://localhost:8571", JWT_CONFIG_PATH)
  private val gethExecutionEngineClients =
    mapOf(
      "follower-geth-2" to geth2ExecutionEngineClient,
    )
  private val followerExecutionEngineClients =
    mapOf(
      "follower-besu" to besuFollowerExecutionEngineClient,
      "follower-erigon" to erigonFollowerExecutionEngineClient,
      "follower-nethermind" to nethermindFollowerExecutionEngineClient,
    ) + gethExecutionEngineClients

  val followerExecutionClientsPostMerge =
    (
      mapOf("follower-geth" to geth1ExecutionEngineClient) +
        (followerExecutionEngineClients - "follower-geth-2" - "follower-nethermind")
      )

  private fun buildWeb3Client(
    rpcUrl: String,
    jwtConfig: Optional<JwtConfig> = Optional.empty(),
  ): Web3j = createWeb3jClient(rpcUrl, jwtConfig).eth1Web3j

  private fun createWeb3jClient(
    endpoint: String,
    jwtConfig: Optional<JwtConfig>,
  ): Web3JClient =
    Web3jClientBuilder()
      .timeout(1.minutes.toJavaDuration())
      .endpoint(endpoint)
      .jwtConfigOpt(jwtConfig)
      .timeProvider(SystemTimeProvider.SYSTEM_TIME_PROVIDER)
      .executionClientEventsPublisher {}
      .build()

  private fun createExecutionClientConfig(
    eeEndpoint: String,
    jwtConfigPath: String? = null,
  ): ApiEndpointConfig = ApiEndpointConfig(URI.create(eeEndpoint).toURL(), jwtConfigPath)
}
