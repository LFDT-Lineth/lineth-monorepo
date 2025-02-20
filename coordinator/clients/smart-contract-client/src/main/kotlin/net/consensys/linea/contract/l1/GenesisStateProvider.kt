package net.consensys.linea.contract.l1

import linea.kotlin.encodeHex
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaGenesisStateProvider

data class GenesisStateProvider(
  override val stateRootHash: ByteArray,
  override val shnarf: ByteArray
) : LineaGenesisStateProvider {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as GenesisStateProvider

    if (!stateRootHash.contentEquals(other.stateRootHash)) return false
    if (!shnarf.contentEquals(other.shnarf)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = stateRootHash.contentHashCode()
    result = 31 * result + shnarf.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "GenesisStateProvider(stateRootHash=${stateRootHash.encodeHex()}, shnarf=${shnarf.encodeHex()})"
  }
}
