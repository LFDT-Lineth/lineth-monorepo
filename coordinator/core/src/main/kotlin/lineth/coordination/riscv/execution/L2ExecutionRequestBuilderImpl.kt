package lineth.coordination.riscv.execution

import linea.clients.ChainConfig
import linea.clients.ExecutionInfo
import linea.clients.ForcedTransaction
import linea.clients.L2ExecutionProofRequestV1
import linea.domain.Block
import linea.domain.BlocksConflation
import linea.domain.toBlockParameter
import linea.domain.toExecutionPayload
import linea.ethapi.ExecutionWitness
import linea.ethapi.ExecutionWitnessClient
import linea.kotlin.encodeHex
import linea.kotlin.minusCoercingUnderflow
import lineth.coordination.FtxRollingInfoProvider
import lineth.coordination.FtxRollingInfoProviderImpl
import lineth.persistence.ForcedTransactionRecord
import lineth.persistence.ForcedTransactionsDao
import tech.pegasys.teku.infrastructure.async.SafeFuture

class L2ExecutionRequestBuilderImpl(
  private val executionWitnessClient: ExecutionWitnessClient,
  private val forcedTransactionsDao: ForcedTransactionsDao,
  private val ftxRollingInfoProvider: FtxRollingInfoProvider = FtxRollingInfoProviderImpl(forcedTransactionsDao),
  private val chainConfig: ChainConfig,
) : L2ExecutionRequestBuilder {

  override fun build(conflation: BlocksConflation): SafeFuture<L2ExecutionProofRequestV1> {
    val allFtxsFuture = forcedTransactionsDao.findBySimulatedExecutionBlock(
      conflation.startBlockNumber..conflation.endBlockNumber,
    )
    val parentBlockNumber = conflation.startBlockNumber.minusCoercingUnderflow(1uL)
    val ftxStateFuture = ftxRollingInfoProvider.getFtxRollingHashByBlockNumber(parentBlockNumber)
    val allWitnessesListFuture = SafeFuture.collectAll(
      conflation.blocks.map { block ->
        executionWitnessClient.getExecutionWitness(block.number.toBlockParameter())
          .thenApply { witness ->
            requireNotNull(witness) { "No execution witness available for block ${block.number}" }
          }
      }.stream(),
    )

    return SafeFuture.allOf(allFtxsFuture, ftxStateFuture, allWitnessesListFuture)
      .thenApply {
        val ftxsByBlock = allFtxsFuture.get()
          .groupBy { it.simulatedExecutionBlockNumber }
          .mapValues { (_, records) -> records.map(ForcedTransactionRecord::toDomain) }
        val ftxState = ftxStateFuture.get()

        L2ExecutionProofRequestV1(
          executions = conflation.blocks.zip(allWitnessesListFuture.get()).map { (block, witness) ->
            block.toExecutionInfo(
              witness = witness,
              forcedTransactions = ftxsByBlock[block.number] ?: emptyList(),
            )
          },
          chainConfig = chainConfig,
          coinbase = conflation.blocks.first().miner.encodeHex(),
          parentFtxRollingHash = ftxState.ftxRollingHash,
          parentFtxNumber = ftxState.ftxNumber,
        )
      }
  }
}

private fun Block.toExecutionInfo(witness: ExecutionWitness, forcedTransactions: List<ForcedTransaction>) =
  ExecutionInfo(
    blockNumber = number,
    executionPayload = toExecutionPayload(),
    executionWitness = witness,
    executionRequests = emptyList(),
    forcedTransactions = forcedTransactions,
  )

private fun ForcedTransactionRecord.toDomain() = ForcedTransaction(
  ftxNumber = ftxNumber,
  deadlineBlockNumber = ftxBlockNumberDeadline,
  signedTxRlp = ftxRlp,
  acceptance = inclusionResult,
)
