package linea.staterecovery.plugin

import linea.kotlin.encodeHex
import linea.kotlin.toBigInteger
import linea.kotlin.toULong
import linea.staterecovery.BlockFromL1RecoveredData
import org.apache.logging.log4j.LogManager
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.datatypes.StateOverrideMap
import org.hyperledger.besu.plugin.data.BlockContext
import org.hyperledger.besu.plugin.data.BlockHeader
import org.hyperledger.besu.plugin.data.BlockOverrides
import org.hyperledger.besu.plugin.data.PluginBlockSimulationResult
import org.hyperledger.besu.plugin.services.BlockSimulationService
import org.hyperledger.besu.plugin.services.BlockchainService
import org.hyperledger.besu.plugin.services.sync.SynchronizationService

class BlockImporter(
  private val blockchainService: BlockchainService,
  private val simulatorService: BlockSimulationService,
  private val synchronizationService: SynchronizationService,
  private val blockHashLookup: BlockHashLookupWithRecoverySupport = BlockHashLookupWithRecoverySupport(
    lookbackWindow = 256UL,
  ),
) {
  private val log = LogManager.getLogger(BlockImporter::class.java)
  private val chainId = blockchainService.chainId.orElseThrow().toULong()

  fun addLookbackHashes(blocksHashes: Map<ULong, ByteArray>) {
    blockHashLookup.addLookbackHashes(blocksHashes)
  }

  fun importBlock(block: BlockFromL1RecoveredData): PluginBlockSimulationResult {
    val executedBlockResult = executeBlockWithTransactionsWithoutSignature(block)
    val result = importBlock(BlockContextData(executedBlockResult.blockHeader, executedBlockResult.blockBody))
    blockHashLookup.addHeadBlockHash(block.header.blockNumber, block.header.blockHash)
    return result
  }

  private fun executeBlockWithTransactionsWithoutSignature(
    block: BlockFromL1RecoveredData,
  ): PluginBlockSimulationResult {
    log.trace(
      "simulating import block={} blockHash={}",
      block.header.blockNumber,
      block.header.blockHash.encodeHex(),
    )
    val transactions = TransactionMapper.mapToBesu(
      block.transactions,
      chainId,
    )
    val parentBlockNumber = block.header.blockNumber.toLong() - 1

    val executedBlockResult =
      simulatorService.simulate(
        parentBlockNumber,
        transactions,
        createOverrides(block, blockHashLookup::getHash),
        StateOverrideMap(),
      )

    log.trace(
      " import simulation result: block={} blockHeader={}",
      executedBlockResult.blockHeader.number,
      executedBlockResult.blockHeader,
    )
    return executedBlockResult
  }

  fun importBlock(context: BlockContext): PluginBlockSimulationResult {
    log.trace(
      "calling simulateAndPersistWorldState block={} blockHeader={}",
      context.blockHeader.number,
      context.blockHeader,
    )
    val parentBlockNumber = context.blockHeader.number - 1
    val importedBlockResult =
      simulatorService.simulateAndPersistWorldState(
        parentBlockNumber,
        context.blockBody.transactions,
        createOverrides(context.blockHeader, blockHashLookup::getHash),
        StateOverrideMap(),
      )
    log.trace(
      "simulateAndPersistWorldState result: block={} blockHeader={}",
      context.blockHeader.number,
      importedBlockResult.blockHeader,
    )
    storeAndSetHead(importedBlockResult)
    return importedBlockResult
  }

  private fun storeAndSetHead(block: PluginBlockSimulationResult) {
    log.debug(
      "storeAndSetHead result: blockHeader={}",
      block.blockHeader,
    )
    blockchainService.storeBlock(
      block.blockHeader,
      block.blockBody,
      block.receipts,
    )
    synchronizationService.setHeadUnsafe(block.blockHeader, block.blockBody)
  }

  companion object {
    fun createOverrides(
      blockFromBlob: BlockFromL1RecoveredData,
      blockHashLookup: (Long) -> Hash,
    ): BlockOverrides {
      return BlockOverrides.builder()
        .blockHash(Hash.wrap(Bytes32.wrap(blockFromBlob.header.blockHash)))
        .feeRecipient(Address.fromHexString(blockFromBlob.header.coinbase.encodeHex()))
        .blockNumber(blockFromBlob.header.blockNumber.toLong())
        .gasLimit(blockFromBlob.header.gasLimit.toLong())
        .timestamp(blockFromBlob.header.blockTimestamp.epochSeconds)
        .difficulty(blockFromBlob.header.difficulty.toBigInteger())
        .mixHashOrPrevRandao(Hash.ZERO)
        .blockHashLookup(blockHashLookup)
        .build()
    }

    fun createOverrides(
      blockHeader: BlockHeader,
      blockHashLookup: (Long) -> Hash,
    ): BlockOverrides {
      return BlockOverrides.builder()
        .feeRecipient(blockHeader.coinbase)
        .blockNumber(blockHeader.number)
        .gasLimit(blockHeader.gasLimit)
        .timestamp(blockHeader.timestamp)
        .difficulty(blockHeader.difficulty.asBigInteger)
        .stateRoot(blockHeader.stateRoot)
        .mixHashOrPrevRandao(Hash.ZERO)
        .blockHashLookup(blockHashLookup)
        .build()
    }
  }
}
