package linea.ethapi

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class EthConfigClientTest {
  private val amsterdamContracts = mapOf(
    "BEACON_ROOTS_ADDRESS" to "0x000F3df6D732807Ef1319fB7B8bB8522d0Beac02",
    "HISTORY_STORAGE_ADDRESS" to "0x0000f90827f1c53a10cb7a02335b175320002935",
    "DEPOSIT_CONTRACT_ADDRESS" to "0x94d4a7449b968b839dcbc64a61f195ca300986ae",
    "WITHDRAWAL_REQUEST_PREDEPLOY_ADDRESS" to "0x66355689a9f067eeb9dc9d899e4192676988279c",
    "CONSOLIDATION_REQUEST_PREDEPLOY_ADDRESS" to "0xf0e003f0de2d583ae28fa8cbf66aa096cdace3ff",
    "BUILDER_DEPOSIT_CONTRACT_ADDRESS" to "0x0000bff46984e3725691fa540a8c7589300d8282",
    "BUILDER_EXIT_CONTRACT_ADDRESS" to "0x000064d678505ad48f8ccb093bc65613800e8282",
  )
  private val pragueContracts =
    amsterdamContracts - "BUILDER_DEPOSIT_CONTRACT_ADDRESS" - "BUILDER_EXIT_CONTRACT_ADDRESS"
  private val expected = SystemContracts(
    beaconRoots = "0x000f3df6d732807ef1319fb7b8bb8522d0beac02",
    historyStorage = "0x0000f90827f1c53a10cb7a02335b175320002935",
    deposit = "0x94d4a7449b968b839dcbc64a61f195ca300986ae",
    withdrawalRequest = "0x66355689a9f067eeb9dc9d899e4192676988279c",
    consolidationRequest = "0xf0e003f0de2d583ae28fa8cbf66aa096cdace3ff",
    builderDepositRequest = "0x0000bff46984e3725691fa540a8c7589300d8282",
    builderExitRequest = "0x000064d678505ad48f8ccb093bc65613800e8282",
  )

  private fun fork(contracts: Map<String, String>, chainId: ULong = 59144UL, activationTime: ULong = 0UL) =
    ForkConfig(activationTime = activationTime, chainId = chainId, systemContracts = contracts)

  @Test
  fun `maps every eth_config key in field order and lowercases addresses`() {
    val config = EthConfig(current = fork(amsterdamContracts), next = null, last = null)
    assertThat(config.systemContracts(59144UL)).isEqualTo(expected)
  }

  @Test
  fun `uses the current fork and ignores scheduled ones`() {
    val other = amsterdamContracts + ("DEPOSIT_CONTRACT_ADDRESS" to "0x" + "11".repeat(20))
    val config = EthConfig(
      current = fork(amsterdamContracts),
      next = fork(other, activationTime = 100UL),
      last = fork(other, activationTime = 100UL),
    )
    assertThat(config.systemContracts(59144UL)).isEqualTo(expected)
  }

  @Test
  fun `rejects when the current fork does not define every system contract`() {
    val config = EthConfig(
      current = fork(pragueContracts),
      next = fork(amsterdamContracts, activationTime = 100UL),
      last = null,
    )
    assertThatThrownBy { config.systemContracts(59144UL) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("current fork does not define all")
  }

  @Test
  fun `rejects a chain id different from the expected one`() {
    val config = EthConfig(current = fork(amsterdamContracts, chainId = 1UL), next = null, last = null)
    assertThatThrownBy { config.systemContracts(59144UL) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("chainId=1")
  }
}
