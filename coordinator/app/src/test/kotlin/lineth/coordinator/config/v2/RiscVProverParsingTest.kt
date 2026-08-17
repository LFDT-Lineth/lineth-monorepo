package lineth.coordinator.config.v2

import lineth.coordinator.config.v2.toml.ProverToml
import lineth.coordinator.config.v2.toml.parseConfig
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds

class RiscVProverParsingTest {
  companion object {
    val toml =
      """
      [riscv-prover]
      fs-polling-interval = "PT1S"
      fork-name = "cancun"
      [riscv-prover.execution]
      fs-requests-directory = "/data/prover/riscv/execution/requests"
      fs-responses-directory = "/data/prover/riscv/execution/responses"
      guest-program-id = "0xdeadbeef"
      [riscv-prover.rollup]
      fs-requests-directory = "/data/prover/riscv/rollup/requests"
      fs-responses-directory = "/data/prover/riscv/rollup/responses"
      guest-program-id = "0xdeadbeef"
      [riscv-prover.proof-aggregation]
      fs-requests-directory = "/data/prover/riscv/aggregation/requests"
      fs-responses-directory = "/data/prover/riscv/aggregation/responses"
      guest-program-id = "0xdeadbeef"
      """.trimIndent()

    val config =
      ProverToml(
        fsPollingInterval = 1.seconds,
        forkName = "cancun",
        execution =
        ProverToml.ProverConfigToml(
          fsRequestsDirectory = "/data/prover/riscv/execution/requests",
          fsResponsesDirectory = "/data/prover/riscv/execution/responses",
          guestProgramId = "0xdeadbeef",
        ),
        rollup =
        ProverToml.ProverConfigToml(
          fsRequestsDirectory = "/data/prover/riscv/rollup/requests",
          fsResponsesDirectory = "/data/prover/riscv/rollup/responses",
          guestProgramId = "0xdeadbeef",
        ),
        proofAggregation =
        ProverToml.ProverConfigToml(
          fsRequestsDirectory = "/data/prover/riscv/aggregation/requests",
          fsResponsesDirectory = "/data/prover/riscv/aggregation/responses",
          guestProgramId = "0xdeadbeef",
        ),
      )

    val tomlMinimal =
      """
      [riscv-prover]
      [riscv-prover.execution]
      fs-requests-directory = "/data/prover/riscv/execution/requests"
      fs-responses-directory = "/data/prover/riscv/execution/responses"
      [riscv-prover.rollup]
      fs-requests-directory = "/data/prover/riscv/rollup/requests"
      fs-responses-directory = "/data/prover/riscv/rollup/responses"
      [riscv-prover.proof-aggregation]
      fs-requests-directory = "/data/prover/riscv/aggregation/requests"
      fs-responses-directory = "/data/prover/riscv/aggregation/responses"
      """.trimIndent()

    val configMinimal =
      ProverToml(
        execution =
        ProverToml.ProverConfigToml(
          fsRequestsDirectory = "/data/prover/riscv/execution/requests",
          fsResponsesDirectory = "/data/prover/riscv/execution/responses",
        ),
        rollup =
        ProverToml.ProverConfigToml(
          fsRequestsDirectory = "/data/prover/riscv/rollup/requests",
          fsResponsesDirectory = "/data/prover/riscv/rollup/responses",
        ),
        proofAggregation =
        ProverToml.ProverConfigToml(
          fsRequestsDirectory = "/data/prover/riscv/aggregation/requests",
          fsResponsesDirectory = "/data/prover/riscv/aggregation/responses",
        ),
      )
  }

  data class WrapperConfig(
    val riscvProver: ProverToml,
  )

  @Test
  fun `should parse riscv prover toml config`() {
    assertThat(
      parseConfig<WrapperConfig>(toml).riscvProver,
    ).isEqualTo(config)
  }

  @Test
  fun `should parse riscv prover toml config with defaults`() {
    assertThat(
      parseConfig<WrapperConfig>(tomlMinimal).riscvProver,
    ).isEqualTo(configMinimal)
  }

  @Test
  fun `should default guestProgramId to null when not specified`() {
    val parsed = parseConfig<WrapperConfig>(tomlMinimal).riscvProver
    assertThat(parsed.execution.guestProgramId).isNull()
    assertThat(parsed.rollup?.guestProgramId).isNull()
    assertThat(parsed.proofAggregation.guestProgramId).isNull()
  }
}
