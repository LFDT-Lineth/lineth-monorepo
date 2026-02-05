/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea
import linea.plugin.acc.test.LineaPluginPoSTestBase
import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import org.junit.jupiter.api.Disabled
import org.junit.jupiter.api.Test
import org.web3j.tx.gas.DefaultGasProvider
import java.math.BigInteger

@Disabled("The sequencer is using the ZkCounter as of now")
class EstimateGasModuleLimitOverflowTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set(
        "--plugin-linea-module-limit-file-path=",
        getResourcePath("/txOverflowModuleLimits.toml"),
      )
      .build()
  }

  @Test
  fun estimateGasFailsForExceedingModuleLineCountTest() {
    val sender: Account = accounts.primaryBenefactor

    val dummyAdder = deployDummyAdder()
    val txData = dummyAdder.add(BigInteger.valueOf(1)).encodeFunctionCall()

    val callParams = EstimateGasTest.CallParams(
      chainId = null,
      from = sender.address,
      nonce = null,
      to = dummyAdder.contractAddress,
      value = null,
      data = txData,
      gas = "0",
      gasPrice = DefaultGasProvider.GAS_PRICE.toString(),
      maxFeePerGas = null,
      maxPriorityFeePerGas = null,
    )

    val reqLinea = EstimateGasTest.BadLineaEstimateGasRequest(callParams)
    val respLinea = reqLinea.execute(minerNode.nodeRequests())
    assertThat(respLinea.code).isEqualTo(-32000)
    assertThat(respLinea.message)
      .isEqualTo("Transaction line count for module HUB=216 is above the limit 52")
  }
}
