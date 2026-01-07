/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea

import com.google.common.base.Strings
import linea.plugin.acc.test.LineaPluginPoSTestBase
import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import net.consensys.linea.config.LineaProfitabilityCliOptions
import net.consensys.linea.config.LineaProfitabilityConfiguration
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.bouncycastle.crypto.digests.KeccakDigest
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.TransferTransaction
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import org.web3j.protocol.http.HttpService
import org.web3j.tx.RawTransactionManager
import java.math.BigInteger
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse

open class SetExtraDataTest : LineaPluginPoSTestBase() {
  protected lateinit var profitabilityConf: LineaProfitabilityConfiguration

  override fun getTestCliOptions(): List<String> {
    return getTestCommandLineOptionsBuilder().build()
  }

  protected open fun getTestCommandLineOptionsBuilder(): TestCommandLineOptionsBuilder {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-fixed-gas-cost-wei=", FIXED_GAS_COST_WEI.toString())
      .set("--plugin-linea-variable-gas-cost-wei=", VARIABLE_GAS_COST_WEI.toString())
      .set("--plugin-linea-min-margin=", MIN_MARGIN.toString())
      .set("--plugin-linea-max-tx-gas-limit=", MAX_TRANSACTION_GAS_LIMIT.toString())
      .set("--plugin-linea-extra-data-pricing-enabled=", "true")
  }

  @BeforeEach
  fun setMinGasPrice() {
    minerNode.miningParameters.setMinTransactionGasPrice(MIN_GAS_PRICE)
  }

  @BeforeEach
  fun createDefaultConfigurations() {
    profitabilityConf =
      LineaProfitabilityCliOptions.create().toDomainObject().toBuilder()
        .fixedCostWei(FIXED_GAS_COST_WEI)
        .variableCostWei(VARIABLE_GAS_COST_WEI)
        .minMargin(MIN_MARGIN)
        .build()
  }

  @Test
  fun setUnsupportedExtraDataReturnsError() {
    val unsupportedExtraData = Bytes32.ZERO

    val reqLinea = FailingLineaSetExtraDataRequest(unsupportedExtraData)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.message)
      .isEqualTo(
        "Unsupported extra data field 0x0000000000000000000000000000000000000000000000000000000000000000",
      )
  }

  @Test
  fun setTooLongExtraDataReturnsError() {
    val tooLongExtraData = Bytes.concatenate(Bytes.of(1), Bytes32.ZERO)

    val reqLinea = FailingLineaSetExtraDataRequest(tooLongExtraData)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.message).isEqualTo("Expected 32 bytes but got 33")
  }

  @Test
  fun setTooShortExtraDataReturnsError() {
    val tooShortExtraData = Bytes32.ZERO.slice(1)

    val reqLinea = FailingLineaSetExtraDataRequest(tooShortExtraData)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.message).isEqualTo("Expected 32 bytes but got 31")
  }

  @Test
  fun successfulSetExtraData() {
    val extraData =
      Bytes32.fromHexString("0x0100000000000000000000000000000000000000000000000000000000000000")

    val reqLinea = LineaSetExtraDataRequest(extraData)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea).isTrue()
  }

  @Test
  fun successfulUpdateMinGasPrice() {
    val doubledMinGasPriceKWei = MIN_GAS_PRICE.multiply(2).divide(1000)
    val hexMinGasPrice =
      Strings.padStart(doubledMinGasPriceKWei.toShortHexString().substring(2), 8, '0')
    val extraData =
      Bytes32.fromHexString(
        "0x010000000000000000${hexMinGasPrice}00000000000000000000000000000000000000",
      )

    val reqLinea = LineaSetExtraDataRequest(extraData)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea).isTrue()
    assertThat(minerNode.miningParameters.minTransactionGasPrice)
      .isEqualTo(MIN_GAS_PRICE.multiply(2))
  }

  @Test
  fun successfulUpdatePricingParameters() {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID)

    val keccakDigest = KeccakDigest(256)
    val txData = StringBuilder()
    txData.append("0x")
    for (i in 0 until 10) {
      keccakDigest.update(byteArrayOf(i.toByte()), 0, 1)
      val out = ByteArray(32)
      keccakDigest.doFinal(out, 0)
      txData.append(BigInteger(out))
    }

    val txUnprofitable =
      txManager.sendTransaction(
        MIN_GAS_PRICE.asBigInteger,
        BigInteger.valueOf((MAX_TX_GAS_LIMIT / 2).toLong()),
        credentials.address,
        txData.toString(),
        BigInteger.ZERO,
      )

    val sender = accounts.getSecondaryBenefactor()
    val recipient = accounts.createAccount("recipient")
    val transferTx: TransferTransaction = accountTransactions.createTransfer(sender, recipient, 1)
    val txHash = minerNode.execute(transferTx)

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash.toHexString()))

    // assert that tx below margin is not confirmed
    minerNode.verify(eth.expectNoTransactionReceipt(txUnprofitable.transactionHash))

    val zeroFixedCostKWei = "00000000"
    val minimalVariableCostKWei = "00000001"
    val minimalMinGasPriceKWei = "00000002"
    val extraData =
      Bytes32.fromHexString(
        "0x01${zeroFixedCostKWei}$minimalVariableCostKWei" +
          "${minimalMinGasPriceKWei}00000000000000000000000000000000000000",
      )

    val reqLinea = LineaSetExtraDataRequest(extraData)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea).isTrue()
    assertThat(minerNode.miningParameters.minTransactionGasPrice).isEqualTo(Wei.of(2000))
    // assert that tx is confirmed now
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txUnprofitable.transactionHash))
  }

  @Test
  fun parseErrorLineaEstimateGasRequestReturnErrorResponse() {
    val httpService = minerNode.nodeRequests().web3jService as HttpService
    val httpClient = HttpClient.newHttpClient()
    val badJsonRequest =
      HttpRequest.newBuilder(URI.create(httpService.url))
        .headers("Content-Type", "application/json")
        .POST(
          HttpRequest.BodyPublishers.ofString(
            """
                        {"jsonrpc":"2.0","method":"linea_setExtraData","params":[malformed json],"id":53}
            """.trimIndent(),
          ),
        )
        .build()
    val errorResponse = httpClient.send(badJsonRequest, HttpResponse.BodyHandlers.ofString())
    assertThat(errorResponse.body())
      .isEqualTo(
        """{"jsonrpc":"2.0","id":null,"error":{"code":-32700,"message":"Parse error"}}""",
      )
  }

  class LineaSetExtraDataRequest(private val extraData: Bytes32) : Transaction<Boolean> {
    override fun execute(nodeRequests: NodeRequests): Boolean {
      return Request(
        "linea_setExtraData",
        listOf(extraData.toHexString()),
        nodeRequests.web3jService,
        LineaSetExtraDataResponse::class.java,
      )
        .send()
        .result
    }
  }

  class FailingLineaSetExtraDataRequest(private val extraData: Bytes) : Transaction<Response.Error> {
    override fun execute(nodeRequests: NodeRequests): Response.Error {
      return Request(
        "linea_setExtraData",
        listOf(extraData.toHexString()),
        nodeRequests.web3jService,
        LineaSetExtraDataResponse::class.java,
      )
        .send()
        .error
    }
  }

  class LineaSetExtraDataResponse : Response<Boolean>()

  companion object {
    protected const val FIXED_GAS_COST_WEI = 0L
    protected const val VARIABLE_GAS_COST_WEI = 1_000_000_000L
    protected const val MIN_MARGIN = 1.5
    protected val MIN_GAS_PRICE: Wei = Wei.of(1_000_000)
    protected const val MAX_TRANSACTION_GAS_LIMIT = 30_000_000L
  }
}
