package lineth.coordinator.app.conflation

import linea.domain.RiscvGenesisDataRollingHashProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture

/** The genesis data rolling hash set in config (`conflation.rollup.genesis-data-rolling-hash`). */
class ConfigRiscvGenesisDataRollingHashProvider(
  private val genesisDataRollingHash: ByteArray,
) : RiscvGenesisDataRollingHashProvider {
  init {
    require(genesisDataRollingHash.size == 32) {
      "genesisDataRollingHash must be 32 bytes, got ${genesisDataRollingHash.size}"
    }
  }

  override fun getGenesisDataRollingHash(): SafeFuture<ByteArray> =
    SafeFuture.completedFuture(genesisDataRollingHash.copyOf())
}
