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
import okhttp3.OkHttpClient
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import org.mockito.kotlin.mock
import tech.pegasys.teku.ethereum.executionclient.schema.ForkChoiceStateV1
import tech.pegasys.teku.ethereum.executionclient.schema.PayloadAttributesV3
import tech.pegasys.teku.infrastructure.bytes.Bytes20
import tech.pegasys.teku.infrastructure.unsigned.UInt64
import java.net.InetSocketAddress
import java.net.URI
import java.nio.file.Path
import java.util.Optional
import java.util.concurrent.LinkedBlockingQueue
import java.util.concurrent.TimeUnit
import kotlin.io.path.writeText
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

class TekuWeb3JClientFactoryTest {
  private val server = HttpServer.create(InetSocketAddress("127.0.0.1", 0), 0).apply { start() }
  private val requests = LinkedBlockingQueue<Pair<String, String?>>()
  private val clients = mutableListOf<Web3jClient>()
  private val endpoint = URI("http://127.0.0.1:${server.address.port}").toURL()

  @AfterEach
  fun tearDown() {
    clients.forEach { it.close() }
    server.stop(0)
  }

  private fun respond(body: String, status: Int = 200, delayMillis: Long = 0) {
    server.createContext("/") { exchange ->
      requests.add(
        exchange.requestBody.readBytes().toString(Charsets.UTF_8) to exchange.requestHeaders.getFirst("Authorization"),
      )
      if (delayMillis > 0) Thread.sleep(delayMillis)
      exchange.use {
        val bytes = body.toByteArray()
        exchange.sendResponseHeaders(status, bytes.size.toLong())
        exchange.responseBody.write(bytes)
      }
    }
  }

  private fun client(jwtPath: String? = null, timeout: Duration = 5.seconds): Web3jClient =
    TekuWeb3JClientFactory.create(endpoint, jwtPath, timeout).also { clients.add(it) }

  @Test
  fun `serializes engine params as hex strings and decodes the result`() {
    respond(
      """{"jsonrpc":"2.0","id":1,"result":{
        |"payloadStatus":{"status":"VALID","latestValidHash":null,"validationError":null},
        |"payloadId":"0x0000000000000001"}}
      """.trimMargin(),
    )
    val forkChoiceState = ForkChoiceStateV1(
      /* headBlockHash = */
      Bytes32.fromHexString("0x" + "11".repeat(32)),
      /* safeBlockHash = */
      Bytes32.fromHexString("0x" + "22".repeat(32)),
      /* finalizedBlockHash = */
      Bytes32.fromHexString("0x" + "33".repeat(32)),
    )
    val payloadAttributes = PayloadAttributesV3(
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

    val response = client().forkChoiceUpdatedV3(
      forkChoiceState,
      Optional.of(payloadAttributes),
    ).get(5, TimeUnit.SECONDS)
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
    val response = client().exchangeCapabilities(emptyList()).get(5, TimeUnit.SECONDS)
    assertThat(response.payload).isNull()
    assertThat(response.errorMessage).contains("-32602", "invalid payload")
  }

  @Test
  fun `reports HTTP authentication errors`() {
    respond("unauthorized", status = 401)
    val response = client().exchangeCapabilities(emptyList()).get(5, TimeUnit.SECONDS)
    assertThat(response.payload).isNull()
    assertThat(response.errorMessage).contains("unauthorized")
  }

  @Test
  fun `applies JWT authentication`(@TempDir tempDir: Path) {
    val jwtPath = tempDir.resolve("jwt.hex").apply { writeText("11".repeat(32)) }
    respond("""{"jsonrpc":"2.0","id":1,"result":[]}""")
    client(jwtPath.toString()).exchangeCapabilities(emptyList()).get(5, TimeUnit.SECONDS)
    assertThat(requests.poll(5, TimeUnit.SECONDS)!!.second).startsWith("Bearer ")
  }

  @Test
  fun `close cancels an outstanding request`() {
    respond("""{"jsonrpc":"2.0","id":1,"result":[]}""", delayMillis = 500)
    val client = client()
    val response = client.exchangeCapabilities(emptyList())
    assertThat(requests.poll(5, TimeUnit.SECONDS)).isNotNull()
    client.close()
    assertThat(response.get(5, TimeUnit.SECONDS).errorMessage).isNotBlank()
  }

  @Test
  fun `close shuts down owned HTTP resources`() {
    val httpClient = OkHttpClient()
    val client = Web3jClient(endpoint.toString(), httpClient, mock())
    client.close()
    assertThat(httpClient.dispatcher.executorService.isShutdown).isTrue()
    assertThat(httpClient.connectionPool.connectionCount()).isZero()
  }

  @Test
  fun `honors endpoint timeout when it is shorter than Teku method timeout`() {
    respond("""{"jsonrpc":"2.0","id":1,"result":[]}""", delayMillis = 500)
    val response = client(timeout = 50.milliseconds).exchangeCapabilities(emptyList()).get(5, TimeUnit.SECONDS)
    assertThat(response.payload).isNull()
    assertThat(response.errorMessage).isNotBlank()
  }
}
