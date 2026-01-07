/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import com.fasterxml.jackson.databind.ObjectMapper
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j

/**
 * Tests that verify the LineaTransactionValidationPlugin correctly rejects BLOB transactions from
 * being executed
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
      .contains("Plugin has marked the transaction as invalid")
  }

  // Ideally the block import test would be conducted with two nodes as follows:
  // 1. Start an additional minimal node with Prague config
  // 2. Ensure additional node is peered to minerNode
  // 3. Send blob tx to additional node
  // 4. Construct block on additional node
  // 5. Send 'debug_getBadBlocks' RPC request to minerNode, confirm that block is rejected from
  // import
  //
  // However we are currently unable to run more than one node per test, due to the CLI options
  // being
  // singleton options and this implemented in dependency repository - linea tracer.
  // Thus simulate the block import as below:
  // 1. Create a premade block containing a blob tx
  // 2. Import the premade block using 'engine_newPayloadV4' Engine API call

  @Test
  fun blobTransactionsIsRejectedFromNodeImport() {
    // Arrange
    val blockWithBlobTxRequest = createBlobTransactionBlockRequest(mapper)

    // Act
    val response = this.importPremadeBlock(blockWithBlobTxRequest)

    // Assert
    assertBlockImportRejected(response, LineaTransactionValidatorPluginErrors.BLOB_TX_NOT_ALLOWED)
  }

  private fun createBlobTransactionBlockRequest(mapper: ObjectMapper): EngineNewPayloadRequest {
    // Block parameters obtained by running `blobTransactionsIsRejectedFromTransactionPool` test
    // without the LineaTransactionSelectorPlugin and LineaTransactionValidatorPlugin plugins.
    val blockWithBlockTxParams = mapOf(
      BlockParams.STATE_ROOT to
        "0x2c1457760c057cf42f2d509648d725ec1f557b9d8729a5361e517952f91d050e",
      BlockParams.LOGS_BLOOM to
        "0x0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000",
      BlockParams.RECEIPTS_ROOT to
        "0xeaa8c40899a61ae59615cf9985f5e2194f8fd2b57d273be63bde6733e89b12ab",
      BlockParams.EXTRA_DATA to
        "0x626573752032352e362e302d6c696e656131",
      TransactionDataKeys.BLOB_TX to
        "0x03f8908205398084f461090084f46109008389544094627306090abab3a6e140" +
        "0e9345bc60c78a8bef578080c001e1a0018ef96865998238a5e1783b6cafbc12" +
        "53235d636f15d318f1fb50ef6a5b8f6a80a0576a95756f32ab705a22b591ab46" +
        "4d5affc8c1c7fcd14d777bac24d83bc44821a01f93b26f4f9989c3fe764f4a58" +
        "d264bcd71b9deab72d6852f5dcdf19d55494f1",
      TransactionDataKeys.BLOB_VERSIONED_HASH to
        "0x018ef96865998238a5e1783b6cafbc1253235d636f15d318f1fb50ef6a5b8f6a",
      BlockParams.EXECUTION_REQUEST to
        "0x01a4664c40aacebd82a2db79f0ea36c06bc6a19adbb10a4a15bf67b328c9b101" +
        "d09e5c6ee6672978fdad9ef0d9e2ceffaee99223555d8601f0cb3bcc4ce1af98" +
        "64779a416e0000000000000000",
      BlockParams.TRANSACTIONS_ROOT to
        "0x7a430a1c9da1f6e25ff8e6e96217c359784f3438dc1d983b4695355d66437f8f",
      BlockParams.WITHDRAWALS_ROOT to
        "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
      BlockParams.GAS_LIMIT to "0x1ca35ef",
      BlockParams.GAS_USED to "0x5208",
      BlockParams.TIMESTAMP to "0x5",
      BlockParams.BASE_FEE_PER_GAS to "0x7",
      BlockParams.EXCESS_BLOB_GAS to "0x0",
      BlockParams.BLOB_GAS_USED to "0x20000",
      BlockParams.BLOCK_NUMBER to "0x1",
      BlockParams.FEE_RECIPIENT to Address.ZERO.toHexString(),
      BlockParams.PREV_RANDAO to Hash.ZERO.toHexString(),
      BlockParams.PARENT_BEACON_BLOCK_ROOT to Hash.ZERO.toHexString(),
    )
    // Seems that the genesis block hash change with each run, despite a constant genesis file
    val genesisBlockHash = getLatestBlockHash()

    val executionPayload = createExecutionPayload(
      mapper,
      genesisBlockHash,
      blockWithBlockTxParams,
      TransactionDataKeys.BLOB_TX,
    )
    val expectedBlobVersionedHashes = createBlobVersionedHashes(
      mapper,
      blockWithBlockTxParams,
      TransactionDataKeys.BLOB_VERSIONED_HASH,
    )
    val executionRequests = createExecutionRequests(mapper, blockWithBlockTxParams)
    // Compute block hash and update payload
    val blockHeader = computeBlockHeader(executionPayload, mapper, blockWithBlockTxParams)
    updateExecutionPayloadWithBlockHash(executionPayload, blockHeader)
    return EngineNewPayloadRequest(
      executionPayload,
      expectedBlobVersionedHashes,
      blockWithBlockTxParams[BlockParams.PARENT_BEACON_BLOCK_ROOT]!!,
      executionRequests,
    )
  }
}
