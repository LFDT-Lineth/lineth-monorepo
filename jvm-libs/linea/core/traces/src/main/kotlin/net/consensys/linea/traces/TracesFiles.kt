package net.consensys.linea.traces

import org.apache.tuweni.bytes.Bytes32

typealias TracesFileNameSupplier = (
  blockNumber: ULong,
  blockHash: Bytes32,
  tracesEngineVersion: String,
  tracesFileExtension: String,
) -> String

object TracesFiles {
  fun rawTracesFileNameSupplierV1(
    blockNumber: ULong,
    blockHash: Bytes32,
    tracesEngineVersion: String,
    tracesFileExtension: String,
  ): String {
    return "$blockNumber-${blockHash.toHexString().lowercase()}.v$tracesEngineVersion.$tracesFileExtension"
  }
}
