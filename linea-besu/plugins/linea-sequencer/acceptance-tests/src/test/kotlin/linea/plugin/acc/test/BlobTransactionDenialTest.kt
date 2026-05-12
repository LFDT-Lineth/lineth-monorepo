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
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j

/**
 * Tests that verify blob transactions are correctly rejected from the transaction pool at the
 * RPC/P2P level (via LineaTransactionPoolValidatorPlugin's TransactionTypeValidator).
 */
class BlobTransactionDenialTest : LineaPluginPoSTestBase() {
  private lateinit var web3j: Web3j
  private lateinit var credentials: Credentials
  private lateinit var recipient: String

  @BeforeEach
  override fun setup() {
    super.setup()
    web3j = minerNode.nodeRequests().eth()
    credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    recipient = accounts.secondaryBenefactor.address
  }

  @Test
  fun blobTransactionsIsRejectedFromTransactionPool() {
    // Act - Send a blob transaction to transaction pool
    val response = sendRawBlobTransaction(web3j, credentials, recipient)
    // No need to build new block.

    // Assert
    assertThat(response.hasError()).isTrue()
    assertThat(response.error.message)
      .contains(TransactionTypeValidationErrors.BLOB_TX_NOT_ALLOWED)
  }
}
