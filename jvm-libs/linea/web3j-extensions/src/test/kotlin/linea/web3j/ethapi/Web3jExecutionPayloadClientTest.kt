package linea.web3j.ethapi

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.ok
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.core.WireMockConfiguration.options
import io.vertx.core.json.JsonObject
import linea.domain.BlockNumberAndHash
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.rlp.RLP
import linea.web3j.createWeb3jHttpService
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.units.bigints.UInt64
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.BlobGas
import org.hyperledger.besu.datatypes.GWei
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.ethereum.core.Block
import org.hyperledger.besu.ethereum.core.BlockBody
import org.hyperledger.besu.ethereum.core.BlockHeaderBuilder
import org.hyperledger.besu.ethereum.core.Withdrawal
import org.hyperledger.besu.ethereum.mainnet.MainnetBlockHeaderFunctions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.NullSource
import org.junit.jupiter.params.provider.ValueSource
import java.util.Optional

class Web3jExecutionPayloadClientTest {
  private val fixture = JsonObject(javaClass.getResource("/amsterdam-block.json")!!.readText())
  private val block = RLP.decodeBlockWithMainnetFunctions(fixture.getString("rawBlock").decodeHex())
  private val blockId = BlockNumberAndHash(20UL, fixture.getString("blockHash").decodeHex())
  private lateinit var server: WireMockServer
  private lateinit var client: Web3jExecutionPayloadClient

  @BeforeEach
  fun setUp() {
    server = WireMockServer(options().dynamicPort())
    server.start()
    client = Web3jExecutionPayloadClient(createWeb3jHttpService(rpcUrl = server.baseUrl()))
  }

  @AfterEach
  fun tearDown() {
    server.stop()
  }

  @Test
  fun `preserves Amsterdam payload from a raw block including signed typed transaction`() {
    stub("debug_getRawBlock", fixture.getString("rawBlock"))
    stub("debug_getRawBlockAccessList", fixture.getString("blockAccessList"))

    val result = client.getExecutionPayload(blockId).get()

    assertThat(result.slotNumber).isEqualTo(20UL)
    assertThat(result.blockHash).isEqualTo(blockId.hash)
    assertThat(result.blockAccessList.encodeHex()).isEqualTo(fixture.getString("blockAccessList"))
    assertThat(result.transactions).hasSize(1)
    assertThat(result.transactions.first()[0]).isEqualTo(2.toByte())
    assertThat(Hash.hash(Bytes.wrap(result.transactions.first())).bytes.toHexString())
      .isEqualTo(fixture.getString("transactionHash"))
    server.verify(postRequestedFor(urlEqualTo("/")).withRequestBody(containing("\"params\":[\"0x14\"]")))
    server.verify(
      postRequestedFor(urlEqualTo("/"))
        .withRequestBody(containing("debug_getRawBlockAccessList"))
        .withRequestBody(containing(blockId.hash.encodeHex())),
    )
  }

  @Test
  fun `preserves withdrawals blob gas and slot zero with a valid empty BAL`() {
    val header = BlockHeaderBuilder.fromHeader(block.header).blockHeaderFunctions(MainnetBlockHeaderFunctions())
      .slotNumber(0L).balHash(Hash.EMPTY_BAL_HASH).blobGasUsed(131072L).excessBlobGas(BlobGas.of(262144L))
      .buildBlockHeader()
    val withdrawal = Withdrawal(UInt64.valueOf(3), UInt64.valueOf(7), Address.ZERO, GWei.of(32))
    val modified = Block(header, BlockBody(block.body.transactions, emptyList(), Optional.of(listOf(withdrawal))))
    stub("debug_getRawBlock", RLP.encodeBlock(modified).encodeHex())
    stub("debug_getRawBlockAccessList", "0xc0")

    val result = client.getExecutionPayload(BlockNumberAndHash(20UL, header.hash.bytes.toArray())).get()

    assertThat(result.slotNumber).isEqualTo(0UL)
    assertThat(result.blockAccessList).isEqualTo("0xc0".decodeHex())
    assertThat(result.blobGasUsed).isEqualTo(131072UL)
    assertThat(result.excessBlobGas).isEqualTo(262144UL)
    assertThat(result.withdrawals).containsExactly(linea.domain.Withdrawal(3UL, 7UL, ByteArray(20), 32UL))
  }

  @Test
  fun `pre Amsterdam blocks do not request a BAL`() {
    val header = BlockHeaderBuilder.fromHeader(block.header)
      .blockHeaderFunctions(MainnetBlockHeaderFunctions())
      .slotNumber(null).balHash(null).buildBlockHeader()
    stub("debug_getRawBlock", RLP.encodeBlock(Block(header, block.body)).encodeHex())

    val result = client.getExecutionPayload(BlockNumberAndHash(20UL, header.hash.bytes.toArray())).get()

    assertThat(result.slotNumber).isNull()
    assertThat(result.blockAccessList).isEmpty()
    server.verify(0, postRequestedFor(urlEqualTo("/")).withRequestBody(containing("debug_getRawBlockAccessList")))
  }

  @ParameterizedTest
  @NullSource
  @ValueSource(strings = ["0x", "0xc0"])
  fun `missing empty or mismatched BAL fails the request`(bal: String?) {
    stub("debug_getRawBlock", fixture.getString("rawBlock"))
    stub("debug_getRawBlockAccessList", bal)

    assertThatThrownBy {
      client.getExecutionPayload(blockId).get()
    }.hasCauseInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `BAL RPC errors fail the request`() {
    stub("debug_getRawBlock", fixture.getString("rawBlock"))
    server.stubFor(
      post(urlEqualTo("/")).withRequestBody(containing("debug_getRawBlockAccessList"))
        .willReturn(ok("""{"jsonrpc":"2.0","id":1,"error":{"code":-32001,"message":"BAL pruned"}}""")),
    )
    assertThatThrownBy { client.getExecutionPayload(blockId).get() }.hasStackTraceContaining("BAL pruned")
  }

  @Test
  fun `a reorg between conflation and the raw block request fails before fetching the BAL`() {
    stub("debug_getRawBlock", fixture.getString("rawBlock"))
    assertThatThrownBy { client.getExecutionPayload(BlockNumberAndHash(20UL, ByteArray(32))).get() }
      .hasStackTraceContaining("Raw block does not match")
    assertThatThrownBy { client.getExecutionPayload(BlockNumberAndHash(21UL, blockId.hash)).get() }
      .hasStackTraceContaining("Raw block does not match")
    server.verify(0, postRequestedFor(urlEqualTo("/")).withRequestBody(containing("debug_getRawBlockAccessList")))
  }

  private fun stub(method: String, result: String?) {
    server.stubFor(
      post(urlEqualTo("/")).withRequestBody(containing(method))
        .willReturn(ok(JsonObject.of("jsonrpc", "2.0", "id", 1, "result", result).encode())),
    )
  }
}
