package lineth.coordinator.app.conflation

import linea.clients.ChainConfig
import linea.domain.BlocksConflation
import lineth.coordinator.clients.prover.ProversConfig
import kotlin.time.Instant

class ChainConfigProvider(
  private val chainId: ULong,
  proversConfig: ProversConfig,
) : (BlocksConflation) -> ChainConfig {

  private val proverAForkName: String = requireNotNull(proversConfig.proverA.execution.forkName) {
    "riscvProver.fork-name must be configured when RISC-V is enabled"
  }
  private val proverBForkName: String? = proversConfig.proverB?.execution?.forkName
  private val switchBlockNumber: ULong? = proversConfig.switchBlockNumberInclusive
  private val switchTimestamp: Instant? = proversConfig.switchBlockTimestamp

  override fun invoke(conflation: BlocksConflation): ChainConfig {
    val forkName = when {
      switchBlockNumber != null ->
        if (conflation.startBlockNumber >= switchBlockNumber) {
          requireNotNull(proverBForkName) {
            "riscvProver.new.fork-name must be configured when switch-block-number-inclusive is set"
          }
        } else {
          proverAForkName
        }
      switchTimestamp != null -> {
        val blockTimestamp = Instant.fromEpochSeconds(conflation.blocks.first().timestamp.toLong())
        if (blockTimestamp >= switchTimestamp) {
          requireNotNull(proverBForkName) {
            "riscvProver.new.fork-name must be configured when switch-block-timestamp is set"
          }
        } else {
          proverAForkName
        }
      }
      else -> proverAForkName
    }
    return ChainConfig(chainId = chainId, forkName = forkName)
  }
}
