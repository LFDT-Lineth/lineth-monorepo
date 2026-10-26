package lineth.coordination.riscv.rollup

import linea.domain.DataRollingHashCalculator
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.datatypes.Hash

/** DRH_n = keccak256(DRH_{n-1} || chunkHash_n) */
object Keccak256DataRollingHashCalculator : DataRollingHashCalculator {
  override fun fold(parentDataRollingHash: ByteArray, chunkHash: ByteArray): ByteArray {
    require(parentDataRollingHash.size == 32) {
      "parentDataRollingHash must be 32 bytes, got ${parentDataRollingHash.size}"
    }
    require(chunkHash.size == 32) { "chunkHash must be 32 bytes, got ${chunkHash.size}" }
    return Hash.hash(Bytes.wrap(Bytes.wrap(parentDataRollingHash), Bytes.wrap(chunkHash))).bytes.toArray()
  }
}
