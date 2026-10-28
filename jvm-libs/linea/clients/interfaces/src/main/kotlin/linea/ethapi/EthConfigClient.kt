package linea.ethapi

import tech.pegasys.teku.infrastructure.async.SafeFuture

/**
 * Client for the EIP-7910 `eth_config` JSON-RPC method.
 */
interface EthConfigClient {
  fun ethConfig(): SafeFuture<EthConfig>
}

/**
 * `eth_config` result: the fork configuration currently in effect and, when a future fork is scheduled, the next
 * and the last configured ones.
 */
data class EthConfig(
  val current: ForkConfig,
  val next: ForkConfig?,
  val last: ForkConfig?,
)

data class ForkConfig(
  val activationTime: ULong,
  val chainId: ULong,
  /** eth_config `systemContracts`: EIP name (e.g. `BEACON_ROOTS_ADDRESS`) to 0x-prefixed lowercase address. */
  val systemContracts: Map<String, String>,
)

/**
 * System-contract addresses the execution engine uses for the chain, as 0x-prefixed lowercase hex.
 * Field order matches the RISC-V guest input.
 */
data class SystemContracts(
  val beaconRoots: String,
  val historyStorage: String,
  val deposit: String,
  val withdrawalRequest: String,
  val consolidationRequest: String,
  val builderDepositRequest: String,
  val builderExitRequest: String,
) {
  companion object {
    /** eth_config `systemContracts` keys, in field order. */
    val ETH_CONFIG_KEYS: List<String> = listOf(
      "BEACON_ROOTS_ADDRESS",
      "HISTORY_STORAGE_ADDRESS",
      "DEPOSIT_CONTRACT_ADDRESS",
      "WITHDRAWAL_REQUEST_PREDEPLOY_ADDRESS",
      "CONSOLIDATION_REQUEST_PREDEPLOY_ADDRESS",
      "BUILDER_DEPOSIT_CONTRACT_ADDRESS",
      "BUILDER_EXIT_CONTRACT_ADDRESS",
    )

    /** Returns null when [systemContracts] does not define every key in [ETH_CONFIG_KEYS] (e.g. a pre-Amsterdam fork). */
    fun fromEthConfig(systemContracts: Map<String, String>): SystemContracts? {
      val addresses = ETH_CONFIG_KEYS.map { key -> systemContracts[key]?.lowercase() ?: return null }
      return SystemContracts(
        beaconRoots = addresses[0],
        historyStorage = addresses[1],
        deposit = addresses[2],
        withdrawalRequest = addresses[3],
        consolidationRequest = addresses[4],
        builderDepositRequest = addresses[5],
        builderExitRequest = addresses[6],
      )
    }
  }
}

/**
 * The system contracts of the fork currently in effect, for proving [expectedChainId].
 * Fails when the current fork does not define all of them (e.g. a pre-Amsterdam fork).
 */
fun EthConfig.systemContracts(expectedChainId: ULong): SystemContracts {
  require(current.chainId == expectedChainId) {
    "eth_config chainId=${current.chainId} does not match expected chainId=$expectedChainId"
  }
  return requireNotNull(SystemContracts.fromEthConfig(current.systemContracts)) {
    "eth_config current fork does not define all of ${SystemContracts.ETH_CONFIG_KEYS}: ${current.systemContracts}"
  }
}
