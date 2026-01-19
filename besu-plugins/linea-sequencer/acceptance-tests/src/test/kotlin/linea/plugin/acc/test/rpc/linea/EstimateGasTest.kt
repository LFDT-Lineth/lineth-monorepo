/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea

import com.fasterxml.jackson.annotation.JsonInclude
import com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL
import com.fasterxml.jackson.core.JsonParser
import com.fasterxml.jackson.core.JsonToken
import com.fasterxml.jackson.databind.DeserializationContext
import com.fasterxml.jackson.databind.JsonDeserializer
import com.fasterxml.jackson.databind.ObjectReader
import com.fasterxml.jackson.databind.annotation.JsonDeserialize
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import linea.plugin.acc.test.LineaPluginPoSTestBase
import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import net.consensys.linea.bl.TransactionProfitabilityCalculator
import net.consensys.linea.config.LineaProfitabilityCliOptions
import net.consensys.linea.rpc.methods.LineaEstimateGas
import net.consensys.linea.utils.CachingTransactionCompressor
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.units.bigints.UInt64
import org.assertj.core.api.Assertions.assertThat
import org.bouncycastle.crypto.digests.KeccakDigest
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.response.RpcErrorType
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import org.web3j.protocol.http.HttpService
import java.io.IOException
import java.math.BigInteger
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.nio.charset.StandardCharsets

open class EstimateGasTest : LineaPluginPoSTestBase() {
  protected lateinit var profitabilityCalculator: TransactionProfitabilityCalculator

  override fun getTestCliOptions(): List<String> {
    return testCommandLineOptionsBuilder.build()
  }

  protected open val testCommandLineOptionsBuilder: TestCommandLineOptionsBuilder
    get() = TestCommandLineOptionsBuilder()
      .set("--plugin-linea-fixed-gas-cost-wei=", FIXED_GAS_COST_WEI.toString())
      .set("--plugin-linea-variable-gas-cost-wei=", VARIABLE_GAS_COST_WEI.toString())
      .set("--plugin-linea-min-margin=", MIN_MARGIN.toString())
      .set("--plugin-linea-estimate-gas-min-margin=", ESTIMATE_GAS_MIN_MARGIN.toString())
      .set("--plugin-linea-max-tx-gas-limit=", MAX_TRANSACTION_GAS_LIMIT.toString())

  @BeforeEach
  fun setMinGasPrice() {
    minerNode.miningParameters.minTransactionGasPrice = MIN_GAS_PRICE
  }

  @BeforeEach
  fun createDefaultConfigurations() {
    val profitabilityConf =
      LineaProfitabilityCliOptions.create().toDomainObject().toBuilder()
        .fixedCostWei(FIXED_GAS_COST_WEI.toLong())
        .variableCostWei(VARIABLE_GAS_COST_WEI.toLong())
        .minMargin(MIN_MARGIN)
        .estimateGasMinMargin(ESTIMATE_GAS_MIN_MARGIN)
        .build()
    profitabilityCalculator = TransactionProfitabilityCalculator(profitabilityConf, CachingTransactionCompressor())
  }

  @Test
  fun lineaEstimateGasMatchesEthEstimateGas() {
    val sender = accounts.secondaryBenefactor

    val callParams = CallParams(
      chainId = null,
      from = sender.address,
      nonce = null,
      to = sender.address,
      value = null,
      data = Bytes.EMPTY.toHexString(),
      gas = "0",
      gasPrice = null,
      maxFeePerGas = null,
      maxPriorityFeePerGas = null,
    )

    val reqEth = RawEstimateGasRequest(callParams)
    val reqLinea = LineaEstimateGasRequest(callParams)
    val respEth = reqEth.execute(minerNode.nodeRequests())
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respEth).isEqualTo(respLinea.result.gasLimit)
  }

  @Test
  fun passingGasPriceFieldWorks() {
    val sender = accounts.secondaryBenefactor

    val callParams = CallParams(
      chainId = null,
      from = sender.address,
      nonce = null,
      to = sender.address,
      value = null,
      data = Bytes.EMPTY.toHexString(),
      gas = "0",
      gasPrice = "0x1234",
      maxFeePerGas = null,
      maxPriorityFeePerGas = null,
    )

    val reqLinea = LineaEstimateGasRequest(callParams)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.hasError()).isFalse()
    assertThat(respLinea.result).isNotNull()
  }

  @Test
  fun passingChainIdFieldWorks() {
    val sender = accounts.secondaryBenefactor

    val callParams = CallParams(
      chainId = "0x539",
      from = sender.address,
      nonce = null,
      to = sender.address,
      value = null,
      data = Bytes.EMPTY.toHexString(),
      gas = "0",
      gasPrice = "0x1234",
      maxFeePerGas = null,
      maxPriorityFeePerGas = null,
    )

    val reqLinea = LineaEstimateGasRequest(callParams)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.hasError()).isFalse()
    assertThat(respLinea.result).isNotNull()
  }

  @Test
  fun passingEIP1559FieldsWorks() {
    val sender = accounts.secondaryBenefactor

    val callParams = CallParams(
      chainId = null,
      from = sender.address,
      nonce = null,
      to = sender.address,
      value = null,
      data = Bytes.EMPTY.toHexString(),
      gas = "0",
      gasPrice = null,
      maxFeePerGas = "0x1234",
      maxPriorityFeePerGas = "0x1",
    )

    val reqLinea = LineaEstimateGasRequest(callParams)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.hasError()).isFalse()
    assertThat(respLinea.result).isNotNull()
  }

  @Test
  fun passingChainIdAndEIP1559FieldsWorks() {
    val sender = accounts.secondaryBenefactor

    val callParams = CallParams(
      chainId = "0x539",
      from = sender.address,
      nonce = null,
      to = sender.address,
      value = null,
      data = Bytes.EMPTY.toHexString(),
      gas = "0",
      gasPrice = null,
      maxFeePerGas = "0x1234",
      maxPriorityFeePerGas = null,
    )

    val reqLinea = LineaEstimateGasRequest(callParams)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.hasError()).isFalse()
    assertThat(respLinea.result).isNotNull()
  }

  @Test
  fun passingStateOverridesWorks() {
    val sender = accounts.secondaryBenefactor

    val actualBalance = minerNode.execute(ethTransactions.getBalance(sender))

    assertThat(actualBalance).isGreaterThan(BigInteger.ONE)

    val callParams = CallParams(
      chainId = "0x539",
      from = sender.address,
      nonce = null,
      to = sender.address,
      value = "1",
      data = Bytes.EMPTY.toHexString(),
      gas = "0",
      gasPrice = "0x1234",
      maxFeePerGas = null,
      maxPriorityFeePerGas = null,
    )

    val zeroBalance = mapOf("balance" to Wei.ZERO.toHexString())

    val stateOverrides = mapOf(accounts.secondaryBenefactor.address to zeroBalance)

    val reqLinea = LineaEstimateGasRequest(callParams, stateOverrides)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.hasError()).isTrue()
    assertThat(respLinea.error.code).isEqualTo(-32000)
    // 0x5d539a1 ~= (0x1234 * 21_000 gas) + 1 value (since the tx is a simple transfer)
    assertThat(respLinea.error.message)
      .isEqualTo(
        "transaction up-front cost 0x5d539a1 exceeds transaction sender account balance 0x0 for sender %s".format(sender.address),
      )
  }

  @Test
  fun passingNonceWorks() {
    val sender = accounts.secondaryBenefactor

    val callParams = CallParams(
      chainId = null,
      from = sender.address,
      nonce = "0",
      to = sender.address,
      value = null,
      data = Bytes.EMPTY.toHexString(),
      gas = "0",
      gasPrice = null,
      maxFeePerGas = "0x1234",
      maxPriorityFeePerGas = null,
    )

    val reqLinea = LineaEstimateGasRequest(callParams)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.hasError()).isFalse()
    assertThat(respLinea.result).isNotNull()

    // try with a future nonce
    val callParamsFuture = CallParams(
      chainId = null,
      from = sender.address,
      nonce = "10",
      to = sender.address,
      value = null,
      data = Bytes.EMPTY.toHexString(),
      gas = "0",
      gasPrice = null,
      maxFeePerGas = "0x1234",
      maxPriorityFeePerGas = null,
    )

    val reqLineaFuture = LineaEstimateGasRequest(callParamsFuture)
    val respLineaFuture = reqLineaFuture.execute(minerNode.nodeRequests())
    assertThat(respLineaFuture.hasError()).isFalse()
    assertThat(respLineaFuture.result).isNotNull()
  }

  @Test
  fun lineaEstimateGasIsProfitable() {
    val sender = accounts.secondaryBenefactor

    val keccakDigest = KeccakDigest(256)
    val txData = StringBuilder()
    txData.append("0x")
    for (i in 0 until 5) {
      keccakDigest.update(byteArrayOf(i.toByte()), 0, 1)
      val out = ByteArray(32)
      keccakDigest.doFinal(out, 0)
      txData.append(BigInteger(out).abs())
    }
    val payload = Bytes.wrap(txData.toString().toByteArray(StandardCharsets.UTF_8))

    val callParams = CallParams(
      chainId = null,
      from = sender.address,
      nonce = null,
      to = sender.address,
      value = null,
      data = payload.toHexString(),
      gas = "0",
      gasPrice = null,
      maxFeePerGas = null,
      maxPriorityFeePerGas = null,
    )

    val reqLinea = LineaEstimateGasRequest(callParams)
    val respLinea = reqLinea.execute(minerNode.nodeRequests()).result

    val estimatedGasLimit = UInt64.fromHexString(respLinea.gasLimit).toLong()
    val baseFee = Wei.fromHexString(respLinea.baseFeePerGas)
    val estimatedPriorityFee = Wei.fromHexString(respLinea.priorityFeePerGas)
    val estimatedMaxGasPrice = baseFee.add(estimatedPriorityFee)

    val tx = org.hyperledger.besu.ethereum.core.Transaction.builder()
      .sender(Address.fromHexString(sender.address))
      .to(Address.fromHexString(sender.address))
      .gasLimit(estimatedGasLimit)
      .gasPrice(estimatedMaxGasPrice)
      .chainId(BigInteger.valueOf(CHAIN_ID))
      .value(Wei.ZERO)
      .payload(payload)
      .signature(LineaEstimateGas.FAKE_SIGNATURE_FOR_SIZE_CALCULATION)
      .build()

    assertIsProfitable(tx, baseFee, estimatedMaxGasPrice, estimatedGasLimit)
  }

  protected open fun assertIsProfitable(
    tx: org.hyperledger.besu.ethereum.core.Transaction,
    baseFee: Wei,
    estimatedMaxGasPrice: Wei,
    estimatedGasLimit: Long,
  ) {
    val minGasPrice = minerNode.miningParameters.minTransactionGasPrice

    val compressedSize = profitabilityCalculator.getCompressedTxSize(tx)
    assertThat(
      profitabilityCalculator.isProfitable(
        "Test",
        tx,
        MIN_MARGIN,
        baseFee,
        estimatedMaxGasPrice,
        estimatedGasLimit,
        minGasPrice,
        compressedSize,
      ),
    ).isTrue()
  }

  @Test
  fun invalidParametersLineaEstimateGasRequestReturnErrorResponse() {
    val sender = accounts.secondaryBenefactor
    val callParams = CallParams(
      chainId = null,
      from = sender.address,
      nonce = null,
      to = null,
      value = "",
      data = "",
      gas = Integer.MAX_VALUE.toString(),
      gasPrice = null,
      maxFeePerGas = null,
      maxPriorityFeePerGas = null,
    )
    val reqLinea = BadLineaEstimateGasRequest(callParams)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.code).isEqualTo(RpcErrorType.INVALID_PARAMS.code)
    assertThat(respLinea.message).isEqualTo(RpcErrorType.INVALID_PARAMS.message)
  }

  @Test
  fun revertedTransactionReturnErrorResponse() {
    val simpleStorage = deploySimpleStorage()
    val sender = accounts.secondaryBenefactor
    val reqLinea = BadLineaEstimateGasRequest(
      CallParams(
        chainId = null,
        from = sender.address,
        nonce = null,
        to = simpleStorage.contractAddress,
        value = "",
        data = "",
        gas = "0",
        gasPrice = null,
        maxFeePerGas = null,
        maxPriorityFeePerGas = null,
      ),
    )
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.code).isEqualTo(3)
    assertThat(respLinea.message).isEqualTo("Execution reverted")
    assertThat(respLinea.data).isEqualTo("\"0x\"")
  }

  @Test
  fun failedTransactionReturnErrorResponse() {
    val sender = accounts.secondaryBenefactor
    val reqLinea = BadLineaEstimateGasRequest(
      CallParams(
        chainId = null,
        from = sender.address,
        nonce = null,
        to = null,
        value = "",
        data = Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY,
        gas = "0",
        gasPrice = null,
        maxFeePerGas = null,
        maxPriorityFeePerGas = null,
      ),
    )
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.code).isEqualTo(-32000)
    assertThat(respLinea.message)
      .isEqualTo(
        "Transaction processing could not be completed due to an exception (Invalid opcode: 0xc8)",
      )
  }

  @Test
  fun parseErrorLineaEstimateGasRequestReturnErrorResponse() {
    val httpService = minerNode.nodeRequests().web3jService as HttpService
    val httpClient = HttpClient.newHttpClient()
    val badJsonRequest = HttpRequest.newBuilder(URI.create(httpService.url))
      .headers("Content-Type", "application/json")
      .POST(
        HttpRequest.BodyPublishers.ofString(
          """
            {"jsonrpc":"2.0","method":"linea_estimateGas","params":[malformed json],"id":53}
            """,
        ),
      )
      .build()
    val errorResponse = httpClient.send(badJsonRequest, HttpResponse.BodyHandlers.ofString())
    assertThat(errorResponse.body())
      .isEqualTo(
        """{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}""",
      )
  }

  protected open fun assertMinGasPriceLowerBound(baseFee: Wei, estimatedMaxGasPrice: Wei) {
    val minGasPrice = minerNode.miningParameters.minTransactionGasPrice
    assertThat(estimatedMaxGasPrice).isEqualTo(minGasPrice)
  }

  class LineaEstimateGasRequest(
    private val callParams: CallParams,
    private val stateOverrides: Map<String, Map<String, String>>? = null,
  ) : Transaction<LineaEstimateGasResponse> {

    override fun execute(nodeRequests: NodeRequests): LineaEstimateGasResponse {
      return try {
        Request(
          "linea_estimateGas",
          listOf(callParams, stateOverrides),
          nodeRequests.web3jService,
          LineaEstimateGasResponse::class.java,
        ).send()
      } catch (e: IOException) {
        throw RuntimeException(e)
      }
    }
  }

  class LineaEstimateGasResponse : Response<LineaEstimateGasResponseData>() {
    @JsonDeserialize(using = LineaEstimateGasResponseDeserializer::class)
    override fun setResult(result: LineaEstimateGasResponseData) {
      super.setResult(result)
    }
  }

  class LineaEstimateGasResponseData(val gasLimit: String, val baseFeePerGas: String, val priorityFeePerGas: String)

  class BadLineaEstimateGasRequest(
    private val badCallParams: CallParams,
  ) : Transaction<Response.Error> {

    override fun execute(nodeRequests: NodeRequests): Response.Error {
      return try {
        Request(
          "linea_estimateGas",
          listOf(badCallParams),
          nodeRequests.web3jService,
          BadLineaEstimateGasResponse::class.java,
        ).send().error
      } catch (e: IOException) {
        throw RuntimeException(e)
      }
    }

    class BadLineaEstimateGasResponse : Response<LineaEstimateGasResponseData>()
  }

  class RawEstimateGasRequest(private val callParams: CallParams) : Transaction<String> {

    override fun execute(nodeRequests: NodeRequests): String {
      return try {
        Request(
          "eth_estimateGas",
          listOf(callParams),
          nodeRequests.web3jService,
          RawEstimateGasResponse::class.java,
        ).send().result
      } catch (e: IOException) {
        throw RuntimeException(e)
      }
    }

    class RawEstimateGasResponse : Response<String>()
  }

  @JsonInclude(NON_NULL)
  data class CallParams(
    val chainId: String?,
    val from: String?,
    val nonce: String?,
    val to: String?,
    val value: String?,
    val data: String?,
    val gas: String?,
    val gasPrice: String?,
    val maxFeePerGas: String?,
    val maxPriorityFeePerGas: String?,
  )

  data class StateOverride(val account: String, val balance: String)

  class LineaEstimateGasResponseDeserializer : JsonDeserializer<LineaEstimateGasResponseData>() {
    private val objectReader: ObjectReader = jacksonObjectMapper().reader()

    override fun deserialize(
      jsonParser: JsonParser,
      deserializationContext: DeserializationContext,
    ): LineaEstimateGasResponseData? {
      return if (jsonParser.currentToken != JsonToken.VALUE_NULL) {
        objectReader.readValue(jsonParser, LineaEstimateGasResponseData::class.java)
      } else {
        null
      }
    }
  }

  companion object {
    protected const val FIXED_GAS_COST_WEI = 0
    protected const val VARIABLE_GAS_COST_WEI = 1_000_000_000
    protected const val MIN_MARGIN = 1.0
    protected const val ESTIMATE_GAS_MIN_MARGIN = 1.1
    protected val MIN_GAS_PRICE: Wei = Wei.of(1_000_000_000)
    protected const val MAX_TRANSACTION_GAS_LIMIT = 30_000_000
  }
}
