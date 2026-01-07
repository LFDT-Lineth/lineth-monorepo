/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.extradata

import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Disabled
import org.junit.jupiter.api.Test

class ExtraDataPricingTestWithoutSetMinGasPrice : ExtraDataPricingTest() {

  override fun getTestCliOptions(): List<String> {
    return getTestCommandLineOptionsBuilder().build()
  }

  override fun getTestCommandLineOptionsBuilder(): TestCommandLineOptionsBuilder {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-extra-data-pricing-enabled=", true.toString())
      .set("--plugin-linea-extra-data-set-min-gas-price-enabled=", false.toString())
  }

  @Disabled("disable since minGasPrice is not updated with this test")
  @Test
  override fun updateMinGasPriceViaExtraData() {
  }

  @Test
  fun minGasPriceNotUpdatedViaExtraData() {
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
      .isEqualTo(MIN_GAS_PRICE)
  }
}
