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
import org.junit.jupiter.api.Test
import org.web3j.tx.gas.DefaultGasProvider

class EstimateGasModuleLimitOverflowLimitlessTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      // set the module limits file
      .set(
        "--plugin-linea-module-limit-file-path=",
        getResourcePath("/moduleLimitsLimitless.toml"),
      )
      // enabled the ZkCounter
      .set("--plugin-linea-limitless-enabled=", "true")
      .build()
  }

  @Test
  fun estimateGasFailsForExceedingModuleLineCountTest() {
    val bls12MapFPtoG1 = deployBLS12_MAP_FP_TO_G1()

    for (i in BlsMapFpToG1OverflowSetup.invalidInputs.indices) {
      val calldata = BlsMapFpToG1OverflowSetup.encodeOverflowCall(bls12MapFPtoG1, i)

      val callParams = EstimateGasTest.CallParams(
        chainId = null,
        from = accounts.secondaryBenefactor.address,
        nonce = null,
        to = bls12MapFPtoG1.contractAddress,
        value = null,
        data = calldata,
        gas = "0",
        gasPrice = DefaultGasProvider.GAS_PRICE.toString(),
        maxFeePerGas = null,
        maxPriorityFeePerGas = null,
      )

      val reqLinea = EstimateGasTest.BadLineaEstimateGasRequest(callParams)
      val respLinea = reqLinea.execute(minerNode.nodeRequests())
      assertThat(respLinea.code).isEqualTo(-32000)
      assertThat(respLinea.message).isEqualTo(BlsMapFpToG1OverflowSetup.EXPECTED_OVERFLOW_MESSAGE)
    }
  }
}
