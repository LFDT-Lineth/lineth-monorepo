/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import io.vertx.core.Vertx
import linea.teku.TekuWeb3JClientFactory
import lineth.vertx.vertxTestOptions
import maru.consensus.ElFork
import maru.core.ext.metrics.TestMetrics.TestMetricsFacade
import maru.executionlayer.ExecutionLayerFactory
import maru.executionlayer.client.EngineApiTestData.executionPayload
import maru.executionlayer.client.EngineApiTestData.finalizedBlockHash
import maru.executionlayer.client.EngineApiTestData.getPayloadResultJson
import maru.executionlayer.client.EngineApiTestData.headBlockHash
import maru.executionlayer.client.EngineApiTestData.payloadAttributes
import maru.executionlayer.client.EngineApiTestData.payloadId
import maru.executionlayer.client.EngineApiTestData.safeBlockHash
import maru.executionlayer.mappers.Mappers.toDomain
import net.consensys.linea.async.get
import org.apache.logging.log4j.LogManager
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.EnumSource
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceStateV1
import tech.pegasys.teku.infrastructure.bytes.Bytes8

/**
 * Checks that the native client decodes execution-layer responses exactly like the Teku/Web3J client it replaces.
 * Delete together with the Teku/Web3J client.
 */
class TekuEngineApiClientParityTest {
  private lateinit var vertx: Vertx
  private lateinit var server: RecordingJsonRpcServer

  @BeforeEach
  fun setUp() {
    vertx = Vertx.vertx(vertxTestOptions)
    server = RecordingJsonRpcServer()
  }

  @AfterEach
  fun tearDown() {
    server.close()
    vertx.close().get()
  }

  @ParameterizedTest
  @EnumSource(ElFork::class)
  fun `native client decodes responses like the Teku client`(fork: ElFork) {
    val methods = EngineApiForkMethods.of(fork)
    val payloadStatus =
      mapOf("status" to "INVALID", "latestValidHash" to "0x" + "ab".repeat(32), "validationError" to "bad")
    server.replyResult(methods.newPayload, payloadStatus)
    server.replyResult(
      methods.forkChoiceUpdated,
      mapOf(
        "payloadStatus" to mapOf("status" to "VALID", "latestValidHash" to "0x" + "cd".repeat(32)),
        "payloadId" to "0x0102030405060708",
      ),
    )
    server.replyResult(methods.getPayload, getPayloadResultJson(fork))
    server.replyResult("eth_getBlockByNumber", tekuDecodableBlock())

    val web3jClient = TekuWeb3JClientFactory.create(server.endpoint)
    try {
      val tekuClient = ExecutionLayerFactory.buildExecutionEngineClient(web3jClient, fork, TestMetricsFacade)
      val nativeClient =
        JsonRpcEngineApiClient(
          rpcClient =
          EngineApiRpcClientFactory.create(
            vertx = vertx,
            metricsFacade = TestMetricsFacade,
            endpoint = server.endpoint,
            jwtSecretPath = null,
            timeoutOverride = null,
            log = LogManager.getLogger(TekuEngineApiClientParityTest::class.java),
          ),
          fork = fork,
          endpoint = server.endpoint,
          metricsFacade = TestMetricsFacade,
        )
      val forkChoiceState =
        ForkChoiceStateV1(Bytes32.wrap(headBlockHash), Bytes32.wrap(safeBlockHash), Bytes32.wrap(finalizedBlockHash))

      assertThat(nativeClient.getPayload(payloadId).get())
        .isEqualTo(tekuClient.getPayload(Bytes8(Bytes.wrap(payloadId))).get().payload)
        .isEqualTo(executionPayload(fork))

      assertThat(nativeClient.newPayload(executionPayload(fork)).get())
        .isEqualTo(tekuClient.newPayload(executionPayload(fork)).get().payload.asInternalExecutionPayload().toDomain())

      assertThat(
        nativeClient.forkChoiceUpdate(headBlockHash, safeBlockHash, finalizedBlockHash, payloadAttributes(fork)).get(),
      ).isEqualTo(tekuClient.forkChoiceUpdate(forkChoiceState, payloadAttributes(fork)).get().payload.toDomain())

      val nativeLatest = nativeClient.getLatestBlockMetadata().get()
      val tekuLatest = tekuClient.getLatestBlockMetadata().get()
      assertThat(nativeLatest.blockHash).isEqualTo(tekuLatest.blockHash)
      assertThat(nativeLatest.timestamp).isEqualTo(tekuLatest.timestamp)
    } finally {
      web3jClient.close()
    }
  }

  private fun tekuDecodableBlock(): Map<String, Any?> =
    mapOf(
      "hash" to "0x" + "ef".repeat(32),
      "parentHash" to "0x" + "01".repeat(32),
      "number" to "0x64",
      "timestamp" to "0x6553f100",
      "totalDifficulty" to "0x0",
      "difficulty" to "0x0",
    )
}
