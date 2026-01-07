/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea

import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import linea.plugin.acc.test.rpc.SendBundleRequest
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.math.BigInteger

class SendBundleMaxBlockGasTest : AbstractSendBundleTest() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-max-bundle-block-gas=", BUNDLE_BLOCK_GAS_LIMIT.toString())
      .build()
  }

  @Test
  fun maxBlockGasForBundlesIsRespected() {
    val token = deployAcceptanceTestToken()

    val numOfTransfers = 2

    // each token transfer has a gas limit of 100k so the bundle does not fit in the max block gas
    // reserved for bundles
    val tokenTransfers = Array(numOfTransfers) { i ->
      transferTokens(
        token,
        accounts.primaryBenefactor,
        i + 1,
        accounts.createAccount("recipient $i"),
        1,
      )
    }

    val tokenTransferBundleRawTxs = tokenTransfers.map { it.rawTx }.toTypedArray()

    val tokenTransferSendBundleRequest =
      SendBundleRequest(BundleParams(tokenTransferBundleRawTxs, Integer.toHexString(2)))
    val tokenTransferSendBundleResponse =
      tokenTransferSendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(tokenTransferSendBundleResponse.hasError()).isFalse()
    assertThat(tokenTransferSendBundleResponse.result.bundleHash).isNotBlank()

    // while 2 simple transfers each with a gas limit of 21k fit
    val tx1 = accountTransactions.createTransfer(
      accounts.secondaryBenefactor,
      accounts.primaryBenefactor,
      1,
    )
    val tx2 = accountTransactions.createTransfer(
      accounts.secondaryBenefactor,
      accounts.primaryBenefactor,
      1,
    )

    val bundleRawTxs = arrayOf(tx1.signedTransactionData(), tx2.signedTransactionData())

    val sendBundleRequest =
      SendBundleRequest(BundleParams(bundleRawTxs, Integer.toHexString(2)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isFalse()
    assertThat(sendBundleResponse.result.bundleHash).isNotBlank()

    // verify simple transfers are mined
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx1.transactionHash()))
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(tx2.transactionHash()))

    // but token transfers are not
    tokenTransfers.forEach { tokenTransfer ->
      minerNode.verify(eth.expectNoTransactionReceipt(tokenTransfer.txHash))
    }
  }

  companion object {
    private val BUNDLE_BLOCK_GAS_LIMIT = BigInteger.valueOf(100_000L)
  }
}
