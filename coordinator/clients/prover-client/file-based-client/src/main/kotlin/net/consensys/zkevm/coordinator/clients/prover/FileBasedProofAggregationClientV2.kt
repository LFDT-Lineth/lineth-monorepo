package net.consensys.zkevm.coordinator.clients.prover

import com.fasterxml.jackson.databind.ObjectMapper
import io.vertx.core.Vertx
import linea.kotlin.encodeHex
import net.consensys.zkevm.coordinator.clients.ProofAggregationProverClientV2
import net.consensys.zkevm.coordinator.clients.prover.serialization.JsonSerialization
import net.consensys.zkevm.coordinator.clients.prover.serialization.ProofToFinalizeJsonResponse
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.domain.ProofToFinalize
import net.consensys.zkevm.domain.ProofsToAggregate
import net.consensys.zkevm.ethereum.crypto.HashFunction
import net.consensys.zkevm.ethereum.crypto.Sha256HashFunction
import net.consensys.zkevm.fileio.FileReader
import net.consensys.zkevm.fileio.FileWriter
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class AggregationProofRequestDto(
  val executionProofs: List<String>,
  val compressionProofs: List<String>,
  val parentAggregationLastBlockTimestamp: Long,
  val parentAggregationLastL1RollingHashMessageNumber: Long,
  val parentAggregationLastL1RollingHash: String
) {
  companion object {
    fun fromDomainObject(
      proofsToAggregate: ProofsToAggregate,
      executionProofResponseFileNameProvider: ProverFileNameProvider,
      compressionProofResponseFileNameProvider: ProverFileNameProvider
    ): AggregationProofRequestDto {
      val executionProofsResponseFiles = proofsToAggregate.executionProofs
        .toIntervalList()
        .map { blockInterval ->
          executionProofResponseFileNameProvider.getFileName(
            ProofIndex(
              startBlockNumber = blockInterval.startBlockNumber,
              endBlockNumber = blockInterval.endBlockNumber
            )
          )
        }

      val compressionProofsResponsesFiles = proofsToAggregate.compressionProofIndexes
        .map {
          compressionProofResponseFileNameProvider.getFileName(
            ProofIndex(
              startBlockNumber = it.startBlockNumber,
              endBlockNumber = it.endBlockNumber,
              hash = it.hash
            )
          )
        }

      return AggregationProofRequestDto(
        executionProofs = executionProofsResponseFiles,
        compressionProofs = compressionProofsResponsesFiles,
        parentAggregationLastBlockTimestamp = proofsToAggregate.parentAggregationLastBlockTimestamp.epochSeconds,
        parentAggregationLastL1RollingHashMessageNumber =
        proofsToAggregate.parentAggregationLastL1RollingHashMessageNumber.toLong(),
        parentAggregationLastL1RollingHash = proofsToAggregate.parentAggregationLastL1RollingHash.encodeHex()
      )
    }
  }
}

internal class AggregationRequestDtoMapper(
  private val executionProofResponseFileNameProvider: ProverFileNameProvider,
  private val compressionProofResponseFileNameProvider: ProverFileNameProvider
) : (ProofsToAggregate) -> SafeFuture<AggregationProofRequestDto> {
  override fun invoke(proofsToAggregate: ProofsToAggregate): SafeFuture<AggregationProofRequestDto> {
    return SafeFuture.completedFuture(
      AggregationProofRequestDto.fromDomainObject(
        proofsToAggregate,
        executionProofResponseFileNameProvider,
        compressionProofResponseFileNameProvider
      )
    )
  }
}

/**
 * Implementation of interface with the Aggregation Prover through Files.
 *
 * Aggregation Prover will ingest file like
 * path/to/prover/requests/<startBlockNumber>-<endBlockNumber>-<requestHash>-getZkAggregatedProof.json
 *
 * When done prover will output file
 * path/to/prover/responses/<startBlockNumber>-<endBlockNumber>-<requestHash>-getZkAggregatedProof.json
 *
 * So, this class will need to watch the file system and wait for the output proof to be generated
 */
class FileBasedProofAggregationClientV2(
  vertx: Vertx,
  config: FileBasedProverConfig,
  hashFunction: HashFunction = Sha256HashFunction(),
  executionProofResponseFileNameProvider: ProverFileNameProvider = ExecutionProofResponseFileNameProvider,
  compressionProofResponseFileNameProvider: ProverFileNameProvider = CompressionProofResponseFileNameProvider,
  jsonObjectMapper: ObjectMapper = JsonSerialization.proofResponseMapperV1
) :
  GenericFileBasedProverClient<
    ProofsToAggregate,
    ProofToFinalize,
    AggregationProofRequestDto,
    ProofToFinalizeJsonResponse
    >(
    config = config,
    vertx = vertx,
    fileWriter = FileWriter(vertx, jsonObjectMapper),
    fileReader = FileReader(
      vertx,
      jsonObjectMapper,
      ProofToFinalizeJsonResponse::class.java
    ),
    requestFileNameProvider = AggregationProofFileNameProvider,
    responseFileNameProvider = AggregationProofFileNameProvider,
    proofIndexProvider = createProofIndexProviderFn(hashFunction),
    requestMapper = AggregationRequestDtoMapper(
      executionProofResponseFileNameProvider = executionProofResponseFileNameProvider,
      compressionProofResponseFileNameProvider = compressionProofResponseFileNameProvider
    ),
    responseMapper = ProofToFinalizeJsonResponse::toDomainObject,
    proofTypeLabel = "aggregation",
    log = LogManager.getLogger(this::class.java)
  ),
  ProofAggregationProverClientV2 {

  companion object {
    fun createProofIndexProviderFn(
      hashFunction: HashFunction,
      executionProofResponseFileNameProvider: ProverFileNameProvider = ExecutionProofResponseFileNameProvider,
      compressionProofResponseFileNameProvider: ProverFileNameProvider = CompressionProofResponseFileNameProvider
    ): (ProofsToAggregate) -> ProofIndex {
      return { request: ProofsToAggregate ->

        val requestDto = AggregationProofRequestDto.fromDomainObject(
          proofsToAggregate = request,
          executionProofResponseFileNameProvider = executionProofResponseFileNameProvider,
          compressionProofResponseFileNameProvider = compressionProofResponseFileNameProvider
        )
        val hash = hashRequest(hashFunction, requestDto)
        ProofIndex(
          startBlockNumber = request.startBlockNumber,
          endBlockNumber = request.endBlockNumber,
          hash = hash
        )
      }
    }

    private fun hashRequest(hashFunction: HashFunction, request: AggregationProofRequestDto): ByteArray {
      val contentBytes = (request.compressionProofs + request.executionProofs).joinToString().toByteArray()
      return hashFunction.hash(contentBytes)
    }
  }
}
