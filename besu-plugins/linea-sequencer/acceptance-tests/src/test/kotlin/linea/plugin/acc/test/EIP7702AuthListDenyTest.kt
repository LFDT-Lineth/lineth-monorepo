/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j

class EIP7702AuthListDenyTest : LineaPluginPoSTestBase() {
  private lateinit var web3j: Web3j

  // Address derived from Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY, listed in denyList.txt
  private val deniedAddress = "0x627306090abab3a6e1400e9345bc60c78a8bef57"

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-deny-list-path=", getResourcePath("/denyList.txt"))
      .set("--plugin-linea-delegate-code-tx-enabled=", "true")
      .build()
  }

  @BeforeEach
  override fun setup() {
    super.setup()
    web3j = minerNode.nodeRequests().eth()
  }

  @Test
  fun eip7702TxWithDeniedAuthorityIsRejected() {
    val sender = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val deniedAuthSigner = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY)

    val response = sendEIP7702WithSeparateAuth(
      web3j = web3j,
      senderCredentials = sender,
      authSignerCredentials = deniedAuthSigner,
      delegationAddress = Address.fromHexStringStrict(sender.address),
    )

    assertThat(response.transactionHash).isNull()
    assertThat(response.error.message).isEqualTo(
      "authorization authority $deniedAddress is blocked as appearing on " +
        "the SDN or other legally prohibited list",
    )
  }

  @Test
  fun eip7702TxWithDeniedDelegationAddressIsRejected() {
    val sender = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)

    val response = sendEIP7702WithSeparateAuth(
      web3j = web3j,
      senderCredentials = sender,
      authSignerCredentials = sender,
      delegationAddress = Address.fromHexStringStrict(deniedAddress),
    )

    assertThat(response.transactionHash).isNull()
    assertThat(response.error.message).isEqualTo(
      "authorization address $deniedAddress is blocked as appearing on " +
        "the SDN or other legally prohibited list",
    )
  }

  @Test
  fun eip7702TxWithCleanAuthListIsAccepted() {
    val sender = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)

    val response = sendEIP7702WithSeparateAuth(
      web3j = web3j,
      senderCredentials = sender,
      authSignerCredentials = sender,
      delegationAddress = Address.fromHexStringStrict(sender.address),
    )

    assertThat(response.error).isNull()
    assertThat(response.transactionHash).isNotNull()
  }
}
