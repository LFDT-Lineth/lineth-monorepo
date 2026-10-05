/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.teku

import com.fasterxml.jackson.databind.ObjectMapper
import com.sun.net.httpserver.HttpServer
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceStateV1
import tech.pegasys.teku.ethereum.executionclient.schema.PayloadAttributesV3
import tech.pegasys.teku.infrastructure.bytes.Bytes20
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import java.net.InetSocketAddress
import java.net.URI
import java.net.URL
import java.nio.file.Path
import java.util.Optional
import java.util.concurrent.CountDownLatch
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit
import kotlin.io.path.writeText
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class TekuWeb3JClientFactoryTest {
  private lateinit var server: HttpServer
  private lateinit var endpoint: URL
  private val requests = LinkedBlockingQueue<Pair<String, String?>>()
  private val clients = mutableListOf<Web3JClient>()
  private val responseGates = mutableListOf<CountDownLatch>()

  @BeforeEach
  fun setUp() {
    server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0).apply { start() }
    endpoint = URI("http://127.0.0.1:${server.address.port}").toURL()
  }

  @AfterEach
  fun tearDown() {
    // Release before stopping the server so a gated handler can never wedge shutdown.
    responseGates.forEach { it.countDown() }
    clients.forEach { it.close() }
    server.stop(0)
  }

  /**
   * [onRequest] runs after the request has been recorded and before the response is written, letting a test hold the
   * exchange open for as long as it needs.
   */
  private fun respond(body: String, status: Int = 200, onRequest: () -> Unit = {}) {
    server.createContext("/") { exchange ->
      requests.add(
        exchange.requestBody.readBytes().toString(Charsets.UTF_8) to exchange.requestHeaders.getFirst("Authorization"),
      )
      onRequest()
      exchange.use {
        val bytes = body.toByteArray()
        exchange.sendResponseHeaders(status, bytes.size.toLong())
        exchange.responseBody.write(bytes)
      }
    }
  }

  private fun client(jwtPath: String? = null, timeout: Duration = 5.seconds): Web3JClient =
    TekuWeb3JClientFactory.create(endpoint, jwtPath, timeout).also { clients.add(it) }

  /**
   * A latch that blocks a request handler until the test releases it. Registered so [tearDown] always opens it, even
   * when a test fails early.
   */
  private fun responseGate(): CountDownLatch = CountDownLatch(1).also { responseGates.add(it) }

  /**
   * Issues `engine_forkchoiceUpdatedV3`. Teku caps `engine_exchangeCapabilities` at a hardcoded 1s
   * (`AbstractExecutionEngineClient.EXCHANGE_CAPABILITIES_TIMEOUT`) and overrides the client's configured timeout with
   * it, which is too tight to survive a loaded CI runner. `forkchoiceUpdated` is budgeted with the far more generous
   * `EL_ENGINE_BLOCK_EXECUTION_TIMEOUT`, so tests that are not specifically about timeouts use this instead.
   */
  private fun Web3JClient.forkChoiceUpdated() = forkChoiceUpdatedV3(forkChoiceState, Optional.of(payloadAttributes))

  @Test
  fun `serializes engine params as hex strings and decodes the result`() {
    respond(FORK_CHOICE_UPDATED_RESPONSE)

    val response = client().forkChoiceUpdated().get(5, TimeUnit.SECONDS)
    assertThat(response.errorMessage).isNull()
    assertThat(
      response.payload.asInternalExecutionPayload().payloadId.orElseThrow().toHexString(),
    ).isEqualTo("0x0000000000000001")
    val serialized = requests.poll(5, TimeUnit.SECONDS)!!.first
    assertThat(serialized).doesNotContain("maxValue", "thirtyTwoEth", "wrappedBytes")
    assertThat(serialized).contains(""""headBlockHash":"0x${"11".repeat(32)}"""")
    assertThat(serialized).contains(""""safeBlockHash":"0x${"22".repeat(32)}"""")
    assertThat(serialized).contains(""""finalizedBlockHash":"0x${"33".repeat(32)}"""")
    assertThat(serialized).contains(""""timestamp":"0x6a4bdc88"""")
    assertThat(serialized).contains(""""prevRandao":"0x${"44".repeat(32)}"""")
    assertThat(serialized).contains(""""suggestedFeeRecipient":"0x${"55".repeat(20)}"""")
    assertThat(serialized).contains(""""parentBeaconBlockRoot":"0x${"66".repeat(32)}"""")
  }

  @Test
  fun `decodes latest block metadata without Web3j`() {
    val hash = "0x" + "11".repeat(32)
    val parentHash = "0x" + "22".repeat(32)
    respond("""{"jsonrpc":"2.0","id":1,"result":{"hash":"$hash","parentHash":"$parentHash","timestamp":"0x2a"}}""")
    val client = client()
    val block = client.powChainHead.get(5, TimeUnit.SECONDS)
    assertThat(block.blockHash.toHexString()).isEqualTo(hash)
    assertThat(block.blockTimestamp.longValue()).isEqualTo(42L)
    assertThat(client.endpoint).isEqualTo(endpoint.toString())
    val request = ObjectMapper().readTree(requests.poll(5, TimeUnit.SECONDS)!!.first)
    assertThat(request["method"].asText()).isEqualTo("eth_getBlockByNumber")
    assertThat(request["params"].toString()).isEqualTo("[\"latest\",false]")
  }

  @Test
  fun `preserves JSON-RPC error code and message`() {
    respond("""{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"invalid payload"}}""")
    val response = client().forkChoiceUpdated().get(5, TimeUnit.SECONDS)
    assertThat(response.payload).isNull()
    assertThat(response.errorMessage).contains("-32602", "invalid payload")
  }

  @Test
  fun `reports HTTP authentication errors`() {
    respond("unauthorized", status = 401)
    val response = client().forkChoiceUpdated().get(5, TimeUnit.SECONDS)
    assertThat(response.payload).isNull()
    assertThat(response.errorMessage).contains("unauthorized")
  }

  @Test
  fun `applies JWT authentication`(@TempDir tempDir: Path) {
    val jwtPath = tempDir.resolve("jwt.hex").apply { writeText("11".repeat(32)) }
    respond(FORK_CHOICE_UPDATED_RESPONSE)
    // Assert the call succeeded first: otherwise a timeout leaves `requests` empty and this fails as an opaque NPE on
    // the poll below instead of reporting the real cause.
    assertThat(client(jwtPath.toString()).forkChoiceUpdated().get(5, TimeUnit.SECONDS).errorMessage).isNull()
    val authorization = requests.poll(5, TimeUnit.SECONDS)
    assertThat(authorization).isNotNull()
    assertThat(authorization!!.second).startsWith("Bearer ")
  }

  @Test
  fun `close cancels an outstanding request`() {
    // Hold the response open until the test releases it, rather than sleeping for a fixed period. A sleep races the
    // test thread: the request is recorded before the sleep starts, so if the test is descheduled long enough the
    // response completes successfully and no cancellation is ever observed.
    val gate = responseGate()
    respond(FORK_CHOICE_UPDATED_RESPONSE, onRequest = { gate.await() })
    val client = client()
    val inFlight = client.forkChoiceUpdated()
    assertThat(requests.poll(5, TimeUnit.SECONDS)).isNotNull()

    client.close()

    // Bounded well inside Teku's 8s EL_ENGINE_BLOCK_EXECUTION_TIMEOUT for forkchoiceUpdated. `close` cancels in-flight
    // calls immediately, so a generous 4s is ample; allowing 8s or more would let Teku's own timeout complete the
    // future and the assertion would hold even if `close` cancelled nothing.
    val response = inFlight.get(CLOSE_CANCELLATION_TIMEOUT_MS, TimeUnit.MILLISECONDS)
    assertThat(response.errorMessage).isNotBlank()
    gate.countDown()
  }

  @Test
  fun `honors endpoint timeout when it is shorter than Teku method timeout`() {
    // The gate keeps the request in flight so the only thing that can complete the future is the endpoint timeout.
    val gate = responseGate()
    respond(FORK_CHOICE_UPDATED_RESPONSE, onRequest = { gate.await() })
    val response = client(timeout = 50.milliseconds).forkChoiceUpdated().get(5, TimeUnit.SECONDS)
    assertThat(response.payload).isNull()
    assertThat(response.errorMessage).isNotBlank()
    gate.countDown()
  }

  companion object {
    /**
     * Upper bound for observing `close`-driven cancellation. Must stay comfortably below Teku's 8s
     * `EL_ENGINE_BLOCK_EXECUTION_TIMEOUT` so the assertion cannot be satisfied by that timeout instead of by `close`.
     */
    private const val CLOSE_CANCELLATION_TIMEOUT_MS = 4_000L

    private val FORK_CHOICE_UPDATED_RESPONSE =
      """
      {"jsonrpc":"2.0","id":1,"result":{
      "payloadStatus":{"status":"VALID","latestValidHash":null,"validationError":null},
      "payloadId":"0x0000000000000001"}}
      """.trimIndent()

    private val forkChoiceState = ForkChoiceStateV1(
      /* headBlockHash = */
      Bytes32.fromHexString("0x" + "11".repeat(32)),
      /* safeBlockHash = */
      Bytes32.fromHexString("0x" + "22".repeat(32)),
      /* finalizedBlockHash = */
      Bytes32.fromHexString("0x" + "33".repeat(32)),
    )

    private val payloadAttributes = PayloadAttributesV3(
      /* timestamp = */
      UInt64.valueOf(1783356552L),
      /* prevRandao = */
      Bytes32.fromHexString("0x" + "44".repeat(32)),
      /* suggestedFeeRecipient = */
      Bytes20.fromHexString("0x" + "55".repeat(20)),
      /* withdrawals = */
      emptyList(),
      /* parentBeaconBlockRoot = */
      Bytes32.fromHexString("0x" + "66".repeat(32)),
    )
  }
}
