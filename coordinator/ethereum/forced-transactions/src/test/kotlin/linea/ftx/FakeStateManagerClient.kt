package linea.ftx

import com.github.michaelbull.result.Result
import linea.clients.GetZkEVMStateMerkleProofResponse
import linea.clients.LineaAccountProof
import linea.clients.StateManagerAccountProofClient
import linea.clients.StateManagerClientV1
import linea.clients.StateManagerErrorType
import linea.domain.BlockInterval
import linea.error.ErrorResponse
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
