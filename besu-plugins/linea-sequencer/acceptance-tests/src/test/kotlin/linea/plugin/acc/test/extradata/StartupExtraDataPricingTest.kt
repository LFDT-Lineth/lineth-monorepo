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
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.datatypes.Wei
import org.junit.jupiter.api.Test
import java.util.Optional

class StartupExtraDataPricingTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return getTestCommandLineOptionsBuilder().build()
  }

  protected fun getTestCommandLineOptionsBuilder(): TestCommandLineOptionsBuilder {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-extra-data-pricing-enabled=", true.toString())
  }

  override fun maybeCustomGenesisExtraData(): Optional<Bytes32> {
    val genesisExtraData =
      createExtraDataPricingField(
        0,
        VARIABLE_GAS_COST.toLong() / WEI_IN_KWEI,
        MIN_GAS_PRICE.toLong() / WEI_IN_KWEI,
      )

    return Optional.of(genesisExtraData)
  }

  @Test
  fun minGasPriceSetFromChainHeadExtraDataAtStartup() {
    // at startup the min gas price should be set from the current chain head block extra data
    assertThat(minerNode.miningParameters.minTransactionGasPrice)
      .isEqualTo(MIN_GAS_PRICE)

    val sender = accounts.secondaryBenefactor
    val recipient = accounts.createAccount("recipient")

    val transferTx = accountTransactions.createTransfer(sender, recipient, 1)
    val txHash = minerNode.execute(transferTx)

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash.toHexString()))
  }

  companion object {
    private val VARIABLE_GAS_COST: Wei = Wei.of(1_200_300_000)
    private val MIN_GAS_PRICE: Wei = VARIABLE_GAS_COST.divide(2)
    private const val WEI_IN_KWEI = 1000
  }
}
