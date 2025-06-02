package net.consensys.zkevm.coordinator.app

import io.vertx.core.Vertx
import linea.contract.l1.LineaRollupSmartContractClientReadOnly
import linea.domain.BlockParameter
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration.Companion.milliseconds

class L1BasedLastFinalizedBlockProviderTest {
  private lateinit var lineaRollupClient: LineaRollupSmartContractClientReadOnly

  @BeforeEach
  fun beforeEach() {
    lineaRollupClient =
      mock<LineaRollupSmartContractClientReadOnly>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
  }

  @Test
  fun `shall wait number of blocks before returning for consistency`() {
    val replies = listOf(100UL, 100UL, 101UL, 101UL, 101UL, 101UL)
    whenever(lineaRollupClient.finalizedL2BlockNumber(eq(BlockParameter.Tag.LATEST)))
      .thenReturn(
        SafeFuture.completedFuture(replies[0]),
        *replies.subList(1, replies.size).map { SafeFuture.completedFuture(it) }.toTypedArray(),
      )

    val resumerCalculator = L1BasedLastFinalizedBlockProvider(
      Vertx.vertx(),
      lineaRollupClient,
      consistentNumberOfBlocksOnL1 = 3u,
      numberOfRetries = 50u,
      pollingInterval = 10.milliseconds,
    )

    assertThat(resumerCalculator.getLastFinalizedBlock().get()).isEqualTo(101.toULong())
  }
}
