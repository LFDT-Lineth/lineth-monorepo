package build.linea.clients

import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Data class to contain Shomei's linea_getProof response.
 * Coordinator is completely oblivious to this response, for this reason,
 * we can just carry the JSON Response as ByteArray or jackson's JsonNode
 */
data class LineaAccountProof(val accountProof: ByteArray) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as LineaAccountProof

    return accountProof.contentEquals(other.accountProof)
  }

  override fun hashCode(): Int {
    return accountProof.contentHashCode()
  }
}

interface StateManagerAccountProofClient {
  fun lineaGetAccountProof(
    address: ByteArray,
    storageKeys: List<ByteArray>,
    blockNumber: ULong,
  ): SafeFuture<LineaAccountProof>
}
