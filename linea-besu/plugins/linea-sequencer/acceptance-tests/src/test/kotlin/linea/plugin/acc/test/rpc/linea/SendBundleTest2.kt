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
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import org.junit.jupiter.api.Test

class SendBundleTest2 : AbstractSendBundleTest() {

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      // intentionally we do not set the deny list for bundles, but only the general one,
      // since we want to test the fallback for bundle to the default
      .set("--plugin-linea-deny-list-path=", getResourcePath("/defaultDenyList.txt"))
      .build()
  }

  @Test
  fun bundleTxRecipientOnDenyListIsNotAccepted() {
    val sender = accounts.secondaryBenefactor
    val recipient = accounts.createAccount(DENY_TO_ADDRESS)

    val tx1 = accountTransactions.createTransfer(sender, recipient, 1)

    val bundleRawTxs = arrayOf(tx1.signedTransactionData())

    val sendBundleRequest =
      SendBundleRequest(BundleParams(bundleRawTxs, Integer.toHexString(1)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isTrue()
    assertThat(sendBundleResponse.error.message)
      .isEqualTo(
        "Invalid transaction in bundle: hash " +
          "0xfb47ad29ecf898031bae210263198385f35818d4d154dc752d942a42acabc0cc, " +
          "reason: recipient 0xf17f52151ebef6c7334fad080c5704d77216b732 is blocked as " +
          "appearing on the SDN or other legally prohibited list",
      )
  }

  @Test
  fun bundleTxFromOnDenyListIsNotAccepted() {
    val sender = Account.fromPrivateKey(ethTransactions, "denied", DENY_FROM_PRIVATE_KEY)
    val recipient = accounts.primaryBenefactor

    val tx1 = accountTransactions.createTransfer(sender, recipient, 1)

    val bundleRawTxs = arrayOf(tx1.signedTransactionData())

    val sendBundleRequest =
      SendBundleRequest(BundleParams(bundleRawTxs, Integer.toHexString(1)))
    val sendBundleResponse = sendBundleRequest.execute(minerNode.nodeRequests())

    assertThat(sendBundleResponse.hasError()).isTrue()
    assertThat(sendBundleResponse.error.message)
      .isEqualTo(
        "Invalid transaction in bundle: hash 0xd631d31a09e865fcd0d86a7f7763747ece057f9f3a63350bb56a206051020a71," +
          " reason: sender 0x44b30d738d2dec1952b92c091724e8aedd52b9b2 " +
          "is blocked as appearing on the SDN or other legally prohibited list",
      )
  }

  companion object {
    private val DENY_TO_ADDRESS =
      Address.fromHexString("0xf17f52151EbEF6C7334FAD080c5704D77216b732")

    // Address 0x44b30d738d2dec1952b92c091724e8aedd52b9b2
    private const val DENY_FROM_PRIVATE_KEY =
      "0xf326e86ba27e2286725a154922094f02573f4921a25a27046b74ec90e653438e"
  }
}
