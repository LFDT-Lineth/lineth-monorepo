/*
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.executionlayer.client

import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.node.NullNode
import com.fasterxml.jackson.databind.node.ObjectNode
import com.fasterxml.jackson.databind.node.TextNode
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import io.vertx.core.Vertx
import lineth.vertx.vertxTestOptions
import maru.consensus.ElFork
import maru.executionlayer.client.EngineApiTestData.executionPayload
import maru.executionlayer.client.EngineApiTestData.executionPayloadJson
import maru.executionlayer.client.EngineApiTestData.finalizedBlockHash
import maru.executionlayer.client.EngineApiTestData.forkChoiceUpdatedRequest
import maru.executionlayer.client.EngineApiTestData.getPayloadRequest
import maru.executionlayer.client.EngineApiTestData.getPayloadResultJson
import maru.executionlayer.client.EngineApiTestData.headBlockHash
import maru.executionlayer.client.EngineApiTestData.latestBlockRequest
import maru.executionlayer.client.EngineApiTestData.newPayloadRequest
import maru.executionlayer.client.EngineApiTestData.payloadAttributes
import maru.executionlayer.client.EngineApiTestData.payloadId
import maru.executionlayer.client.EngineApiTestData.safeBlockHash
import maru.executionlayer.client.EngineApiTestData.withoutId
import maru.executionlayer.manager.ExecutionPayloadStatus
import maru.executionlayer.manager.ForkChoiceUpdatedResult
import maru.executionlayer.manager.LatestBlockMetadata
import maru.executionlayer.manager.PayloadStatus
import net.consensys.linea.async.get
import net.consensys.linea.jsonrpc.JsonRpcErrorResponseException
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import org.apache.logging.log4j.LogManager
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.EnumSource
import org.junit.jupiter.params.provider.MethodSource
import java.net.ServerSocket
import java.net.URI
import java.nio.file.Files
import java.time.Clock
import java.time.Instant
import java.time.ZoneOffset
import java.util.Base64
import java.util.concurrent.ExecutionException
import java.util.concurrent.TimeUnit
import java.util.concurrent.TimeoutException
import javax.crypto.Mac
import javax.crypto.spec.SecretKeySpec
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class JsonRpcEngineApiClientTest {
  private lateinit var vertx: Vertx
  private lateinit var server: RecordingJsonRpcServer
  private lateinit var meterRegistry: SimpleMeterRegistry

  @BeforeEach
  fun setUp() {
    vertx = Vertx.vertx(vertxTestOptions)
    server = RecordingJsonRpcServer()
    meterRegistry = SimpleMeterRegistry()
  }

  @AfterEach
  fun tearDown() {
    server.close()
    vertx.close().get()
  }

  private fun client(
    fork: ElFork,
    endpoint: java.net.URL = server.endpoint,
    jwtSecretPath: java.nio.file.Path? = null,
    timeoutOverride: Duration? = 30.seconds,
    clock: Clock = Clock.systemUTC(),
  ): JsonRpcEngineApiClient {
    val metricsFacade = MicrometerMetricsFacade(meterRegistry)
    val rpcClient =
      EngineApiRpcClientFactory.create(
        vertx = vertx,
        metricsFacade = metricsFacade,
        endpoint = endpoint,
        jwtSecretPath = jwtSecretPath,
        timeoutOverride = timeoutOverride,
        log = LogManager.getLogger(JsonRpcEngineApiClientTest::class.java),
        clock = clock,
      )
    return JsonRpcEngineApiClient(rpcClient, fork, endpoint, metricsFacade)
  }

  private val validStatus = mapOf("status" to "VALID", "latestValidHash" to "0x" + "ab".repeat(32))

  @ParameterizedTest
  @EnumSource(ElFork::class)
  fun `requests conform to the Engine API spec and carry the expected values`(fork: ElFork) {
    val methods = EngineApiForkMethods.of(fork)
    server.replyResult(methods.newPayload, validStatus)
    server.replyResult(
      methods.forkChoiceUpdated,
      mapOf("payloadStatus" to validStatus, "payloadId" to "0x" + "01".repeat(8)),
    )
    server.replyResult(methods.getPayload, getPayloadResultJson(fork))
    server.replyResult("eth_getBlockByNumber", mapOf("hash" to "0x" + "cd".repeat(32), "timestamp" to "0x10"))
    val client = client(fork)

    client.newPayload(executionPayload(fork)).get()
    client.forkChoiceUpdate(headBlockHash, safeBlockHash, finalizedBlockHash, null).get()
    client.forkChoiceUpdate(headBlockHash, safeBlockHash, finalizedBlockHash, payloadAttributes(fork)).get()
    client.getPayload(payloadId).get()
    client.getLatestBlockMetadata().get()

    assertThat(server.requests.map { withoutId(it.body) })
      .containsExactly(
        newPayloadRequest(fork),
        forkChoiceUpdatedRequest(fork, withAttributes = false),
        forkChoiceUpdatedRequest(fork, withAttributes = true),
        getPayloadRequest(fork),
        latestBlockRequest(),
      )
    server.requests.filter { it.method.startsWith("engine_") }.forEach { request ->
      assertThat(EngineApiSpec.paramsViolations(request.method, request.body["params"]))
        .describedAs(request.method)
        .isEmpty()
    }
    assertThat(server.requests).allSatisfy { assertThat(it.authorization).isNull() }
  }

  @ParameterizedTest
  @EnumSource(ElFork::class)
  fun `getPayload decodes the payload the execution layer built`(fork: ElFork) {
    val method = EngineApiForkMethods.of(fork).getPayload
    assertThat(EngineApiSpec.resultViolations(method, getPayloadResultJson(fork))).isEmpty()
    server.replyResult(method, getPayloadResultJson(fork))

    assertThat(client(fork).getPayload(payloadId).get()).isEqualTo(executionPayload(fork))
  }

  @Test
  fun `newPayload decodes every payload status`() {
    val client = client(ElFork.Prague)
    val method = EngineApiForkMethods.of(ElFork.Prague).newPayload

    server.replyResult(method, validStatus)
    assertThat(client.newPayload(executionPayload(ElFork.Prague)).get())
      .isEqualTo(PayloadStatus(ExecutionPayloadStatus.VALID, ByteArray(32) { 0xab.toByte() }, null))

    server.replyResult(
      method,
      mapOf("status" to "INVALID", "latestValidHash" to "0x" + "00".repeat(32), "validationError" to "bad block"),
    )
    assertThat(client.newPayload(executionPayload(ElFork.Prague)).get())
      .isEqualTo(PayloadStatus(ExecutionPayloadStatus.INVALID, ByteArray(32), "bad block"))

    server.replyResult(method, mapOf("status" to "SYNCING", "latestValidHash" to null, "validationError" to null))
    assertThat(client.newPayload(executionPayload(ElFork.Prague)).get())
      .isEqualTo(PayloadStatus(ExecutionPayloadStatus.SYNCING, null, null))

    server.replyResult(method, mapOf("status" to "ACCEPTED"))
    assertThat(client.newPayload(executionPayload(ElFork.Prague)).get())
      .isEqualTo(PayloadStatus(ExecutionPayloadStatus.ACCEPTED, null, null))
  }

  @Test
  fun `forkChoiceUpdate decodes payload id when present and absent`() {
    val client = client(ElFork.Cancun)
    val method = EngineApiForkMethods.of(ElFork.Cancun).forkChoiceUpdated

    server.replyResult(method, mapOf("payloadStatus" to validStatus, "payloadId" to "0x0102030405060708"))
    assertThat(
      client.forkChoiceUpdate(headBlockHash, safeBlockHash, finalizedBlockHash, payloadAttributes(ElFork.Cancun)).get(),
    )
      .isEqualTo(
        ForkChoiceUpdatedResult(
          PayloadStatus(ExecutionPayloadStatus.VALID, ByteArray(32) { 0xab.toByte() }, null),
          byteArrayOf(1, 2, 3, 4, 5, 6, 7, 8),
        ),
      )

    server.replyResult(method, mapOf("payloadStatus" to mapOf("status" to "SYNCING"), "payloadId" to null))
    assertThat(client.forkChoiceUpdate(headBlockHash, safeBlockHash, finalizedBlockHash, null).get())
      .isEqualTo(ForkChoiceUpdatedResult(PayloadStatus(ExecutionPayloadStatus.SYNCING, null, null), null))
  }

  @Test
  fun `getLatestBlockMetadata decodes hash and timestamp and isOnline reflects reachability`() {
    server.replyResult("eth_getBlockByNumber", mapOf("hash" to "0x" + "cd".repeat(32), "timestamp" to "0x6553f100"))

    assertThat(client(ElFork.Prague).getLatestBlockMetadata().get())
      .isEqualTo(LatestBlockMetadata(ByteArray(32) { 0xcd.toByte() }, 1_700_000_000UL))
    assertThat(client(ElFork.Prague).isOnline().get()).isTrue()

    val closedPort = ServerSocket(0).use { it.localPort }
    val unreachable = URI("http://127.0.0.1:$closedPort").toURL()
    assertThat(client(ElFork.Prague, endpoint = unreachable).isOnline().get()).isFalse()
  }

  @Test
  fun `execution layer JSON-RPC errors fail the future and are counted as failures`() {
    val method = EngineApiForkMethods.of(ElFork.Osaka).getPayload
    server.replyError(method, code = -38001, message = "Unknown payload")

    assertThatThrownBy { client(ElFork.Osaka).getPayload(payloadId).get() }
      .isInstanceOf(ExecutionException::class.java)
      .cause()
      .isInstanceOf(JsonRpcErrorResponseException::class.java)
      .hasMessageContaining("code=-38001 message=Unknown payload")

    awaitRequestLatencyCount(fork = ElFork.Osaka, method = "getPayload", status = "failure", expected = 1)
  }

  @Test
  fun `successful requests are timed with the existing engine API metric tags`() {
    server.replyResult(EngineApiForkMethods.of(ElFork.Prague).newPayload, validStatus)

    client(ElFork.Prague).newPayload(executionPayload(ElFork.Prague)).get()

    awaitRequestLatencyCount(fork = ElFork.Prague, method = "newPayload", status = "success", expected = 1)
  }

  @ParameterizedTest(name = "{0}")
  @MethodSource("malformedResults")
  fun `results violating the spec fail the future`(case: MalformedResult) {
    if (case.method.startsWith("engine_")) {
      assertThat(EngineApiSpec.resultViolations(case.method, case.result ?: NullNode.instance)).isNotEmpty()
    }
    server.replyResult(case.method, case.result)
    val client = client(case.fork)
    val methods = EngineApiForkMethods.of(case.fork)

    val response =
      when (case.method) {
        methods.getPayload -> client.getPayload(payloadId)
        methods.newPayload -> client.newPayload(executionPayload(case.fork))
        methods.forkChoiceUpdated -> client.forkChoiceUpdate(headBlockHash, safeBlockHash, finalizedBlockHash, null)
        else -> client.getLatestBlockMetadata()
      }

    assertThatThrownBy { response.get() }.isInstanceOf(ExecutionException::class.java)
  }

  @Test
  fun `invalid inputs fail the future without sending a request`() {
    val client = client(ElFork.Amsterdam)

    assertThat(client.getPayload(ByteArray(4))).isCompletedExceptionally()
    assertThat(client.newPayload(executionPayload(ElFork.Prague))).isCompletedExceptionally()
    assertThat(server.requests).isEmpty()
  }

  @Test
  fun `sends a freshly signed JWT with every request when a secret is configured`() {
    val secret = ByteArray(32) { it.toByte() }
    val secretFile = Files.createTempFile("jwt", ".hex").apply { toFile().deleteOnExit() }
    Files.writeString(secretFile, "0x" + secret.joinToString("") { "%02x".format(it) } + "\n")
    var now = Instant.ofEpochSecond(1_700_000_000)
    val clock = object : Clock() {
      override fun getZone() = ZoneOffset.UTC

      override fun withZone(zone: java.time.ZoneId?) = this

      override fun instant(): Instant = now
    }
    server.replyResult("eth_getBlockByNumber", mapOf("hash" to "0x" + "cd".repeat(32), "timestamp" to "0x1"))
    val client = client(ElFork.Prague, jwtSecretPath = secretFile, clock = clock)

    client.getLatestBlockMetadata().get()
    now = now.plusSeconds(30)
    client.getLatestBlockMetadata().get()

    val tokens = server.requests.map { requireNotNull(it.authorization).removePrefix("Bearer ") }
    assertThat(tokens).hasSize(2)
    tokens.forEach { assertThat(hasValidSignature(it, secret)).isTrue() }
    assertThat(tokens.map(::issuedAt)).containsExactly(1_700_000_000L, 1_700_000_030L)
  }

  @Test
  fun `getPayload uses the 1s spec timeout unless a timeout is configured`() {
    server.replyResult(
      EngineApiForkMethods.of(ElFork.Prague).getPayload,
      getPayloadResultJson(ElFork.Prague),
      delay = 1500.milliseconds,
    )

    assertThatThrownBy { client(ElFork.Prague, timeoutOverride = null).getPayload(payloadId).get() }
      .isInstanceOf(ExecutionException::class.java)
      .cause()
      .isInstanceOf(TimeoutException::class.java)
      .hasMessageContaining("engine_getPayloadV4")

    assertThat(client(ElFork.Prague, timeoutOverride = 5.seconds).getPayload(payloadId).get(10, TimeUnit.SECONDS))
      .isEqualTo(executionPayload(ElFork.Prague))
  }

  /** The timer records in a completion callback that may run after the caller's `get()` returns. */
  private fun awaitRequestLatencyCount(fork: ElFork, method: String, status: String, expected: Long) {
    await().atMost(5, TimeUnit.SECONDS).untilAsserted {
      assertThat(requestLatencyCount(fork, method, status)).isEqualTo(expected)
    }
  }

  private fun requestLatencyCount(fork: ElFork, method: String, status: String): Long =
    meterRegistry.meters
      .filter { it.id.name.endsWith("request.latency") }
      .filter {
        it.id.getTag("fork") == fork.name && it.id.getTag("method") == method && it.id.getTag("status") == status
      }
      .sumOf { (it as io.micrometer.core.instrument.Timer).count() }

  private fun hasValidSignature(token: String, secret: ByteArray): Boolean {
    val (header, claims, signature) = token.split(".")
    val mac = Mac.getInstance("HmacSHA256").apply { init(SecretKeySpec(secret, "HmacSHA256")) }
    val expected = Base64.getUrlEncoder().withoutPadding().encodeToString(mac.doFinal("$header.$claims".toByteArray()))
    return expected == signature
  }

  private fun issuedAt(token: String): Long {
    val claims = String(Base64.getUrlDecoder().decode(token.split(".")[1]))
    return Regex("\"iat\":(\\d+)").find(claims)!!.groupValues[1].toLong()
  }

  class MalformedResult(
    private val description: String,
    val fork: ElFork,
    val method: String,
    val result: JsonNode?,
  ) {
    override fun toString(): String = description
  }

  companion object {
    private val objectMapper = jacksonObjectMapper()

    private fun json(value: Any): JsonNode = objectMapper.valueToTree(value)

    private fun getPayloadResult(fork: ElFork, edit: ObjectNode.() -> Unit): ObjectNode =
      getPayloadResultJson(fork).apply(edit)

    private fun executionPayloadOf(fork: ElFork, edit: ObjectNode.() -> Unit): ObjectNode =
      getPayloadResult(fork) { (get("executionPayload") as ObjectNode).edit() }

    @JvmStatic
    fun malformedResults(): List<MalformedResult> {
      val prague = EngineApiForkMethods.of(ElFork.Prague)
      val validHash = "0x" + "ab".repeat(32)
      fun prague(description: String, method: String, result: JsonNode?) =
        MalformedResult(description, ElFork.Prague, method, result)
      return listOf(
        prague("getPayload: null result", prague.getPayload, null),
        prague("getPayload: result is not an object", prague.getPayload, TextNode("0x00")),
        prague(
          "getPayload: no executionPayload",
          prague.getPayload,
          getPayloadResult(ElFork.Prague) {
            remove("executionPayload")
          },
        ),
        prague(
          "getPayload: payload without blockHash",
          prague.getPayload,
          executionPayloadOf(ElFork.Prague) {
            remove("blockHash")
          },
        ),
        prague(
          "getPayload: non-hex blockHash",
          prague.getPayload,
          executionPayloadOf(ElFork.Prague) {
            put("blockHash", "0xzz")
          },
        ),
        prague(
          "getPayload: blockNumber as a JSON number",
          prague.getPayload,
          executionPayloadOf(ElFork.Prague) {
            put("blockNumber", 100)
          },
        ),
        MalformedResult(
          "getPayloadV1: payload without transactions",
          ElFork.Paris,
          EngineApiForkMethods.of(ElFork.Paris).getPayload,
          executionPayloadJson(ElFork.Paris).apply { remove("transactions") },
        ),
        MalformedResult(
          "getPayloadV6: payload without blockAccessList",
          ElFork.Amsterdam,
          EngineApiForkMethods.of(ElFork.Amsterdam).getPayload,
          executionPayloadOf(ElFork.Amsterdam) { remove("blockAccessList") },
        ),
        prague("newPayload: null result", prague.newPayload, null),
        prague("newPayload: no status", prague.newPayload, json(mapOf("latestValidHash" to validHash))),
        prague("newPayload: unknown status", prague.newPayload, json(mapOf("status" to "NOT_A_STATUS"))),
        prague(
          "newPayload: non-hex latestValidHash",
          prague.newPayload,
          json(mapOf("status" to "VALID", "latestValidHash" to "0xzz")),
        ),
        prague(
          "forkchoiceUpdated: no payloadStatus",
          prague.forkChoiceUpdated,
          json(mapOf("payloadId" to "0x0102030405060708")),
        ),
        prague(
          "forkchoiceUpdated: unknown status",
          prague.forkChoiceUpdated,
          json(mapOf("payloadStatus" to mapOf("status" to "NOT_A_STATUS"))),
        ),
        prague(
          "forkchoiceUpdated: non-hex payloadId",
          prague.forkChoiceUpdated,
          json(mapOf("payloadStatus" to mapOf("status" to "VALID"), "payloadId" to "0xzz")),
        ),
        prague("eth_getBlockByNumber: no block", "eth_getBlockByNumber", null),
        prague("eth_getBlockByNumber: no timestamp", "eth_getBlockByNumber", json(mapOf("hash" to validHash))),
      )
    }
  }
}
