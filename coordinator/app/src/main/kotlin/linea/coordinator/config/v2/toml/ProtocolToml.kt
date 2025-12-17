package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.ProtocolConfig
import linea.domain.BlockParameter
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class ProtocolToml(
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
  }

  data class Layer1Config(
    val contractAddress: String,
    val blockTime: Duration = 12.seconds,
  )

  data class Layer2Config(
    val contractAddress: String,
    // hoplite limitation: it does not work with nullable BlockParameter.BlockNumber?
    val contractDeploymentBlockNumber: ULong?,
  )

  fun reified(): ProtocolConfig {
    return ProtocolConfig(
      genesis =
      ProtocolConfig.Genesis(
        genesisStateRootHash = this.genesis.genesisStateRootHash,
        genesisShnarf = this.genesis.genesisShnarf,
      ),
      l1 =
      ProtocolConfig.Layer1Config(
        contractAddress = this.l1.contractAddress,
        blockTime = this.l1.blockTime,
      ),
      l2 =
      ProtocolConfig.Layer2Config(
        contractAddress = this.l2.contractAddress,
        contractDeploymentBlockNumber =
        this.l2.contractDeploymentBlockNumber
          ?.let { BlockParameter.BlockNumber(it) },
      ),
    )
  }
}
