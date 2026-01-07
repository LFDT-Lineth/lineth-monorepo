/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package linea.plugin.acc.test

import com.fasterxml.jackson.core.JsonProcessingException
import com.fasterxml.jackson.databind.JsonNode
import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.node.ArrayNode
import com.fasterxml.jackson.databind.node.ObjectNode
import com.google.common.io.Resources
import okhttp3.Response
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.hyperledger.besu.consensus.clique.CliqueExtraData
import org.hyperledger.besu.crypto.SECP256K1
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.BlobGas
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.datatypes.RequestType
import org.hyperledger.besu.datatypes.TransactionType
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.parameters.EnginePayloadParameter
import org.hyperledger.besu.ethereum.core.BlockHeader
import org.hyperledger.besu.ethereum.core.Difficulty
import org.hyperledger.besu.ethereum.core.Request
import org.hyperledger.besu.ethereum.core.Transaction
import org.hyperledger.besu.ethereum.eth.transactions.ImmutableTransactionPoolConfiguration
import org.hyperledger.besu.ethereum.eth.transactions.TransactionPoolConfiguration
import org.hyperledger.besu.ethereum.mainnet.BodyValidation
import org.hyperledger.besu.ethereum.mainnet.MainnetBlockHeaderFunctions
import org.hyperledger.besu.tests.acceptance.dsl.EngineAPIService
import org.hyperledger.besu.tests.acceptance.dsl.node.RunnableNode
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory.CliqueOptions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.web3j.crypto.Blob
import org.web3j.crypto.BlobUtils
import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameterName
import org.web3j.protocol.core.methods.response.EthBlock
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.tx.gas.DefaultGasProvider
import org.web3j.utils.Numeric
import java.io.File
import java.math.BigInteger
import java.nio.charset.StandardCharsets
import java.time.Instant
import java.util.Optional
import java.util.concurrent.Executors
import java.util.concurrent.ScheduledExecutorService
import java.util.concurrent.TimeUnit
import java.util.function.Supplier

/**
 * Record containing all parameters required for engine_newPayloadV4 API calls.
 *
 * @param executionPayload ExecutionPayloadV3-compatible block data
 * @param expectedBlobVersionedHashes Array of 32-byte blob versioned hashes for validation
 * @param parentBeaconBlockRoot 32-byte root of the parent beacon block
 * @param executionRequests Array of execution layer triggered requests per EIP-7685
 */
data class EngineNewPayloadRequest(
  val executionPayload: ObjectNode,
  val expectedBlobVersionedHashes: ArrayNode,
  val parentBeaconBlockRoot: String,
  val executionRequests: ArrayNode,
)

/**
 * This file initializes a Besu node configured for the Prague fork and makes it available to
 * acceptance tests.
 */
abstract class LineaPluginPoSTestBase : LineaPluginTestBase() {

  private lateinit var engineApiService: EngineAPIService
  protected lateinit var mapper: ObjectMapper

  private val consensusScheduler: ScheduledExecutorService = Executors.newSingleThreadScheduledExecutor()
  protected var blockTimeSeconds: Long? = null
  protected var buildBlocksInBackground: Boolean = true

  companion object {
    private val GAS_PRICE: BigInteger = DefaultGasProvider.GAS_PRICE
    private val GAS_LIMIT: BigInteger = DefaultGasProvider.GAS_LIMIT
    private val VALUE: BigInteger = BigInteger.ZERO
    private const val DATA = "0x"
    private val secp256k1 = SECP256K1()
  }

  // Override this in subclasses to use a different genesis file template
  protected open fun getGenesisFileTemplatePath(): String {
    return "/clique/clique-to-pos.json.tpl"
  }

  @BeforeEach
  override fun setup() {
    minerNode = createCliqueNodeWithExtraCliOptionsAndRpcApis(
      "miner1",
      getCliqueOptions(),
      getTestCliOptions(),
      setOf("LINEA", "MINER", "PLUGINS"),
      true,
      DEFAULT_REQUESTED_PLUGINS,
    )
    minerNode.transactionPoolConfiguration = ImmutableTransactionPoolConfiguration.builder()
      .from(TransactionPoolConfiguration.DEFAULT)
      .noLocalPriority(true)
      .build()
    cluster.start(minerNode)
    mapper = ObjectMapper()
    engineApiService = EngineAPIService(minerNode, ethTransactions, mapper)
    blockTimeSeconds = getDefaultSlotTimeSeconds()
    buildNewBlocksInBackground()
  }

  @AfterEach
  override fun stop() {
    try {
      consensusScheduler.shutdownNow()
    } finally {
      super.stop()
    }
  }

  // Ideally GenesisConfigurationFactory.createCliqueGenesisConfig would support a custom genesis
  // file path. We have resorted to inlining its logic here to allow a flexible genesis file path.
  override fun provideGenesisConfig(
    validators: Collection<RunnableNode>,
    cliqueOptions: CliqueOptions,
  ): String {
    // Target state
    val genesisTemplate = GenesisConfigurationFactory.readGenesisFile(getGenesisFileTemplatePath())
    val hydratedGenesisTemplate = genesisTemplate
      .replace("%blockperiodseconds%", cliqueOptions.blockPeriodSeconds().toString())
      .replace("%epochlength%", cliqueOptions.epochLength().toString())
      .replace("%createemptyblocks%", cliqueOptions.createEmptyBlocks().toString())

    val addresses = validators.map { it.address }
    val extraDataString = CliqueExtraData.createGenesisExtraDataString(addresses)
    val genesis = hydratedGenesisTemplate.replace("%extraData%", extraDataString)

    return maybeCustomGenesisExtraData()
      .map { ed -> setGenesisCustomExtraData(genesis, ed) }
      .orElse(genesis)
  }

  protected fun buildNewBlocksInBackground() {
    consensusScheduler.scheduleAtFixedRate(
      {
        try {
          if (buildBlocksInBackground) {
            buildNewBlock(
              Instant.now().epochSecond,
              blockTimeSeconds!! * 1000 - 200,
            ) { !buildBlocksInBackground }
          }
        } catch (e: Exception) {
          throw RuntimeException(e)
        }
      },
      0,
      blockTimeSeconds!!,
      TimeUnit.SECONDS,
    )
  }

  // No-arg override for simple test cases, we take sensible defaults from the genesis config
  protected fun buildNewBlock() {
    val latestTimestamp = minerNode.execute(ethTransactions.block()).timestamp
    engineApiService.buildNewBlock(
      latestTimestamp.toLong() + blockTimeSeconds!!,
      blockTimeSeconds!! * 1000,
    )
  }

  private fun getDefaultSlotTimeSeconds(): Long {
    val genesisConfigSerialized = minerNode.genesisConfig.get()
    val genesisConfig: JsonNode = try {
      mapper.readTree(genesisConfigSerialized)
    } catch (e: JsonProcessingException) {
      throw RuntimeException(e)
    }
    return genesisConfig.path("config").path("clique").path("blockperiodseconds").asLong()
  }

  /**
   * @param blockTimestampSeconds The Unix timestamp (in seconds) to assign to the new block.
   * @param blockBuildingTimeMs The duration (in milliseconds) allocated for the Besu node to build the block.
   * @param stopBlockProduction Supplier that returns true when block production should stop
   */
  protected fun buildNewBlock(
    blockTimestampSeconds: Long,
    blockBuildingTimeMs: Long,
    stopBlockProduction: Supplier<Boolean>,
  ) {
    engineApiService.buildNewBlock(blockTimestampSeconds, blockBuildingTimeMs, stopBlockProduction)
  }

  protected fun buildNewBlockAndWait() {
    val initialBlockNumber = getLatestBlockNumber()
    buildNewBlock()
    await()
      .atMost(3 * blockTimeSeconds!!, TimeUnit.SECONDS)
      .pollInterval(500, TimeUnit.MILLISECONDS)
      .untilAsserted { assertThat(getLatestBlockNumber()).isGreaterThan(initialBlockNumber) }
  }

  /**
   * Creates and sends a blob transaction. This method is designed to be stateless and should not
   * rely on any class properties or instance methods. All required data should be passed as
   * parameters. This makes it easier to test and reuse in different contexts.
   */
  protected fun sendRawBlobTransaction(
    web3j: Web3j,
    credentials: Credentials,
    recipient: String,
  ): EthSendTransaction {
    val nonce = web3j
      .ethGetTransactionCount(credentials.address, DefaultBlockParameterName.PENDING)
      .send()
      .transactionCount

    // Take blob file from public reference so we can sanity check values -
    // https://github.com/LFDT-web3j/web3j/blob/9dbd2f90468538408eeb9a1e87e8e73a9f3dda3b/crypto/src/test/java/org/web3j/crypto/BlobUtilsTest.java#L63-L83
    val blobUrl = File(getResourcePath("/blob.txt")).toURI().toURL()
    val blobHexString = Resources.toString(blobUrl, StandardCharsets.UTF_8)
    val blob = Blob(Numeric.hexStringToByteArray(blobHexString))
    val kzgCommitment = BlobUtils.getCommitment(blob)
    val kzgProof = BlobUtils.getProof(blob, kzgCommitment)
    val versionedHash = BlobUtils.kzgToVersionedHash(kzgCommitment)
    val rawTransaction = RawTransaction.createTransaction(
      listOf(blob),
      listOf(kzgCommitment),
      listOf(kzgProof),
      CHAIN_ID,
      nonce,
      GAS_PRICE,
      GAS_PRICE,
      GAS_LIMIT,
      recipient,
      VALUE,
      DATA,
      BigInteger.ONE,
      listOf(versionedHash),
    )
    val signedMessage = TransactionEncoder.signMessage(rawTransaction, credentials)
    val hexValue = Numeric.toHexString(signedMessage)
    return web3j.ethSendRawTransaction(hexValue).send()
  }

  /**
   * Creates and sends EIP7702 delegate code transaction. This method is designed to be stateless
   * and should not rely on any class properties or instance methods. All required data should be
   * passed as parameters. This makes it easier to test and reuse in different contexts.
   */
  protected fun sendRawEIP7702Transaction(
    web3j: Web3j,
    credentials: Credentials,
    recipient: String,
  ): String {
    val nonce = web3j
      .ethGetTransactionCount(credentials.address, DefaultBlockParameterName.PENDING)
      .send()
      .transactionCount

    // 7702 transaction
    val codeDelegation = org.hyperledger.besu.ethereum.core.CodeDelegation.builder()
      .chainId(BigInteger.valueOf(CHAIN_ID))
      .address(Address.fromHexStringStrict(recipient))
      .nonce(1)
      .signAndBuild(
        secp256k1.createKeyPair(
          secp256k1.createPrivateKey(credentials.ecKeyPair.privateKey),
        ),
      )

    val tx = Transaction.builder()
      .type(TransactionType.DELEGATE_CODE)
      .chainId(BigInteger.valueOf(CHAIN_ID))
      .nonce(nonce.toLong())
      .maxPriorityFeePerGas(Wei.of(GAS_PRICE))
      .maxFeePerGas(Wei.of(GAS_PRICE))
      .gasLimit(GAS_LIMIT.toLong())
      .to(Address.fromHexStringStrict(credentials.address))
      .value(Wei.ZERO)
      .payload(Bytes.EMPTY)
      .accessList(emptyList())
      .codeDelegations(listOf(codeDelegation))
      .signAndBuild(
        secp256k1.createKeyPair(
          secp256k1.createPrivateKey(credentials.ecKeyPair.privateKey),
        ),
      )

    return minerNode.execute(ethTransactions.sendRawTransaction(tx.encoded().toHexString()))
  }

  /**
   * Imports a premade block using the Engine API to the test node.
   *
   * @param executionPayload Complete execution payload with block data
   * @param expectedBlobVersionedHashes Array of expected blob hashes
   * @param parentBeaconBlockRoot Root hash of the parent beacon block
   * @param executionRequests Array of execution layer requests
   * @return HTTP response from the Engine API call containing validation results
   */
  protected fun importPremadeBlock(
    executionPayload: ObjectNode,
    expectedBlobVersionedHashes: ArrayNode,
    parentBeaconBlockRoot: String,
    executionRequests: ArrayNode,
  ): Response {
    return engineApiService.importPremadeBlock(
      executionPayload,
      expectedBlobVersionedHashes,
      parentBeaconBlockRoot,
      executionRequests,
    )
  }

  protected fun importPremadeBlock(
    executionPayloadRequest: EngineNewPayloadRequest,
  ): Response {
    return engineApiService.importPremadeBlock(
      executionPayloadRequest.executionPayload,
      executionPayloadRequest.expectedBlobVersionedHashes,
      executionPayloadRequest.parentBeaconBlockRoot,
      executionPayloadRequest.executionRequests,
    )
  }

  protected fun getLatestBlock(): EthBlock.Block {
    return minerNode
      .nodeRequests()
      .eth()
      .ethGetBlockByNumber(DefaultBlockParameterName.LATEST, false)
      .send()
      .block
  }

  /**
   * Retrieves the hash of the latest block from the test node.
   *
   * @return The hexadecimal hash string of the latest block
   */
  protected fun getLatestBlockHash(): String {
    return getLatestBlock().hash
  }

  protected fun getLatestBlockNumber(): Long {
    return minerNode.nodeRequests().eth().ethBlockNumber().send().blockNumber.toLong()
  }

  /**
   * Creates an execution payload for block import testing.
   *
   * Constructs an ExecutionPayloadV3-compatible JSON object that can be used with the
   * engine_newPayloadV4 method as defined in the Prague fork specification. The payload includes
   * all required block header fields and an optional transaction.
   *
   * @param mapper JSON object mapper for creating Jackson nodes
   * @param genesisBlockHash Hash of the genesis/parent block to reference
   * @param blockParams Map containing all block parameters using [BlockParams] constants as keys
   * @param transactionKey Key in blockParams map containing transaction data, or empty string for no transactions
   * @return ObjectNode representing the execution payload compatible with engine_newPayloadV4
   * @see [Prague Engine API Specification](https://github.com/ethereum/execution-apis/blob/main/src/engine/prague.md)
   */
  protected fun createExecutionPayload(
    mapper: ObjectMapper,
    genesisBlockHash: String,
    blockParams: Map<String, String>,
    transactionKey: String,
  ): ObjectNode {
    val payload = mapper.createObjectNode()
      .put("parentHash", genesisBlockHash)
      .put("feeRecipient", blockParams[BlockParams.FEE_RECIPIENT])
      .put("stateRoot", blockParams[BlockParams.STATE_ROOT])
      .put("logsBloom", blockParams[BlockParams.LOGS_BLOOM])
      .put("prevRandao", blockParams[BlockParams.PREV_RANDAO])
      .put("gasLimit", blockParams[BlockParams.GAS_LIMIT])
      .put("gasUsed", blockParams[BlockParams.GAS_USED])
      .put("timestamp", blockParams[BlockParams.TIMESTAMP])
      .put("extraData", blockParams[BlockParams.EXTRA_DATA])
      .put("baseFeePerGas", blockParams[BlockParams.BASE_FEE_PER_GAS])
      .put("excessBlobGas", blockParams[BlockParams.EXCESS_BLOB_GAS])
      .put("blobGasUsed", blockParams[BlockParams.BLOB_GAS_USED])
      .put("receiptsRoot", blockParams[BlockParams.RECEIPTS_ROOT])
      .put("blockNumber", blockParams[BlockParams.BLOCK_NUMBER])

    // Add transactions
    val transactions = mapper.createArrayNode()
    if (blockParams.containsKey(transactionKey)) {
      transactions.add(blockParams[transactionKey])
    }
    payload.set<ArrayNode>("transactions", transactions)

    // Add withdrawals (empty list)
    val withdrawals = mapper.createArrayNode()
    payload.set<ArrayNode>("withdrawals", withdrawals)

    return payload
  }

  /**
   * Creates blob versioned hashes array from block parameters.
   *
   * Extracts blob versioned hashes from block parameters for transactions that include blob
   * data. Each hash is 32 bytes and used to validate blob data integrity.
   *
   * @param mapper JSON object mapper for creating Jackson nodes
   * @param blockParams Map containing block parameters with blob hash data
   * @param versionedHashKey Key in blockParams for accessing the blob versioned hash
   * @return ArrayNode containing the blob versioned hash (32 bytes)
   */
  protected fun createBlobVersionedHashes(
    mapper: ObjectMapper,
    blockParams: Map<String, String>,
    versionedHashKey: String,
  ): ArrayNode {
    val hashes = mapper.createArrayNode()
    if (blockParams.containsKey(versionedHashKey)) {
      hashes.add(blockParams[versionedHashKey])
    }
    return hashes
  }

  /**
   * Creates an empty versioned hashes array for non-blob transactions.
   *
   * @param mapper JSON object mapper for creating Jackson nodes
   * @return Empty ArrayNode for blocks without blob transactions
   */
  protected fun createEmptyVersionedHashes(mapper: ObjectMapper): ArrayNode {
    return mapper.createArrayNode()
  }

  /**
   * Creates EIP-7685 execution requests array for Engine API block import.
   *
   * @param mapper JSON object mapper for creating Jackson nodes
   * @param blockParams Map containing block parameters with execution request data
   * @return ArrayNode containing execution requests as hex-encoded byte arrays
   */
  protected fun createExecutionRequests(
    mapper: ObjectMapper,
    blockParams: Map<String, String>,
  ): ArrayNode {
    val requests = mapper.createArrayNode()
    requests.add(blockParams[BlockParams.EXECUTION_REQUEST])
    return requests
  }

  /**
   * Computes a complete block header from execution payload and block parameters.
   *
   * Creates a Besu BlockHeader instance that includes all required fields for Prague fork
   * including execution requests commitment. The computed header is used to generate the correct
   * blockHash for Engine API validation.
   *
   * @param executionPayload JSON execution payload created by [createExecutionPayload]
   * @param mapper JSON object mapper for parsing the payload
   * @param blockParams Map containing all block parameters and roots
   * @return Complete BlockHeader instance with computed hash
   */
  protected fun computeBlockHeader(
    executionPayload: ObjectNode,
    mapper: ObjectMapper,
    blockParams: Map<String, String>,
  ): BlockHeader {
    val blockParam = mapper.readValue(executionPayload.toString(), EnginePayloadParameter::class.java)

    val transactionsRoot = Hash.fromHexString(blockParams[BlockParams.TRANSACTIONS_ROOT])
    val withdrawalsRoot = Hash.fromHexString(blockParams[BlockParams.WITHDRAWALS_ROOT])

    // Take code from AbstractEngineNewPayload in Besu codebase
    val executionRequestBytes = Bytes.fromHexString(blockParams[BlockParams.EXECUTION_REQUEST]!!)
    val executionRequestBytesData = executionRequestBytes.slice(1)
    val executionRequest = Request(
      RequestType.of(executionRequestBytes[0].toInt()),
      executionRequestBytesData,
    )
    val maybeRequests = Optional.of(listOf(executionRequest))

    return BlockHeader(
      blockParam.parentHash,
      Hash.EMPTY_LIST_HASH, // OMMERS_HASH_CONSTANT
      blockParam.feeRecipient,
      blockParam.stateRoot,
      transactionsRoot,
      blockParam.receiptsRoot,
      blockParam.logsBloom,
      Difficulty.ZERO,
      blockParam.blockNumber,
      blockParam.gasLimit,
      blockParam.gasUsed,
      blockParam.timestamp,
      Bytes.fromHexString(blockParam.extraData),
      blockParam.baseFeePerGas,
      blockParam.prevRandao,
      0, // Nonce
      withdrawalsRoot,
      blockParam.blobGasUsed,
      BlobGas.fromHexString(blockParam.excessBlobGas),
      Bytes32.fromHexString(blockParams[BlockParams.PARENT_BEACON_BLOCK_ROOT]!!),
      maybeRequests.map { BodyValidation.requestsHash(it) }.orElse(null),
      null, // BAL hash
      MainnetBlockHeaderFunctions(),
    )
  }

  /**
   * Updates the execution payload with the computed block hash.
   *
   * @param executionPayload JSON execution payload to update
   * @param blockHeader Block header containing the computed hash
   */
  protected fun updateExecutionPayloadWithBlockHash(
    executionPayload: ObjectNode,
    blockHeader: BlockHeader,
  ) {
    executionPayload.put("blockHash", blockHeader.blockHash.toHexString())
  }

  /**
   * Asserts that a block import was rejected with the expected validation error.
   *
   * @param response HTTP response from the Engine API call
   * @param expectedValidationError Expected validation error message to check for
   */
  protected fun assertBlockImportRejected(response: Response, expectedValidationError: String) {
    val result = mapper.readTree(response.body?.string()).get("result")
    val status = result.get("status").asText()
    val validationError = result.get("validationError").asText()
    assertThat(status).isEqualTo("INVALID")
    assertThat(validationError).contains(expectedValidationError)
  }

  // Constants for transaction-related data keys
  object TransactionDataKeys {
    const val BLOB_TX = "BLOB_TX"
    const val DELEGATE_CALL_TX = "DELEGATE_CALL_TX"
    const val BLOB_VERSIONED_HASH = "BLOB_VERSIONED_HASH"
  }

  // Constants for block parameter keys
  object BlockParams {
    const val STATE_ROOT = "STATE_ROOT"
    const val LOGS_BLOOM = "LOGS_BLOOM"
    const val RECEIPTS_ROOT = "RECEIPTS_ROOT"
    const val EXTRA_DATA = "EXTRA_DATA"
    const val EXECUTION_REQUEST = "EXECUTION_REQUEST"
    const val TRANSACTIONS_ROOT = "TRANSACTIONS_ROOT"
    const val WITHDRAWALS_ROOT = "WITHDRAWALS_ROOT"
    const val GAS_LIMIT = "GAS_LIMIT"
    const val GAS_USED = "GAS_USED"
    const val TIMESTAMP = "TIMESTAMP"
    const val BASE_FEE_PER_GAS = "BASE_FEE_PER_GAS"
    const val EXCESS_BLOB_GAS = "EXCESS_BLOB_GAS"
    const val BLOB_GAS_USED = "BLOB_GAS_USED"
    const val BLOCK_NUMBER = "BLOCK_NUMBER"
    const val FEE_RECIPIENT = "FEE_RECIPIENT"
    const val PREV_RANDAO = "PREV_RANDAO"
    const val PARENT_BEACON_BLOCK_ROOT = "PARENT_BEACON_BLOCK_ROOT"
  }

  // Constants for validation error messages
  object LineaTransactionValidatorPluginErrors {
    const val BLOB_TX_NOT_ALLOWED = "LineaTransactionValidatorPlugin - BLOB_TX_NOT_ALLOWED"
    const val DELEGATE_CODE_TX_NOT_ALLOWED = "LineaTransactionValidatorPlugin - DELEGATE_CODE_TX_NOT_ALLOWED"
  }
}
