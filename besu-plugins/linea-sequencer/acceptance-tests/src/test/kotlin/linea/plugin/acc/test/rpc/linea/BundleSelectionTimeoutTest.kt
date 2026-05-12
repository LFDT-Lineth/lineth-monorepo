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
    val newAccounts = createAccounts(numAccounts = 4, initialBalanceEther = 5)
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
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.bytes.toHexString()))

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
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transfer2TxHash.bytes.toHexString()))
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
    // Stop background block production so it cannot fire a competing block-2
    // creation attempt alongside the explicit one below. The background scheduler
    // uses the default 5s build window, which permits ~10 selection retries — any
    // one of which can complete the bundle inside the plugin selection budget.
    buildBlocksInBackground = false

    // Mirror singleBundleSelectionTimeout's per-tx work (2_000 iter,
    // MAX_TX_GAS_LIMIT / 10). The plugin selection budget is ~100ms (2% of the 5s
    // slot, configured in LineaPluginTestBase). The bundle must do enough work to
    // exceed that budget, and each tx must be small enough that the timeout fires
    // mid-bundle.
    //
    // Total compute time alone doesn't decide whether the timeout fires — total
    // wall-clock time does. Many small txs is more reliable than a few big ones
    // for the same total compute, because:
    //   1. Per-tx selector overhead (signature recovery, module-limit check,
    //      profitability calc, HUB tracing setup) is paid once per tx and doesn't
    //      shrink under cache warmup or JIT, so 30 small txs accumulate ~3x more
    //      fixed overhead than 9 large ones.
    //   2. The cache/JIT-sensitive compute portion is a smaller fraction of total
    //      time, so variance from warm caches or a fast runner can't drop the
    //      bundle below the budget.
    // The previous shape (9 txs × 7_000 iter, MAX_TX_GAS_LIMIT / 3) made each tx
    // large enough that on a fast runner the whole bundle could finish inside
    // the budget — no timeout, calls[1] gets committed, test fails.
    val calls = generateMulmodCalls(
      accounts.primaryBenefactor,
      mulmodExecutor,
      1,
      31,
      2_000,
      MAX_TX_GAS_LIMIT / 10,
    )
    val bundle1Calls = calls.copyOfRange(0, 1)
    val bundle2Calls = calls.copyOfRange(1, 31)

    val sendBundleRequestSmall = SendBundleRequest(
      BundleParams(bundle1Calls.map { it.rawTx }.toTypedArray(), Integer.toHexString(2)),
    )

    // this bundle is meant to go in timeout during its selection
    val sendBundleRequestBig1 = SendBundleRequest(
      BundleParams(bundle2Calls.map { it.rawTx }.toTypedArray(), Integer.toHexString(2)),
    )

    // second bundle contains one tx only to be fast to execute,
    // and ensure timeout occurs on the 2nd bundle and following are not even considered.
    // We are sending a bunch of bundles instead of just one to reproduce what happened in
    // production, where each following bundle were not skipped and would take ~200ms
    // to be not selected, due to the fact the first tx in the bundle was executed.
    val followingBundleCount = 5
    val followingBundlesCalls = (0..followingBundleCount)
      .map { i -> calls.copyOfRange(1, 2 + i) }
    val followingSendBundleRequests = followingBundlesCalls
      .map { SendBundleRequest(BundleParams(it.map { it.rawTx }.toTypedArray(), Integer.toHexString(2))) }

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

    // Build block 2 with a short block-building window so Besu's iterative selector
    // gets only a single selection pass. The default 5s window allows ~10 selection
    // retries; each retry has its own ~100ms plugin selection budget, so a fast
    // runner has many independent chances to complete the bundle inside the budget
    // and include the tx that should have timed out. A short window collapses that
    // to a single pass.
    buildNewBlockAndWait(300L)

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(transferTxHash.bytes.toHexString()))
    val transferReceipt = ethTransactions.getTransactionReceipt(transferTxHash.bytes.toHexString())
    assertThat(transferReceipt.execute(minerNode.nodeRequests()))
      .isPresent
      .map(TransactionReceipt::getBlockNumber)
      .contains(BigInteger.TWO)

    // first bundle is successful
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(calls[0].txHash))

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

    // following bundles not selected
    assertBundleTxNotIncluded(bundle2Calls, "big bundle 2")
    followingBundlesCalls.forEachIndexed { index, bundleRawTxs ->
      assertBundleTxNotIncluded(bundleRawTxs, "following bundle ${index + 1}")
    }
  }

  fun assertBundleTxNotIncluded(
    bundleCalls: Array<MulmodCall>,
    bundleLabel: String,
  ) {
    bundleCalls
      .forEachIndexed { index, call ->
        runCatching { minerNode.verify(eth.expectNoTransactionReceipt(call.txHash)) }
          .onFailure {
            throw AssertionError("bundle $bundleLabel, tx $index was included but expected to timeout", it)
          }
      }
  }
}
