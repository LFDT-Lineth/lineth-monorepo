package linea.ethapi

import linea.domain.Block
import linea.domain.BlockParameter
import linea.domain.BlockWithTxHashes
import linea.domain.EthLog
import linea.domain.FeeHistory
import linea.domain.Transaction
import linea.domain.TransactionForEthCall
import linea.domain.TransactionReceipt
import linea.domain.createBlock
import linea.domain.toBlockWithRandomTxHashes
import linea.kotlin.decodeHex
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import kotlin.random.Random
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class FakeEthApiClient(
  initialLogsDb: Set<EthLog> = emptySet(),
  val chainId: ULong = 101UL,
  val genesisTimestamp: Instant = Instant.parse("2025-04-01T00:00:00Z"),
  val blockTime: Duration = 1.seconds,
  initialTagsBlocks: Map<BlockParameter.Tag, ULong> = mapOf(
    BlockParameter.Tag.EARLIEST to 0UL,
    BlockParameter.Tag.LATEST to 0UL,
    BlockParameter.Tag.SAFE to 0UL,
    BlockParameter.Tag.FINALIZED to 0UL,
    BlockParameter.Tag.PENDING to 0UL,
  ),
  private val topicsTranslation: Map<String, String> = emptyMap(),
  private val log: Logger = LogManager.getLogger(FakeEthApiClient::class.java),
) : EthApiClient {
  private val blockTags: MutableMap<BlockParameter.Tag, ULong> = initialTagsBlocks.toMutableMap()
  private val logsDb: MutableList<EthLog> = mutableListOf()
  private val blocksDb: MutableMap<ULong, Block> = mutableMapOf()

  // key = which block number to throw error
  // value = how many times left to throw error, null means always throw
  private var getLogsBlocksForcedErrorsCounts: MutableMap<ULong, ULong?> = mutableMapOf()

  init {
    require(initialTagsBlocks.keys.size == BlockParameter.Tag.entries.size) {
      "Please specify all block tags: ${BlockParameter.Tag.entries.joinToString(", ")}"
    }
    setLogs(initialLogsDb)
  }

  @Synchronized
  fun addBlocks(blocks: List<Block>) {
    blocks.forEach { block -> blocksDb[block.number] = block }
  }

  @Synchronized
  fun addLogs(logs: Set<EthLog>) = setLogs(logsDb.toSet() + logs)

  @Synchronized
  fun setLogs(logs: List<EthLog>) {
    val set = logs.toSet()
    require(set.size == logs.size) { "logs have duplicates" }
    setLogs(set)
  }

  @Synchronized
  fun setLogs(logs: Set<EthLog>) {
    logsDb.clear()
    logsDb.addAll(logs.sortedBy { it.blockNumber })
  }

  @Synchronized
  fun setLatestBlockTag(blockNumber: ULong) {
    blockTags[BlockParameter.Tag.LATEST] = blockNumber
    coerceTagsAtMostTo(
      listOf(BlockParameter.Tag.FINALIZED, BlockParameter.Tag.SAFE, BlockParameter.Tag.PENDING),
      blockNumber,
    )
  }

  @Synchronized
  fun setGetLogsBlocksForcedErrorsCounts(getLogsBlocksForcedErrorsCounts: MutableMap<ULong, ULong?>) {
    this.getLogsBlocksForcedErrorsCounts = getLogsBlocksForcedErrorsCounts
  }

  @Synchronized
  fun setSafeBlockTag(blockNumber: ULong) {
    blockTags[BlockParameter.Tag.SAFE] = blockNumber

    coerceTagsAtLeastTo(
      listOf(BlockParameter.Tag.LATEST, BlockParameter.Tag.PENDING),
      blockNumber,
    )
    coerceTagsAtMostTo(
      listOf(BlockParameter.Tag.FINALIZED),
      blockNumber,
    )
  }

  @Synchronized
  fun setFinalizedBlockTag(blockNumber: ULong) {
    blockTags[BlockParameter.Tag.FINALIZED] = blockNumber

    coerceTagsAtLeastTo(
      listOf(BlockParameter.Tag.SAFE, BlockParameter.Tag.LATEST, BlockParameter.Tag.PENDING),
      blockNumber,
    )
  }

  private fun coerceTagsAtMostTo(tags: List<BlockParameter.Tag>, maxBlockNumber: ULong) {
    tags.forEach { tag ->
      val blockNumber = blockTags[tag] ?: throw IllegalStateException("Block tag $tag doesn't exist")
      if (blockNumber > maxBlockNumber) {
        blockTags[tag] = maxBlockNumber
      }
    }
  }

  private fun coerceTagsAtLeastTo(tags: List<BlockParameter.Tag>, minBlockNumber: ULong) {
    tags.forEach { tag ->
      val blockNumber = blockTags[tag] ?: throw IllegalStateException("Block tag $tag doesn't exist")
      if (blockNumber < minBlockNumber) {
        blockTags[tag] = minBlockNumber
      }
    }
  }

  override fun ethChainId(): SafeFuture<ULong> {
    return SafeFuture.completedFuture(chainId)
  }

  override fun ethBlockNumber(): SafeFuture<ULong> {
    return SafeFuture.completedFuture(blockTags[BlockParameter.Tag.LATEST]!!)
  }

  override fun ethProtocolVersion(): SafeFuture<Int> = SafeFuture.completedFuture(67)

  override fun ethCoinbase(): SafeFuture<ByteArray> {
    TODO("Not yet implemented")
  }

  override fun ethMining(): SafeFuture<Boolean> {
    TODO("Not yet implemented")
  }

  override fun ethGasPrice(): SafeFuture<BigInteger> {
    TODO("Not yet implemented")
  }

  override fun ethMaxPriorityFeePerGas(): SafeFuture<BigInteger> {
    TODO("Not yet implemented")
  }

  override fun ethFeeHistory(
    blockCount: Int,
    newestBlock: BlockParameter,
    rewardPercentiles: List<Double>,
  ): SafeFuture<FeeHistory> {
    TODO("Not yet implemented")
  }

  override fun ethGetBalance(address: ByteArray, blockParameter: BlockParameter): SafeFuture<BigInteger> {
    TODO("Not yet implemented")
  }

  override fun ethGetTransactionCount(address: ByteArray, blockParameter: BlockParameter): SafeFuture<ULong> {
    TODO("Not yet implemented")
  }

  override fun ethFindBlockByNumberFullTxs(blockParameter: BlockParameter): SafeFuture<Block?> {
    val blockNumber = blockParameterToBlockNumber(blockParameter)
    if (isAfterHead(blockNumber)) {
      return SafeFuture.completedFuture(null)
    }

    val block = blocksDb[blockNumber] ?: generateFakeBlock(blockNumber)

    blocksDb[blockNumber] = block

    return SafeFuture.completedFuture(block)
  }

  override fun ethFindBlockByNumberTxHashes(blockParameter: BlockParameter): SafeFuture<BlockWithTxHashes?> {
    return this.ethFindBlockByNumberFullTxs(blockParameter).thenApply { block -> block?.toBlockWithRandomTxHashes() }
  }

  override fun ethGetTransactionByHash(transactionHash: ByteArray): SafeFuture<Transaction?> {
    TODO("Not yet implemented")
  }

  override fun ethGetTransactionReceipt(transactionHash: ByteArray): SafeFuture<TransactionReceipt?> {
    TODO("Not yet implemented")
  }

  override fun ethSendRawTransaction(signedTransactionData: ByteArray): SafeFuture<ByteArray> {
    TODO("Not yet implemented")
  }

  override fun ethCall(
    transaction: TransactionForEthCall,
    blockParameter: BlockParameter,
    stateOverride: StateOverride?,
  ): SafeFuture<ByteArray> {
    TODO("Not yet implemented")
  }

  override fun ethEstimateGas(transaction: TransactionForEthCall): SafeFuture<ULong> {
    TODO("Not yet implemented")
  }

  private fun isAfterHead(blockNumber: ULong): Boolean {
    return blockNumber > blockTags[BlockParameter.Tag.LATEST]!!
  }

  private fun generateFakeBlock(blockNumber: ULong): Block {
    val parentBlock = blocksDb[blockNumber - 1UL]
    val timestamp = genesisTimestamp + (blockTime * blockNumber.toInt())
    return createBlock(
      number = blockNumber,
      parentHash = parentBlock?.hash ?: Random.nextBytes(32),
      timestamp = timestamp,
    )
  }

  @Synchronized
  override fun getLogs(
    fromBlock: BlockParameter,
    toBlock: BlockParameter,
    address: String,
    topics: List<String?>,
  ): SafeFuture<List<EthLog>> {
    if (isAtBlockNumberToThrow(
        fromBlockNumber = blockParameterToBlockNumber(fromBlock),
        toBlockNumber = blockParameterToBlockNumber(toBlock),
      )
    ) {
      throw IllegalStateException("for error testing when calling getLogs")
    }

    val addressBytes = address.decodeHex()
    val topicsFilter = topics.map { it?.decodeHex() }
    val logsInBlockRange = findLogsInRange(fromBlock, toBlock)
    return logsInBlockRange
      .filter { log ->
        val addressMatch = log.address.contentEquals(addressBytes)
        val logFilterMatch = matchesTopicFilter(log.topics, topicsFilter)
        // this is meant for debugging purposes
        addressMatch && logFilterMatch
      }
      .let { logsMatching ->
        log.trace(
          "logDb: {}",
          logsDb.joinToString(prefix = "\n   ", separator = "\n   ") { log -> log.toString() },
        )
        log.debug(
          "getLogs: {}..{} address={} topics={} logsSize={} logs={}",
          fromBlock,
          toBlock,
          address,
          topics.joinToString(", ") { t -> t?.let { topicsTranslation[t] ?: t } ?: "null" },
          logsMatching.size,
          logsMatching.joinToString(prefix = "\n   ", separator = "\n   ") { log -> log.toString() },
        )
        SafeFuture.completedFuture(logsMatching)
      }
  }

  private fun findLogsInRange(fromBlock: BlockParameter, toBlock: BlockParameter): List<EthLog> {
    return logsDb.filter { isInRange(it.blockNumber, fromBlock, toBlock) }
  }

  private fun isAtBlockNumberToThrow(fromBlockNumber: ULong, toBlockNumber: ULong): Boolean {
    var shouldThrow = false
    val targetRange = fromBlockNumber..toBlockNumber
    getLogsBlocksForcedErrorsCounts.forEach { errorBlockNumber, throwTimes ->
      if (targetRange.contains(errorBlockNumber)) {
        if (throwTimes != null && throwTimes > 0UL) {
          getLogsBlocksForcedErrorsCounts[errorBlockNumber] = getLogsBlocksForcedErrorsCounts[errorBlockNumber]!! - 1UL
        }
        shouldThrow = shouldThrow || throwTimes == null || getLogsBlocksForcedErrorsCounts[errorBlockNumber]!! > 0UL
      }
    }
    return shouldThrow
  }

  private fun isInRange(blockNumber: ULong, fromBlock: BlockParameter, toBlock: BlockParameter): Boolean {
    val fromBlockNumber: ULong = blockParameterToBlockNumber(fromBlock)
    val toBlockNumber: ULong = blockParameterToBlockNumber(toBlock)

    return blockNumber in fromBlockNumber..toBlockNumber
  }

  private fun blockParameterToBlockNumber(blockParameter: BlockParameter): ULong {
    return when (blockParameter) {
      is BlockParameter.Tag -> blockTags[blockParameter]
        ?: throw IllegalArgumentException("Invalid blockParameter=$blockParameter")

      is BlockParameter.BlockNumber -> blockParameter.getNumber()
    }
  }

  companion object {
    fun matchesTopicFilter(logTopics: List<ByteArray>, topicsFilter: List<ByteArray?>): Boolean {
      if (topicsFilter.size > logTopics.size) return false

      return logTopics
        .zip(topicsFilter)
        .all { (logTopic, topicFilter) ->
          topicFilter
            ?.let { logTopic.contentEquals(topicFilter) }
            ?: true // if topic is null at this index, is not filtered by this topic, shall return true
        }
    }
  }
}
