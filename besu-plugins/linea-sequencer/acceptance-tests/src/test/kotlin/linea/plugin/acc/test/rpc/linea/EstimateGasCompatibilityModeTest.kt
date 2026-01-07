/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea

import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import org.junit.jupiter.api.Test
import java.math.BigDecimal
import java.math.RoundingMode

class EstimateGasCompatibilityModeTest : EstimateGasTest() {

  override fun getTestCliOptions(): List<String> {
    return testCommandLineOptionsBuilder
      .set("--plugin-linea-estimate-gas-compatibility-mode-enabled=", "true")
      .set(
        "--plugin-linea-estimate-gas-compatibility-mode-multiplier=",
        PRICE_MULTIPLIER.toPlainString(),
      )
      .build()
  }

  override fun assertMinGasPriceLowerBound(baseFee: Wei, estimatedMaxGasPrice: Wei) {
    // since we are in compatibility mode, we want to check that returned profitable priority fee is
    // the min priority fee per gas * multiplier + base fee
    val minGasPrice = minerNode.miningParameters.minTransactionGasPrice
    val minPriorityFee = minGasPrice.subtract(baseFee)
    val compatibilityMinPriorityFee = Wei.of(
      PRICE_MULTIPLIER
        .multiply(BigDecimal(minPriorityFee.asBigInteger))
        .setScale(0, RoundingMode.CEILING)
        .toBigInteger(),
    )

    // since we are in compatibility mode, we want to check that returned profitable priority fee is
    // the min priority fee per gas * multiplier + base fee
    val expectedMaxGasPrice = baseFee.add(compatibilityMinPriorityFee)
    assertThat(estimatedMaxGasPrice).isEqualTo(expectedMaxGasPrice)
  }

  @Test
  fun lineaEstimateGasPriorityFeeMinGasPriceLowerBound() {
    val sender: Account = accounts.secondaryBenefactor

    val callParams = CallParams(
      chainId = null,
      from = sender.address,
      nonce = null,
      to = null,
      value = "",
      data = "",
      gas = "0",
      gasPrice = null,
      maxFeePerGas = null,
      maxPriorityFeePerGas = null,
    )

    val reqLinea = LineaEstimateGasRequest(callParams)
    val respLinea = reqLinea.execute(minerNode.nodeRequests()).result

    val baseFee = Wei.fromHexString(respLinea.baseFeePerGas)
    val estimatedPriorityFee = Wei.fromHexString(respLinea.priorityFeePerGas)
    val estimatedMaxGasPrice = baseFee.add(estimatedPriorityFee)

    assertMinGasPriceLowerBound(baseFee, estimatedMaxGasPrice)
  }

  companion object {
    private val PRICE_MULTIPLIER = BigDecimal.valueOf(1.2)
  }
}
