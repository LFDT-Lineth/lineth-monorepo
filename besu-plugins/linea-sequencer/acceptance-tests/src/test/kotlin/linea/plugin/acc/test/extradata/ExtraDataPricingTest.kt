/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.extradata

import linea.plugin.acc.test.LineaPluginPoSTestBase
import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import net.consensys.linea.metrics.LineaMetricCategory.PRICING_CONF
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.hyperledger.besu.tests.acceptance.dsl.transaction.NodeRequests
import org.hyperledger.besu.tests.acceptance.dsl.transaction.Transaction
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.protocol.core.Request
import org.web3j.utils.Numeric
import java.io.IOException
import java.math.BigInteger
import java.util.AbstractMap

open class ExtraDataPricingTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return getTestCommandLineOptionsBuilder().build()
  }

  protected open fun getTestCommandLineOptionsBuilder(): TestCommandLineOptionsBuilder {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-extra-data-pricing-enabled=", true.toString())
  }

  @Test
  open fun updateMinGasPriceViaExtraData() {
    minerNode.miningParameters.minTransactionGasPrice = MIN_GAS_PRICE
    val doubleMinGasPrice = MIN_GAS_PRICE.multiply(2)

    val extraData =
      createExtraDataPricingField(
        0,
        MIN_GAS_PRICE.toLong() / WEI_IN_KWEI,
        doubleMinGasPrice.toLong() / WEI_IN_KWEI,
      )
    val reqSetExtraData = MinerSetExtraDataRequest(extraData)
    val respSetExtraData = reqSetExtraData.execute(minerNode.nodeRequests())

    assertThat(respSetExtraData).isTrue()

    val sender = accounts.secondaryBenefactor
    val recipient = accounts.createAccount("recipient")

    val transferTx = accountTransactions.createTransfer(sender, recipient, 1)
    val txHash = minerNode.execute(transferTx)

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash.toHexString()))

    assertThat(minerNode.miningParameters.minTransactionGasPrice)
      .isEqualTo(doubleMinGasPrice)
  }

  @Test
  fun updateProfitabilityParamsViaExtraData() {
    val web3j = minerNode.nodeRequests().eth()
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.createAccount("recipient")
    minerNode.miningParameters.minTransactionGasPrice = MIN_GAS_PRICE

    val extraData =
      createExtraDataPricingField(
        MIN_GAS_PRICE.multiply(2).toLong() / WEI_IN_KWEI,
        MIN_GAS_PRICE.toLong() / WEI_IN_KWEI,
        MIN_GAS_PRICE.toLong() / WEI_IN_KWEI,
      )
    val reqSetExtraData = MinerSetExtraDataRequest(extraData)
    val respSetExtraData = reqSetExtraData.execute(minerNode.nodeRequests())

    assertThat(respSetExtraData).isTrue()

    // when this first tx is mined the above extra data pricing will have effect on following txs
    val profitableTx =
      accountTransactions.createTransfer(sender, recipient, 1)
    val profitableTxHash = minerNode.execute(profitableTx)

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(profitableTxHash.toHexString()))

    // this tx will be evaluated with the previously set extra data pricing to be unprofitable
    val unprofitableTx =
      RawTransaction.createTransaction(
        BigInteger.ZERO,
        MIN_GAS_PRICE.asBigInteger,
        BigInteger.valueOf(21000),
        recipient.address,
        "",
      )

    val signedUnprofitableTx =
      TransactionEncoder.signMessage(
        unprofitableTx,
        Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY),
      )

    val signedUnprofitableTxResp =
      web3j.ethSendRawTransaction(Numeric.toHexString(signedUnprofitableTx)).send()

    assertThat(signedUnprofitableTxResp.hasError()).isTrue()
    assertThat(signedUnprofitableTxResp.error.message).isEqualTo("Gas price too low")

    assertThat(getTxPoolContent()).isEmpty()

    val fixedCostMetric =
      getMetricValue(PRICING_CONF, "values", listOf(AbstractMap.SimpleEntry("field", "fixed_cost_wei")))

    assertThat(fixedCostMetric)
      .isEqualTo(MIN_GAS_PRICE.multiply(2).asBigInteger.toDouble())

    val variableCostMetric =
      getMetricValue(PRICING_CONF, "values", listOf(AbstractMap.SimpleEntry("field", "variable_cost_wei")))

    assertThat(variableCostMetric).isEqualTo(MIN_GAS_PRICE.asBigInteger.toDouble())

    val ethGasPriceMetric =
      getMetricValue(PRICING_CONF, "values", listOf(AbstractMap.SimpleEntry("field", "eth_gas_price_wei")))

    assertThat(ethGasPriceMetric).isEqualTo(MIN_GAS_PRICE.asBigInteger.toDouble())
  }

  class MinerSetExtraDataRequest(private val extraData: Bytes32) : Transaction<Boolean> {

    override fun execute(nodeRequests: NodeRequests): Boolean {
      return try {
        Request(
          "miner_setExtraData",
          listOf(extraData.toHexString()),
          nodeRequests.web3jService,
          MinerSetExtraDataResponse::class.java,
        )
          .send()
          .result
      } catch (e: IOException) {
        throw RuntimeException(e)
      }
    }

    class MinerSetExtraDataResponse : org.web3j.protocol.core.Response<Boolean>()
  }

  companion object {
    @JvmStatic
    protected val MIN_GAS_PRICE: Wei = Wei.of(1_000_000_000)

    @JvmStatic
    protected val WEI_IN_KWEI = 1000
  }
}
