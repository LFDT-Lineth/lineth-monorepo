package build.linea.clients

import com.fasterxml.jackson.databind.node.ArrayNode
import com.github.michaelbull.result.Result
import linea.domain.BlockInterval
import linea.kotlin.encodeHex
import net.consensys.linea.errors.ErrorResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

enum class StateManagerErrorType : ClientError {
  UNKNOWN,
  UNSUPPORTED_VERSION,
  BLOCK_MISSING_IN_CHAIN,
}

sealed interface StateManagerRequest<ResponseType : StateManagerResponse> : ClientRequest<ResponseType>
sealed class GetChainHeadRequest() : StateManagerRequest<GetChainHeadResponse>

data class GetStateMerkleProofRequest(val blockInterval: BlockInterval) :
  StateManagerRequest<GetZkEVMStateMerkleProofResponse>,
  BlockInterval by blockInterval

sealed interface StateManagerResponse

data class GetZkEVMStateMerkleProofResponse(
  val zkStateMerkleProof: ArrayNode,
  val zkParentStateRootHash: ByteArray,
  val zkEndStateRootHash: ByteArray,
  val zkStateManagerVersion: String,
) : StateManagerResponse {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as GetZkEVMStateMerkleProofResponse

    if (zkStateMerkleProof != other.zkStateMerkleProof) return false
    if (!zkParentStateRootHash.contentEquals(other.zkParentStateRootHash)) return false
    if (!zkEndStateRootHash.contentEquals(other.zkEndStateRootHash)) return false
    if (zkStateManagerVersion != other.zkStateManagerVersion) return false

    return true
  }

  override fun hashCode(): Int {
    var result = zkStateMerkleProof.hashCode()
    result = 31 * result + zkParentStateRootHash.contentHashCode()
    result = 31 * result + zkEndStateRootHash.contentHashCode()
    result = 31 * result + zkStateManagerVersion.hashCode()
    return result
  }

  override fun toString(): String {
    return "GetZkEVMStateMerkleProofResponse(" +
      "zkStateMerkleProof=$zkStateMerkleProof, zkParentStateRootHash=${zkParentStateRootHash.encodeHex()}, " +
      "zkEndStateRootHash=${zkEndStateRootHash.encodeHex()}, " +
      "zkStateManagerVersion='$zkStateManagerVersion')"
  }
}

data class GetChainHeadResponse(val headBlockNumber: ULong) : StateManagerResponse

interface StateManagerClientV1 : AsyncClient<StateManagerRequest<*>> {
  /**
   * Get the head block number of the chain.
   * @return GetZkEVMStateMerkleProofResponse
   * @throws ClientException with errorType StateManagerErrorType when know error occurs
   */
  fun rollupGetStateMerkleProof(
    blockInterval: BlockInterval,
  ): SafeFuture<GetZkEVMStateMerkleProofResponse> = rollupGetStateMerkleProofWithTypedError(blockInterval)
    .unwrapResultMonad()

  /**
   * This is for backward compatibility with the old version in the coordinator side.
   * This error typing is not really usefull anymore
   */
  fun rollupGetStateMerkleProofWithTypedError(
    blockInterval: BlockInterval,
  ): SafeFuture<Result<GetZkEVMStateMerkleProofResponse, ErrorResponse<StateManagerErrorType>>>

  fun rollupGetHeadBlockNumber(): SafeFuture<ULong>

  override fun <Response> makeRequest(request: ClientRequest<Response>): SafeFuture<Response> {
    @Suppress("UNCHECKED_CAST")
    return when (request) {
      is GetStateMerkleProofRequest -> rollupGetStateMerkleProof(request.blockInterval) as SafeFuture<Response>
      is GetChainHeadRequest -> rollupGetHeadBlockNumber()
        .thenApply { GetChainHeadResponse(it) } as SafeFuture<Response>

      else -> throw IllegalArgumentException("Unknown request type: $request")
    }
  }
}
