/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test.rpc.linea

import linea.plugin.acc.test.FakeChainSecurityPolicyTxValidatorPlugin
import linea.plugin.acc.test.RawTransactionHelper
import linea.plugin.acc.test.TestCommandLineOptionsBuilder
import linea.plugin.acc.test.rpc.ForcedTransactionParam
import linea.plugin.acc.test.rpc.GetForcedTransactionInclusionStatusRequest
import linea.plugin.acc.test.rpc.SendForcedRawTransactionRequest
import net.consensys.linea.config.LineaForcedTransactionCliOptions
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.crypto.Hash
import kotlin.time.Duration.Companion.milliseconds

class ForcedTransactionChainSecurityPolicyTest : AbstractForcedTransactionTest() {
  private val chainSecurityViolationBeforeDeadlineInclusionAllowance = 3

  override fun getRequestedPlugins(): List<String> =
    DEFAULT_REQUESTED_PLUGINS + FakeChainSecurityPolicyTxValidatorPlugin.PLUGIN_NAME

  override fun getTestCliOptions(): List<String> =
    TestCommandLineOptionsBuilder()
      .set("--plugin-linea-module-limit-file-path=", getResourcePath("/moduleLimitsLimitless.toml"))
      .set("--plugin-linea-limitless-enabled=", "true")
      .set(
        LineaForcedTransactionCliOptions.FORCED_TX_CHAIN_SECURITY_VIOLATION_BEFORE_DEADLINE_INCLUSION_ALLOWANCE + "=",
        chainSecurityViolationBeforeDeadlineInclusionAllowance.toString(),
      )
      .build()

  @BeforeEach
  override fun setup() {
    FakeChainSecurityPolicyTxValidatorPlugin.reset()
    super.setup()
  }

  @Test
  fun `should include forced tx when not blocked by security policy`() {
    buildBlocksInBackground = false

    val rawTx = createSignedTransfer(accounts.primaryBenefactor, accounts.secondaryBenefactor, 0)
    val ftxNumber = nextForcedTxNumber()

    val response = SendForcedRawTransactionRequest(
      listOf(ForcedTransactionParam(ftxNumber, rawTx, DEADLINE)),
    ).execute(minerNode.nodeRequests())
    assertThat(response.hasError()).isFalse()

    buildNewBlockAndWait(500.milliseconds)

    assertThat(ftxInclusionStatus(ftxNumber)).isEqualTo("Included")
  }

  @Test
  fun `should not mine tx rejected by security layer before deadline toleration window`() {
    val account1 = accounts.primaryBenefactor
    val account2 = accounts.secondaryBenefactor
    val account3 = accounts.thirdBenefactor
    val recipient = accounts.createAccount("recipient")

    val ftx1 = RawTransactionHelper.createSignedTransfer(CHAIN_ID, account1, recipient, 0)
    val ftx2 = RawTransactionHelper.createSignedTransfer(CHAIN_ID, account2, recipient, 0)
    val ftx3 = RawTransactionHelper.createSignedTransfer(CHAIN_ID, account3, recipient, 0)
    val ftx4 = RawTransactionHelper.createSignedTransfer(CHAIN_ID, account1, recipient, 1)
    val ftx5 = RawTransactionHelper.createSignedTransfer(CHAIN_ID, account2, recipient, 1)

    val ftxs = listOf(ftx1, ftx2, ftx3, ftx4, ftx5)
    val ftxHashes = ftxs.map(Hash::sha3)
    val ftxNumbers = ftxs.map { nextForcedTxNumber() }

    // Mark ftx3 as rejected by 3rd SecurityTransaction validator
    FakeChainSecurityPolicyTxValidatorPlugin.blockBySender(account3.address)

    val deadline = 10L
    val sendResponse = SendForcedRawTransactionRequest(
      listOf(
        ForcedTransactionParam(ftxNumbers[0], ftx1, deadline),
        ForcedTransactionParam(ftxNumbers[1], ftx2, deadline),
        ForcedTransactionParam(ftxNumbers[2], ftx3, deadline),
        ForcedTransactionParam(ftxNumbers[3], ftx4, deadline),
        ForcedTransactionParam(ftxNumbers[4], ftx5, deadline),
      ),
    ).execute(minerNode.nodeRequests())

    assertThat(sendResponse.hasError()).isFalse()
    assertThat(sendResponse.result).hasSize(5)

    buildNewBlockAndWait(blockBuildingTime = 300.milliseconds)
    buildNewBlockAndWait(blockBuildingTime = 300.milliseconds)
    buildNewBlockAndWait(blockBuildingTime = 300.milliseconds)
    // assert that first ellegible tx get included but the rest are on hold because of the security layer rejection
    assertThat(getFtxInclusionStatus(ftxNumbers))
      .containsExactly("Included", "Included", null, null, null)

    await()
      .untilAsserted {
        val blockNumber = buildNewBlockAndWait(blockBuildingTime = 300.milliseconds)
        assertThat(blockNumber).isEqualTo(deadline - chainSecurityViolationBeforeDeadlineInclusionAllowance - 1)
      }

    minerNode.verify(eth.expectSuccessfulTransactionReceipt(ftxHashes[0]))
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(ftxHashes[1]))
    assertThat(findTransactionReceipt(ftxHashes[2])).isNull()
    assertThat(findTransactionReceipt(ftxHashes[3])).isNull()
    assertThat(findTransactionReceipt(ftxHashes[4])).isNull()
    assertThat(getFtxInclusionStatus(ftxNumbers))
      .containsExactly("Included", "Included", null, null, null)

    // mine 1 extra block to be within (deadline - chainSecurityViolationBeforeDeadlineInclusionAllowance)
    buildNewBlockAndWait(blockBuildingTime = 300.milliseconds)
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(ftxHashes[2]))
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(ftxHashes[3]))
    minerNode.verify(eth.expectSuccessfulTransactionReceipt(ftxHashes[4]))
    assertThat(getFtxInclusionStatus(ftxNumbers))
      .containsExactly("Included", "Included", "Included", "Included", "Included")
  }

  fun getFtxInclusionStatus(ftxNumbers: List<Long>): List<String?> {
    return ftxNumbers.map { ftxNumber ->
      GetForcedTransactionInclusionStatusRequest(ftxNumber)
        .execute(minerNode.nodeRequests())
        .let { response ->
          response.result?.inclusionResult
            ?: response.error?.message?.let { throw RuntimeException(it) }
        }
    }
  }

  private fun ftxInclusionStatus(ftxNumber: Long): String? =
    GetForcedTransactionInclusionStatusRequest(ftxNumber)
      .execute(minerNode.nodeRequests())
      .let { response ->
        response.result?.inclusionResult
          ?: response.error?.message?.let { throw RuntimeException(it) }
      }

  companion object {
    private const val DEADLINE = 1_000_000L
  }
}
