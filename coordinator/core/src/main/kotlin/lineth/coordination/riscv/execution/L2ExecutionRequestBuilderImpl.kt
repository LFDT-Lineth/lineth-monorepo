package lineth.coordination.riscv.execution

import linea.clients.ExecutionInfo
import linea.clients.ForcedTransaction
import linea.clients.L2ExecutionProofRequestV1
import linea.domain.BlockNumberAndHash
import linea.domain.BlockParameter
import linea.domain.BlocksConflation
import linea.domain.ExecutionPayload
import linea.ethapi.ExecutionPayloadClient
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
  private val executionPayloadClient: ExecutionPayloadClient,
  private val forcedTransactionsDao: ForcedTransactionsDao,
  private val ftxRollingInfoProvider: FtxRollingInfoProvider = FtxRollingInfoProviderImpl(forcedTransactionsDao),
  private val chainId: ULong,
) : L2ExecutionRequestBuilder {

  override fun build(conflation: BlocksConflation): SafeFuture<L2ExecutionProofRequestV1> {
    val allFtxsFuture = forcedTransactionsDao.findBySimulatedExecutionBlock(
      conflation.startBlockNumber..conflation.endBlockNumber,
    )
    val parentBlockNumber = conflation.startBlockNumber.minusCoercingUnderflow(1uL)
    val ftxStateFuture = ftxRollingInfoProvider.getFtxRollingHashByBlockNumber(parentBlockNumber)
    val allExecutionsListFuture = SafeFuture.collectAll(
      conflation.blocks.map { block ->
        val payloadFuture = executionPayloadClient.getExecutionPayload(BlockNumberAndHash(block.number, block.hash))
        val witnessFuture = executionWitnessClient.getExecutionWitness(BlockParameter.fromHash(block.hash))
        payloadFuture.thenCombine(witnessFuture) { payload, witness ->
          payload to requireNotNull(witness) { "No execution witness available for block ${block.number}" }
        }
      }.stream(),
    )

    return SafeFuture.allOf(allFtxsFuture, ftxStateFuture, allExecutionsListFuture)
      .thenApply {
        val ftxsByBlock = allFtxsFuture.get()
          .groupBy { it.simulatedExecutionBlockNumber }
          .mapValues { (_, records) -> records.map(ForcedTransactionRecord::toDomain) }
        val ftxState = ftxStateFuture.get()

        L2ExecutionProofRequestV1(
          executions = allExecutionsListFuture.get().map { (payload, witness) ->
            payload.toExecutionInfo(
              witness = witness,
              forcedTransactions = ftxsByBlock[payload.blockNumber] ?: emptyList(),
            )
          },
          chainId = chainId,
          coinbase = conflation.blocks.first().miner.encodeHex(),
          parentFtxRollingHash = ftxState.ftxRollingHash,
          parentFtxNumber = ftxState.ftxNumber,
        )
      }
  }
}

private fun ExecutionPayload.toExecutionInfo(witness: ExecutionWitness, forcedTransactions: List<ForcedTransaction>) =
  ExecutionInfo(
    blockNumber = blockNumber,
    executionPayload = this,
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
