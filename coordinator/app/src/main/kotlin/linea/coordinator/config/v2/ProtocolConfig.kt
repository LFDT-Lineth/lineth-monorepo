package linea.coordinator.config.v2

import linea.domain.BlockParameter
import linea.kotlin.encodeHex
import kotlin.time.Duration

data class ProtocolConfig(
  val genesis: Genesis,
  val l1: Layer1Config,
  val l2: Layer2Config,
) {
  data class Genesis(
    val genesisStateRootHash: ByteArray,
    val genesisShnarf: ByteArray,
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as Genesis

      if (!genesisStateRootHash.contentEquals(other.genesisStateRootHash)) return false
      if (!genesisShnarf.contentEquals(other.genesisShnarf)) return false

      return true
    }

    override fun hashCode(): Int {
      var result = genesisStateRootHash.contentHashCode()
      result = 31 * result + genesisShnarf.contentHashCode()
      return result
    }

    override fun toString(): String {
      return "Genesis(stateRootHash=${genesisStateRootHash.encodeHex()}, shnarf=${genesisShnarf.encodeHex()})"
    }
  }

  data class Layer1Config(
    val contractAddress: String,
    val blockTime: Duration,
  )

  data class Layer2Config(
    val contractAddress: String,
    val contractDeploymentBlockNumber: BlockParameter.BlockNumber?,
  )
}
