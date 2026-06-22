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
import org.apache.tuweni.bytes.Bytes
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.utils.Numeric

class EthSendRawTransactionSimulationModExpLimitlessTest : LineaPluginPoSTestBase() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      // set the module limits file
      .set(
        "--plugin-linea-module-limit-file-path=",
        getResourcePath("/moduleLimitsLimitless.toml"),
      )
      // enabled the ZkCounter
      .set("--plugin-linea-limitless-enabled=", "true")
      .set("--plugin-linea-tx-pool-simulation-check-api-enabled=", "true")
      .build()
  }

  @Test
  fun validModExpCallsAreAccepted() {
    val modExp = deployModExp()

    val validInputs = arrayOf(
      Bytes.EMPTY,
      Bytes.fromHexString("0000000000000000000000000000000000000000000000000000000000000000"),
      Bytes.fromHexString("000000000000000000000000000000000000000000000000000000000000013f"),
      Bytes.fromHexString("0000000000000000000000000000000000000000000000000000000000000200"),
      Bytes.fromHexString("00000000000000000000000000000000000000000000000000000000000002"),
    )

    for (i in validInputs.indices) {
      val mulmodOverflow =
        encodedCallModExp(modExp, accounts.getSecondaryBenefactor(), i, validInputs[i])

      val web3j = minerNode.nodeRequests().eth()
      val resp: EthSendTransaction =
        web3j.ethSendRawTransaction(Numeric.toHexString(mulmodOverflow)).send()
      assertThat(resp.hasError()).isFalse()

      minerNode.verify(eth.expectSuccessfulTransactionReceipt(resp.transactionHash))
    }
  }
}
