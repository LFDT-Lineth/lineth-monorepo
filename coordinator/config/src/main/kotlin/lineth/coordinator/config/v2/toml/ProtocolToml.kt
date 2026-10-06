package lineth.coordinator.config.v2.toml

import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import linea.domain.BlockParameter
import lineth.coordinator.config.v2.ProtocolConfig
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class ProtocolToml(
  @param:ConfigSection("Lineth genesis state root and shnarf.")
  val genesis: Genesis,
  @param:ConfigSection("L1 rollup contract and timing settings.")
  val l1: Layer1Config,
  @param:ConfigSection("L2 contract settings.")
  val l2: Layer2Config,
) {
  data class Genesis(
    @param:ConfigDoc(
      description = "Genesis state root hash of the Lineth chain (hex).",
      example = "0x01d9afcd495c870f3ae9d8362cd0257a7de2057055058183596719285cae6101",
    )
    val genesisStateRootHash: ByteArray,
    @param:ConfigDoc(
      description = "Genesis shnarf (starting shnarf) of the Lineth chain (hex).",
      example = "0xc286ff42414401ccdc23ea8e738775378e8f6c6f7b2966eb2747798d45571b79",
    )
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
    @param:ConfigDoc(
      description = "Address of the Lineth rollup contract on L1.",
      example = "0xDc64a140Aa3E981100a9becA4E685f962f0cF6C9",
    )
    val contractAddress: String,
    @param:ConfigDoc(description = "Average L1 block time, used for timing estimates.", default = "PT12S")
    val blockTime: Duration = 12.seconds,
    @param:ConfigDoc(
      description = "L1 block number at which the rollup contract was deployed. Omit if not applicable.",
      example = "3",
    )
    val contractDeploymentBlockNumber: ULong?,
  )

  data class Layer2Config(
    @param:ConfigDoc(
      description = "Address of the Lineth contract on L2.",
      example = "0xe537D669CA013d86EBeF1D64e40fC74CADC91987",
    )
    val contractAddress: String,
    // hoplite limitation: it does not work with nullable BlockParameter.BlockNumber?
    @param:ConfigDoc(
      description = "L2 block number at which the contract was deployed. Omit if not applicable.",
      example = "3",
    )
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
        contractDeploymentBlockNumber = this.l1.contractDeploymentBlockNumber
          ?.let { BlockParameter.BlockNumber(it) },
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
