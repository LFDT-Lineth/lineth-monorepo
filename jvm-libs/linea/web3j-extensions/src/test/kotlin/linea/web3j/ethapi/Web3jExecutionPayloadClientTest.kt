package linea.web3j.ethapi

import com.github.tomakehurst.wiremock.WireMockServer
import com.github.tomakehurst.wiremock.client.WireMock.containing
import com.github.tomakehurst.wiremock.client.WireMock.ok
import com.github.tomakehurst.wiremock.client.WireMock.post
import com.github.tomakehurst.wiremock.client.WireMock.postRequestedFor
import com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo
import com.github.tomakehurst.wiremock.core.WireMockConfiguration.options
import io.vertx.core.json.JsonObject
import linea.domain.BlockParameter
import linea.domain.toBesu
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.rlp.RLP
import linea.web3j.EthBlockExtended
import linea.web3j.createWeb3jHttpService
import linea.web3j.mappers.toDomain
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.ethereum.core.Block
import org.hyperledger.besu.ethereum.core.BlockBody
import org.hyperledger.besu.ethereum.core.BlockHeaderBuilder
import org.hyperledger.besu.ethereum.core.encoding.EncodingContext
import org.hyperledger.besu.ethereum.core.encoding.TransactionEncoder
import org.hyperledger.besu.ethereum.mainnet.MainnetBlockHeaderFunctions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.NullSource
import org.junit.jupiter.params.provider.ValueSource
import org.web3j.protocol.ObjectMapperFactory
import org.web3j.protocol.Web3j

class Web3jExecutionPayloadClientTest {
  private val fixture = JsonObject(javaClass.getResource("/amsterdam-block.json")!!.readText())
  private val block = RLP.decodeBlockWithMainnetFunctions(fixture.getString("rawBlock").decodeHex())
  private val rpcBlock = JsonObject(javaClass.getResource("/amsterdam-block-rpc.json")!!.readText())
  private val acquiredBlock by lazy { toDomain(rpcBlock) }
  private lateinit var server: WireMockServer
  private lateinit var client: Web3jExecutionPayloadClient
  private lateinit var ethClient: Web3jEthApiClient
  private lateinit var web3j: Web3j

  @BeforeEach
  fun setUp() {
    server = WireMockServer(options().dynamicPort())
    server.start()
    val service = createWeb3jHttpService(rpcUrl = server.baseUrl())
    web3j = Web3j.build(service)
    client = Web3jExecutionPayloadClient(service)
    ethClient = Web3jEthApiClient(web3j, service, blockValidator = ::validateLinethBlock)
  }

  @AfterEach
  fun tearDown() {
    web3j.shutdown()
    server.stop()
  }

  @Test
  fun `preserves slot zero and uses the committed empty BAL without an RPC`() {
    val acquired = acquiredBlock.copy(slotNumber = 0UL, blockAccessListHash = Hash.EMPTY_BAL_HASH.bytes.toArray())

    val result = client.getExecutionPayload(acquired).get()

    assertThat(result.slotNumber).isEqualTo(0UL)
    assertThat(result.blockAccessList).isEqualTo("0xc0".decodeHex())
    assertThat(result.blobGasUsed).isEqualTo(0UL)
    assertThat(result.excessBlobGas).isEqualTo(0UL)
    assertThat(result.withdrawals).isEmpty()
    server.verify(0, postRequestedFor(urlEqualTo("/")))
  }

  @ParameterizedTest
  @NullSource
  @ValueSource(strings = ["0x", "0xc0", "0xzz"])
  fun `missing empty or mismatched BAL fails the request`(bal: String?) {
    stub("debug_getRawBlockAccessList", bal)

    assertThatThrownBy {
      client.getExecutionPayload(acquiredBlock).get()
    }.hasCauseInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `acquires Amsterdam metadata once and preserves RLP and prover payload`() {
    stub("eth_getBlockByNumber", rpcBlock.copy())
    val acquired = requireNotNull(ethClient.ethFindBlockByNumberFullTxs(BlockParameter.fromNumber(20UL)).get())
    assertThat(acquired.slotNumber).isEqualTo(20UL)
    assertThat(acquired.blockAccessListHash).isEqualTo(rpcBlock.getString("blockAccessListHash").decodeHex())
    assertThat(RLP.encodeBlock(acquired.toBesu())).isEqualTo(fixture.getString("rawBlock").decodeHex())
    stub("debug_getRawBlockAccessList", fixture.getString("blockAccessList"))
    val payload = client.getExecutionPayload(acquired).get()
    assertThat(payload.slotNumber).isEqualTo(20UL)
    assertThat(payload.blockHash).isEqualTo(acquired.hash)
    assertThat(payload.blockAccessList.encodeHex()).isEqualTo(fixture.getString("blockAccessList"))
    assertThat(payload.transactions.single()).isEqualTo(
      TransactionEncoder.encodeOpaqueBytes(block.body.transactions.single(), EncodingContext.BLOCK_BODY).toArray(),
    )
    server.verify(
      1,
      postRequestedFor(urlEqualTo("/"))
        .withRequestBody(containing("debug_getRawBlockAccessList"))
        .withRequestBody(containing(acquired.hash.encodeHex())),
    )
    server.verify(2, postRequestedFor(urlEqualTo("/")))
  }

  @Test
  fun `missing block and legacy absent header fields remain absent`() {
    stub("eth_getBlockByNumber", null)
    assertThat(ethClient.ethFindBlockByNumberFullTxs(BlockParameter.fromNumber(20UL)).get()).isNull()

    val header = BlockHeaderBuilder.fromHeader(block.header).blockHeaderFunctions(MainnetBlockHeaderFunctions())
      .slotNumber(null).balHash(null).requestsHash(null).parentBeaconBlockRoot(null)
      .excessBlobGas(null).blobGasUsed(null).withdrawalsRoot(null).buildBlockHeader()
    val legacy = Block(header, BlockBody(block.body.transactions, emptyList()))
    val json = rpcBlock.copy().put("hash", header.hash.bytes.toHexString())
    listOf(
      "slotNumber",
      "blockAccessListHash",
      "requestsHash",
      "parentBeaconBlockRoot",
      "excessBlobGas",
      "blobGasUsed",
      "withdrawalsRoot",
      "withdrawals",
    ).forEach { json.remove(it) }
    stub("eth_getBlockByNumber", json)
    val acquired = requireNotNull(ethClient.ethFindBlockByNumberFullTxs(BlockParameter.fromNumber(20UL)).get())
    assertThat(acquired.slotNumber).isNull()
    assertThat(RLP.encodeBlock(acquired.toBesu())).isEqualTo(RLP.encodeBlock(legacy))
    assertThat(client.getExecutionPayload(acquired).get().blockAccessList).isEmpty()
    server.verify(2, postRequestedFor(urlEqualTo("/")))
  }

  @Test
  fun `transaction hash acquisition also retains header metadata`() {
    val json = rpcBlock.copy().put("transactions", block.body.transactions.map { it.hash.bytes.toHexString() })
    stub("eth_getBlockByNumber", json)
    val acquired = requireNotNull(ethClient.ethFindBlockByNumberTxHashes(BlockParameter.fromNumber(20UL)).get())
    assertThat(acquired.slotNumber).isEqualTo(20UL)
    assertThat(acquired.transactions.single()).isEqualTo(block.body.transactions.single().hash.bytes.toArray())
  }

  @Test
  fun `L2 acquisition rejects unsupported fields before requesting a BAL`() {
    val invalidFields = listOf(
      "withdrawals" to listOf(
        JsonObject(
          mapOf(
            "index" to "0x1", "validatorIndex" to "0x1", "address" to ByteArray(20).encodeHex(),
            "amount" to "0x1",
          ),
        ),
      ),
      "withdrawalsRoot" to ByteArray(32).encodeHex(),
      "requestsHash" to ByteArray(32).encodeHex(),
      "blobGasUsed" to "0x1",
      "excessBlobGas" to "0x1",
      "parentBeaconBlockRoot" to ByteArray(32) { 1 }.encodeHex(),
      "requestsHash" to null,
      "blobGasUsed" to null,
    )
    for ((field, value) in invalidFields) {
      stub("eth_getBlockByNumber", rpcBlock.copy().put(field, value))
      assertThatThrownBy { ethClient.ethFindBlockByNumberFullTxs(BlockParameter.fromNumber(20UL)).get() }
        .hasCauseInstanceOf(IllegalArgumentException::class.java)
    }
    server.verify(invalidFields.size, postRequestedFor(urlEqualTo("/")))
  }

  @Test
  fun `metadata requires paired fields and compares hashes by content`() {
    assertThatThrownBy { acquiredBlock.copy(slotNumber = null) }.isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy { acquiredBlock.copy(blockAccessListHash = null) }
      .isInstanceOf(IllegalArgumentException::class.java)
    assertThatThrownBy { acquiredBlock.copy(blockAccessListHash = byteArrayOf(1)) }
      .isInstanceOf(IllegalArgumentException::class.java)
    val same = acquiredBlock.copy(blockAccessListHash = requireNotNull(acquiredBlock.blockAccessListHash).copyOf())
    assertThat(same).isEqualTo(acquiredBlock).hasSameHashCodeAs(acquiredBlock)
    assertThat(acquiredBlock.copy(slotNumber = 0UL)).isNotEqualTo(acquiredBlock)
  }

  @ParameterizedTest
  @ValueSource(booleans = [false, true])
  fun `mapping errors fail the future even when no BAL RPC is needed`(amsterdam: Boolean) {
    val invalid = acquiredBlock.copy(
      slotNumber = if (amsterdam) 0UL else null,
      blockAccessListHash = if (amsterdam) Hash.EMPTY_BAL_HASH.bytes.toArray() else null,
      transactions = listOf(acquiredBlock.transactions.single().copy(v = null, yParity = null)),
    )

    val future = client.getExecutionPayload(invalid)

    assertThat(future).isCompletedExceptionally()
    assertThatThrownBy { future.get() }.hasStackTraceContaining("Error mapping transaction to Besu")
    server.verify(0, postRequestedFor(urlEqualTo("/")))
  }

  private fun toDomain(json: JsonObject): linea.domain.Block = ObjectMapperFactory.getObjectMapper()
    .readValue(json.encode(), EthBlockExtended.Block::class.java).toDomain()

  private fun stub(method: String, result: Any?) {
    server.stubFor(
      post(urlEqualTo("/")).withRequestBody(containing(method))
        .willReturn(ok(JsonObject(mapOf("jsonrpc" to "2.0", "id" to 1, "result" to result)).encode())),
    )
  }
}
