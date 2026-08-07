package lineth.ftx

import com.github.michaelbull.result.Result
import lineth.clients.GetZkEVMStateMerkleProofResponse
import lineth.clients.LineaAccountProof
import lineth.clients.StateManagerAccountProofClient
import lineth.clients.StateManagerClientV1
import lineth.clients.StateManagerErrorType
import lineth.domain.BlockInterval
import lineth.error.ErrorResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

class FakeStateManagerClient :
  StateManagerClientV1,
  StateManagerAccountProofClient {
  override fun rollupGetStateMerkleProofWithTypedError(
    blockInterval: BlockInterval,
  ): SafeFuture<Result<GetZkEVMStateMerkleProofResponse, ErrorResponse<StateManagerErrorType>>> {
    TODO("Not yet implemented")
  }

  override fun rollupGetVirtualStateMerkleProofWithTypedError(
    blockNumber: ULong,
    transaction: ByteArray,
  ): SafeFuture<Result<GetZkEVMStateMerkleProofResponse, ErrorResponse<StateManagerErrorType>>> {
    TODO("Not yet implemented")
  }

  override fun rollupGetHeadBlockNumber(): SafeFuture<ULong> {
    TODO("Not yet implemented")
  }

  override fun lineaGetAccountProof(
    address: ByteArray,
    storageKeys: List<ByteArray>,
    blockNumber: ULong,
  ): SafeFuture<LineaAccountProof> {
    TODO("Not yet implemented")
  }
}
