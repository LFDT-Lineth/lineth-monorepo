package linea.staterecovery.plugin

import linea.staterecovery.BlockFromL1RecoveredData
import net.consensys.encodeHex
import net.consensys.toBigInteger
import net.consensys.toULong
import org.apache.logging.log4j.LogManager
import org.apache.tuweni.bytes.Bytes32
import org.hyperledger.besu.datatypes.AccountOverrideMap
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Hash
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
  private val synchronizationService: SynchronizationService
) {
  private val log = LogManager.getLogger(BlockImporter::class.java)
  private val chainId = blockchainService.chainId.orElseThrow().toULong()

  fun importBlock(block: BlockFromL1RecoveredData): PluginBlockSimulationResult {
    val executedBlockResult = executeBlockWithTransactionsWithoutSignature(block)
    return importBlock(BlockContextData(executedBlockResult.blockHeader, executedBlockResult.blockBody))
  }

  private fun executeBlockWithTransactionsWithoutSignature(
    block: BlockFromL1RecoveredData
  ): PluginBlockSimulationResult {
    log.debug(
      "simulating import block={} blockHash={}",
      block.header.blockNumber,
      block.header.blockHash.encodeHex()
    )
    val transactions = TransactionMapper.mapToBesu(
      block.transactions,
      chainId
    )
    val parentBlockNumber = block.header.blockNumber.toLong() - 1

    val executedBlockResult =
      simulatorService.simulate(
        parentBlockNumber,
        transactions,
        createOverrides(block),
        AccountOverrideMap()
      )

    log.debug(
      " import simulation result: block={} blockHeader={}",
      executedBlockResult.blockHeader.number,
      executedBlockResult.blockHeader
    )
    return executedBlockResult
  }

  private fun createOverrides(blockFromBlob: BlockFromL1RecoveredData): BlockOverrides {
    return BlockOverrides.builder()
      .blockHash(Hash.wrap(Bytes32.wrap(blockFromBlob.header.blockHash)))
      .feeRecipient(Address.fromHexString(blockFromBlob.header.coinbase.encodeHex()))
      .blockNumber(blockFromBlob.header.blockNumber.toLong())
      .gasLimit(blockFromBlob.header.gasLimit.toLong())
      .timestamp(blockFromBlob.header.blockTimestamp.epochSeconds)
      .difficulty(blockFromBlob.header.difficulty.toBigInteger())
      .mixHashOrPrevRandao(Hash.ZERO)
      .build()
  }

  fun importBlock(context: BlockContext): PluginBlockSimulationResult {
    log.debug(
      "calling simulateAndPersistWorldState block={} blockHeader={}",
      context.blockHeader.number,
      context.blockHeader
    )
    val parentBlockNumber = context.blockHeader.number - 1
    val importedBlockResult =
      simulatorService.simulateAndPersistWorldState(
        parentBlockNumber,
        context.blockBody.transactions,
        createOverrides(context.blockHeader),
        AccountOverrideMap()
      )
    log.debug(
      "simulateAndPersistWorldState result: block={} blockHeader={}",
      context.blockHeader.number,
      importedBlockResult.blockHeader
    )
    storeAndSetHead(importedBlockResult)
    return importedBlockResult
  }

  private fun createOverrides(blockHeader: BlockHeader): BlockOverrides {
    return BlockOverrides.builder()
      .feeRecipient(blockHeader.coinbase)
      .blockNumber(blockHeader.number)
      .gasLimit(blockHeader.gasLimit)
      .timestamp(blockHeader.timestamp)
      .difficulty(blockHeader.difficulty.asBigInteger)
      .stateRoot(blockHeader.stateRoot)
      .mixHashOrPrevRandao(Hash.ZERO)
      .build()
  }

  private fun storeAndSetHead(block: PluginBlockSimulationResult) {
    log.debug(
      "storeAndSetHead result: blockHeader={}",
      block.blockHeader
    )
    blockchainService.storeBlock(
      block.blockHeader,
      block.blockBody,
      block.receipts
    )
    synchronizationService.setHeadUnsafe(block.blockHeader, block.blockBody)
  }
}
