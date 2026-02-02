package net.consensys.zkevm.coordinator.clients.prover

import build.linea.clients.LineaAccountProof
import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.node.ArrayNode
import io.vertx.core.Vertx
import linea.kotlin.encodeHex
import net.consensys.zkevm.coordinator.clients.InvalidityProofRequest
import net.consensys.zkevm.coordinator.clients.InvalidityProofResponse
import net.consensys.zkevm.coordinator.clients.InvalidityProverClientV1
import net.consensys.zkevm.coordinator.clients.prover.serialization.JsonSerialization
import net.consensys.zkevm.domain.ProofIndex
import net.consensys.zkevm.fileio.FileReader
import net.consensys.zkevm.fileio.FileWriter
import org.apache.logging.log4j.LogManager
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.nio.file.Path

data class InvalidityProofRequestDto(
  val ftxRLP: String,
  val ftxNumber: Long,
  val prevFtxRollingHash: String,
  val ftxBlockNumberDeadline: Long,
  val invalidityType: String,
  val zkParentStateRootHash: String,
  val conflatedExecutionTracesFile: String,
  val accountProof: AccountProofDto?,
  val zkStateMerkleProof: ArrayNode?,
  val simulatedExecutionBlockNumber: Long,
  val simulatedExecutionBlockTimestamp: Long,
) {
  companion object {
    fun fromDomainObject(invalidityProofRequest: InvalidityProofRequest): InvalidityProofRequestDto {
      return InvalidityProofRequestDto(
        ftxRLP = invalidityProofRequest.ftxRlp.encodeHex(),
        ftxNumber = invalidityProofRequest.ftxNumber.toLong(),
        prevFtxRollingHash = invalidityProofRequest.prevFtxRollingHash.encodeHex(),
        ftxBlockNumberDeadline = invalidityProofRequest.ftxBlockNumberDeadline.toLong(),
        invalidityType = invalidityProofRequest.invalidityReason.name,
        zkParentStateRootHash = invalidityProofRequest.zkParentStateRootHash.encodeHex(),
        conflatedExecutionTracesFile = invalidityProofRequest.tracesResponse.tracesFileName,
        accountProof = invalidityProofRequest.accountProof?.let {
          AccountProofDto.fromDomainObject(it)
        },
        zkStateMerkleProof = invalidityProofRequest.zkStateMerkleProof?.zkStateMerkleProof,
        simulatedExecutionBlockNumber = invalidityProofRequest.simulatedExecutionBlockNumber.toLong(),
        simulatedExecutionBlockTimestamp = invalidityProofRequest.simulatedExecutionBlockTimestamp.epochSeconds,
      )
    }
  }
}

data class AccountProofDto(val accountProof: String) {
  companion object {
    fun fromDomainObject(lineaAccountProof: LineaAccountProof): AccountProofDto {
      return AccountProofDto(
        accountProof = lineaAccountProof.accountProof.encodeHex(),
      )
    }
  }
}

class FileBasedInvalidityProverClient(
  val config: FileBasedProverConfig,
  val vertx: Vertx,
  jsonObjectMapper: ObjectMapper = JsonSerialization.proofResponseMapperV1,
) :
  GenericFileBasedProverClient<
    InvalidityProofRequest,
    InvalidityProofResponse,
    InvalidityProofRequestDto,
    InvalidityProofResponse,
    >(
    config = config,
    vertx = vertx,
    fileWriter = FileWriter(vertx, jsonObjectMapper),
    fileReader = FileReader(
      vertx,
      jsonObjectMapper,
      InvalidityProofResponse::class.java,
    ),
    requestFileNameProvider = InvalidityProofFileNameProvider,
    responseFileNameProvider = InvalidityProofFileNameProvider,
    proofIndexProvider = FileBasedInvalidityProverClient::invalidityProofIndex,
    requestMapper = {
        invalidityProofRequest ->
      SafeFuture.completedFuture(
        InvalidityProofRequestDto.fromDomainObject(invalidityProofRequest),
      )
    },
    responseMapper = { throw UnsupportedOperationException("Invalidity proof response will not be parsed!") },
    proofTypeLabel = "invalidity",
    log = LogManager.getLogger(FileBasedInvalidityProverClient::class.java),
  ),
  InvalidityProverClientV1 {
  override fun parseResponse(responseFilePath: Path, proofIndex: ProofIndex): SafeFuture<InvalidityProofResponse> {
    return SafeFuture.completedFuture(
      InvalidityProofResponse(
        ftxNumber = proofIndex.endBlockNumber,
      ),
    )
  }

  companion object {
    fun invalidityProofIndex(request: InvalidityProofRequest): ProofIndex {
      return ProofIndex(
        startBlockNumber = request.simulatedExecutionBlockNumber,
        endBlockNumber = request.ftxNumber,
        hash = request.ftxHash,
      )
    }
  }
}
