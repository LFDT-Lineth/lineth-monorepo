package linea.consensus

import org.hyperledger.besu.datatypes.HardforkId

interface HardForkIdProvider {
  fun getHardForkId(): HardforkId
}
