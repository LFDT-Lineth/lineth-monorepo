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
import linea.plugin.acc.test.rpc.assertSuccessResponse
import linea.plugin.acc.test.tests.web3j.generated.MulmodExecutor
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import org.junit.jupiter.api.Test
import org.web3j.protocol.core.methods.response.TransactionReceipt
import java.math.BigInteger

class BundleSelectionTimeoutTest : AbstractSendBundleTest() {

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

  private fun generateMulmodCalls(
    account: Account,
    mulmodExecutor: MulmodExecutor,
    startNonce: Int,
    endNonce: Int,
    numberOfMulmodIterations: Int,
    gasLimit: Int,
  ): Array<MulmodCall> {
    return (startNonce..endNonce).map { nonce ->
      mulmodOperation(
        mulmodExecutor,
        account,
        nonce,
        numberOfMulmodIterations,
        BigInteger.valueOf(gasLimit.toLong()),
      )
    }.toTypedArray()
  }

  private fun createSendBundleRequest(calls: Array<MulmodCall>, blockNumber: Long): SendBundleRequest {
    val rawTxs = calls.map { it.rawTx }.toTypedArray()
    return SendBundleRequest(
      BundleParams(rawTxs, blockNumber = blockNumber.toString(16)),
    )
  }

  @Test
  fun singleBundleSelectionTimeout() {
    val mulmodExecutor = deployMulmodExecutor()
    val newAccounts = createAccounts(4, 5)
    // stop automatic block production to
    // ensure bundle and transfers are evaluated in the same block
    buildBlocksInBackground = false

    val callsBigBundle = generateMulmodCalls(
      newAccounts[0],
      mulmodExecutor,
      0,
      30,
      2_000,
      MAX_TX_GAS_LIMIT / 10,
    )
    val callsSmallBundle = generateMulmodCalls(
      newAccounts[1],
      mulmodExecutor,
      0,
      3,
      2,
      MAX_TX_GAS_LIMIT / 10,
    )

    val previousHeadBlockNumber = getLatestBlockNumber()
    val sendBundleRequest = createSendBundleRequest(callsBigBundle, previousHeadBlockNumber + 1L)
    val sendSmallBundleRequest = createSendBundleRequest(callsSmallBundle, previousHeadBlockNumber + 2L)

    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())
    sendBundleResponse.assertSuccessResponse()

    val sendSmallBundleResponse = sendSmallBundleRequest.execute(minerNode.nodeRequests())
    sendSmallBundleResponse.assertSuccessResponse()

    val transferTxHash = accountTransactions
      .createTransfer(newAccounts[2], accounts.primaryBenefactor, 1)
      .execute(minerNode.nodeRequests())

    buildNewBlockAndWait()
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()))

    // none of the big bundle txs must be included in a block
    callsBigBundle
      .map { it.txHash }
      .forEach { txHash ->
        minerNode.verify(eth.expectNoTransactionReceipt(txHash))
      }

    val transfer2TxHash = accountTransactions
      .createTransfer(newAccounts[3], accounts.primaryBenefactor, 1)
      .execute(minerNode.nodeRequests())

    buildNewBlockAndWait()
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transfer2TxHash.toHexString()))
    // all tx in small bundle where included in a block
    callsSmallBundle
      .map { it.txHash }
      .forEach { txHash ->
        minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash))
      }
  }

  @Test
  fun multipleBundleSelectionTimeout() {
    val mulmodExecutor = deployMulmodExecutor()

    val calls = generateMulmodCalls(
      accounts.primaryBenefactor,
      mulmodExecutor,
      1,
      10,
      2_000,
      MAX_TX_GAS_LIMIT / 10,
    )

    val rawTxs = calls.map { it.rawTx }.toTypedArray()

    val sendBundleRequestSmall = SendBundleRequest(
      BundleParams(rawTxs.copyOfRange(0, 1), Integer.toHexString(2)),
    )

    // this bundle is meant to go in timeout during its selection
    val sendBundleRequestBig1 = SendBundleRequest(
      BundleParams(rawTxs.copyOfRange(1, 10), Integer.toHexString(2)),
    )

    // second bundle contains one tx only to be fast to execute,
    // and ensure timeout occurs on the 2nd bundle and following are not even considered.
    // We are sending a bunch of bundles instead of just one to reproduce what happened in
    // production, where each following bundle where not skipped and would take ~200ms
    // to be not selected, due to the fact the first tx in the bundle was executed.
    val followingBundleCount = 5
    val followingSendBundleRequests = Array(followingBundleCount) { i ->
      SendBundleRequest(
        BundleParams(rawTxs.copyOfRange(1, 2 + i), Integer.toHexString(2)),
      )
    }

    val sendBundleResponseSmall = sendBundleRequestSmall.execute(minerNode.nodeRequests())
    val sendBundleResponseBig1 = sendBundleRequestBig1.execute(minerNode.nodeRequests())
    val followingBundleResponses = followingSendBundleRequests
      .map { req -> req.execute(minerNode.nodeRequests()) }

    val transferTxHash = accountTransactions
      .createTransfer(accounts.secondaryBenefactor, accounts.primaryBenefactor, 1)
      .execute(minerNode.nodeRequests())

    sendBundleResponseSmall.assertSuccessResponse()
    sendBundleResponseBig1.assertSuccessResponse()
    followingBundleResponses.forEach { resp -> resp.assertSuccessResponse() }

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.toHexString()))
    val transferReceipt = ethTransactions.getTransactionReceipt(transferTxHash.toHexString())
    assertThat(transferReceipt.execute(minerNode.nodeRequests()))
      .isPresent
      .map(TransactionReceipt::getBlockNumber)
      .contains(BigInteger.TWO)

    // first bundle is successful
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(calls[0].txHash))

    // following bundles are not selected
    calls
      .drop(1)
      .map { it.txHash }
      .forEach { txHash -> minerNode.verify(eth.expectNoTransactionReceipt(txHash)) }
    val log = getLog()
    assertThat(log)
      .withFailMessage {
        "Expected to find PLUGIN_SELECTION_TIMEOUT in logs, " +
          "but bundle ${sendBundleResponseBig1.result.bundleHash} was not included for some other reason"
      }
      .contains("PLUGIN_SELECTION_TIMEOUT")
    assertThat(log)
      .contains(
        "Bundle selection interrupted while processing bundle ${sendBundleResponseBig1.result.bundleHash}",
      )
  }
}
