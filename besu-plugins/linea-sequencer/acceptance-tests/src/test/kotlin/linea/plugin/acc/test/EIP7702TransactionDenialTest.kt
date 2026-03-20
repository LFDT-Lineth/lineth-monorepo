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
import org.junit.jupiter.api.Assertions.assertThrows
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j

/**
 * Tests that verify the LineaTransactionValidationPlugin correctly rejects EIP7702 DELEGATE_CODE
 * transactions from being executed when explicitly disabled via CLI option
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
    assertThat(exception.message).contains("Plugin has marked the transaction as invalid")
  }

  // Ideally the block import test would be conducted with two nodes as follows:
  // 1. Start an additional minimal node with Prague config
  // 2. Ensure additional node is peered to minerNode
  // 3. Send EIP7702 tx to additional node
  // 4. Construct block on additional node
  // 5. Send 'debug_getBadBlocks' RPC request to minerNode, confirm that block is rejected from
  // import
  //
  // However we are currently unable to run more than one node per test, due to the CLI options
  // being
  // singleton options and this implemented in dependency repository - linea tracer.
  // Thus simulate the block import as below:
  // 1. Create a premade block containing a EIP7702 tx
  // 2. Import the premade block using 'engine_newPayloadV4' Engine API call

  @Test
  fun EIP7702TransactionIsRejectedFromNodeImport() {
    // Arrange
    val blockWithEIP7702TxRequest = createEIP7702TransactionBlockRequest(mapper)

    // Act
    val response = this.importPremadeBlock(blockWithEIP7702TxRequest)

    // Assert
    assertBlockImportRejected(
      response,
      LineaTransactionValidatorPluginErrors.DELEGATE_CODE_TX_NOT_ALLOWED,
    )
  }

  private fun createEIP7702TransactionBlockRequest(mapper: ObjectMapper): EngineNewPayloadRequest {
    // Block parameters obtained by running `EIP7702TransactionIsRejectedFromNodeImport` test
    // without the LineaTransactionSelectorPlugin and LineaTransactionValidatorPlugin plugins.
    val blockWithBlockTxParams = mapOf(
      BlockParams.STATE_ROOT to "0x217cc246352b4a22254ab139cc4a5a37e1dbe75b63fcf12161674773f5043bbe",
      BlockParams.LOGS_BLOOM to "0x0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000" +
        "0000000000000000000000000000000000000000000000000000000000000000",
      BlockParams.RECEIPTS_ROOT to
        "0x036c7d20420edbce24b5062148bce80563b48fd4532fb1c068b2c96a53117019",
      BlockParams.EXTRA_DATA to
        "0x626573752032352e372e302d6c696e656134",
      TransactionDataKeys.DELEGATE_CALL_TX to
        "0x04f8cd8205398084f461090084f46109008389544094fe3b557e8fb62b89f491" +
        "6b721be55ceb828dbd738080c0f85ef85c82053994627306090abab3a6e1400e" +
        "9345bc60c78a8bef570101a0972498bc9ef3b18ec9f16f3dd59e7b622cc07fce" +
        "1459d7485f424658e4013aa6a038dfeeaaa952cf3eb3fb81ccac2b57a69f57b4" +
        "57d1350353ac80d11ddc5dfeb180a0713d685d1b0fd47e7e7e75d9d8aaf2fa0d" +
        "4a8811aec37fa54cb0ca4deb632dcaa01a66a49c9bbd92f11a0e67fb096a6e86" +
        "2a5da8bf05af32f6390ee431011fba81",
      BlockParams.EXECUTION_REQUEST to
        "0x01a4664c40aacebd82a2db79f0ea36c06bc6a19adbb10a4a15bf67b328c9b101" +
        "d09e5c6ee6672978fdad9ef0d9e2ceffaee99223555d8601f0cb3bcc4ce1af98" +
        "64779a416e0000000000000000",
      BlockParams.TRANSACTIONS_ROOT to "0xb84030d9aae336c44f284a9710bc8f6771a38d0bccdeb1d837f871bacd1d07c9",
      BlockParams.WITHDRAWALS_ROOT to "0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
      BlockParams.GAS_LIMIT to "0x1ca35ef",
      BlockParams.GAS_USED to "0x8fc0",
      BlockParams.TIMESTAMP to "0x5",
      BlockParams.BASE_FEE_PER_GAS to "0x7",
      BlockParams.EXCESS_BLOB_GAS to "0x0",
      BlockParams.BLOB_GAS_USED to "0x0",
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
      TransactionDataKeys.DELEGATE_CALL_TX,
    )
    val expectedBlobVersionedHashes = createEmptyVersionedHashes(mapper)
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
