package linea.domain

import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Provides the data rolling hash the RISC-V DA stream starts from. Only needed while no RISC-V rollup is finalized on
 * L1; after that, resume anchors on the persisted rollup rows.
 */
fun interface RiscvGenesisDataRollingHashProvider {
  fun getGenesisDataRollingHash(): SafeFuture<ByteArray>
}
