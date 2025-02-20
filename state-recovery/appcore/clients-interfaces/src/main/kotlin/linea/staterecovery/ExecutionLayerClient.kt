package linea.staterecovery

import linea.domain.BlockNumberAndHash
import linea.domain.BlockParameter
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class StateRecoveryStatus(
  val headBlockNumber: ULong,
  val stateRecoverStartBlockNumber: ULong?
)
interface ExecutionLayerClient {
  fun getBlockNumberAndHash(blockParameter: BlockParameter): SafeFuture<BlockNumberAndHash>
  fun addLookbackHashes(blocksHashes: Map<ULong, ByteArray>): SafeFuture<Unit>
  fun lineaEngineImportBlocksFromBlob(blocks: List<BlockFromL1RecoveredData>): SafeFuture<Unit>
  fun lineaGetStateRecoveryStatus(): SafeFuture<StateRecoveryStatus>
  fun lineaEnableStateRecovery(stateRecoverStartBlockNumber: ULong): SafeFuture<StateRecoveryStatus>
}
