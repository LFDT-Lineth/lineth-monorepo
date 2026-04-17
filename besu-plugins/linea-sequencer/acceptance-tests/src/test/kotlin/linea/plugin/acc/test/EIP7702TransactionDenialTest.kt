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
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.junit.jupiter.api.Assertions.assertThrows
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j

/**
 * Tests that verify EIP-7702 DELEGATE_CODE transactions are correctly rejected from the transaction
 * pool at the RPC/P2P level when explicitly disabled via CLI option (via
 * LineaTransactionPoolValidatorPlugin's TransactionTypeValidator).
 */
class EIP7702TransactionDenialTest : LineaPluginPoSTestBase() {
  private lateinit var web3j: Web3j
  private lateinit var credentials: Credentials
  private lateinit var recipient: String

  override fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder()
      .set("--plugin-linea-delegate-code-tx-enabled=", "false")
      .build()
  }

  @BeforeEach
  override fun setup() {
    super.setup()
    web3j = minerNode.nodeRequests().eth()
    credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    recipient = accounts.secondaryBenefactor.address
  }

  @Test
  fun eip7702TransactionIsRejectedFromTransactionPool() {
    // Act - Send an EIP7702 DELEGATE_CODE transaction to transaction pool and expect it to be
    // rejected
    // We use 'minerNode.execute' here which throw us a RuntimeException directly
    val exception = assertThrows(RuntimeException::class.java) {
      sendRawEIP7702Transaction(web3j, credentials, recipient)
    }
    // No need to build new block.

    // Assert
    assertThat(exception.message).contains(TransactionTypeValidationErrors.DELEGATE_CODE_TX_NOT_ALLOWED)
  }
}
