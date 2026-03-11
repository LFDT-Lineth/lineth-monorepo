package linea.ftx

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import build.linea.clients.LineaAccountProof
import build.linea.clients.StateManagerAccountProofClient
import build.linea.clients.StateManagerClientV1
import build.linea.clients.StateManagerErrorType
import com.github.michaelbull.result.Result
import linea.domain.BlockInterval
import net.consensys.linea.errors.ErrorResponse
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
