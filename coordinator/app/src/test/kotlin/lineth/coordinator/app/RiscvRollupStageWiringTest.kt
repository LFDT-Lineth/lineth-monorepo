package lineth.coordinator.app

import io.vertx.core.Vertx
import linea.domain.BlockParameter
import linea.domain.createBlock
import linea.domain.toBlockWithRandomTxHashes
import linea.ethapi.EthApiClient
import lineth.conflation.calculators.BlobTriggerCalculatorByTargetBlockNumbers
import lineth.conflation.calculators.RollupTriggerCalculatorByConflationCount
import lineth.conflation.calculators.RollupTriggerCalculatorByDataLimit
import lineth.conflation.calculators.RollupTriggerCalculatorByGas
import lineth.conflation.calculators.RollupTriggerCalculatorByTargetBlockNumbers
import lineth.conflation.calculators.RollupTriggerCalculatorByTxCount
import lineth.coordinator.app.conflation.ConfigRiscvGenesisDataRollingHashProvider
import lineth.coordinator.app.conflation.ConflationAppHelper.awaitRiscvCutoverFinalized
import lineth.coordinator.app.conflation.ConflationAppHelper.isRiscvCutoverFinalized
import lineth.coordinator.app.conflation.ConflationAppHelper.rollupBlobTriggers
import lineth.coordinator.app.conflation.ConflationAppHelper.rollupTriggers
import lineth.coordinator.app.conflation.ConflationAppHelper.validateRiscvRollupTransition
import lineth.coordinator.app.conflation.EthApiL2BlocksFetcher
import lineth.coordinator.config.v2.ConflationConfig
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito.mock
import org.mockito.kotlin.any
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicLong
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Instant

class RiscvRollupStageWiringTest {
  private val genesisTime = Instant.parse("2026-10-01T00:00:00Z")
  private val cutoverBlock = 100UL
  private val cutover = timestampOf(cutoverBlock)
  private val vertx = Vertx.vertx()

  @AfterEach
  fun tearDown() {
    vertx.close().toCompletionStage().toCompletableFuture().get()
  }

  private fun timestampOf(blockNumber: ULong): Instant = Instant.fromEpochSeconds(
    genesisTime.epochSeconds + blockNumber.toLong(),
  )

  /** An L2 chain whose block n has timestamp genesis + n seconds, up to [headBlock]. */
  private fun l2Chain(headBlock: ULong): EthApiClient {
    val l2EthClient = mock<EthApiClient>()
    fun block(number: ULong) = createBlock(number = number, timestamp = timestampOf(number))
    whenever(l2EthClient.ethFindBlockByNumberTxHashes(any())).thenAnswer {
      val number = (it.arguments[0] as BlockParameter.BlockNumber).number
      SafeFuture.completedFuture(if (number <= headBlock) block(number).toBlockWithRandomTxHashes() else null)
    }
    whenever(l2EthClient.ethGetBlockByNumberTxHashes(any())).thenAnswer {
      val number = (it.arguments[0] as BlockParameter.BlockNumber).number
      SafeFuture.completedFuture(block(number).toBlockWithRandomTxHashes())
    }
    whenever(l2EthClient.ethGetBlockByNumberFullTxs(any())).thenAnswer {
      SafeFuture.completedFuture(block((it.arguments[0] as BlockParameter.BlockNumber).number))
    }
    return l2EthClient
  }

  private fun cutoverFinalized(lastFinalizedBlock: ULong, l2EthClient: EthApiClient = l2Chain(200UL)): Boolean =
    isRiscvCutoverFinalized(lastFinalizedBlock, timestampOf(lastFinalizedBlock), cutover, l2EthClient).get()

  @Test
  fun `the cutover is finalized from the last pre-RISC-V block on`() {
    assertThat(cutoverFinalized(cutoverBlock - 2UL)).isFalse()
    assertThat(cutoverFinalized(cutoverBlock - 1UL)).`as`("last pre-RISC-V block").isTrue()
    assertThat(cutoverFinalized(cutoverBlock)).isTrue()
    assertThat(cutoverFinalized(cutoverBlock + 50UL)).isTrue()
  }

  @Test
  fun `the cutover is not finalized while the cutover block does not exist yet`() {
    assertThat(cutoverFinalized(cutoverBlock - 1UL, l2Chain(headBlock = cutoverBlock - 1UL))).isFalse()
  }

  @Test
  fun `waiting for the cutover completes once L1 finalizes the last pre-RISC-V block`() {
    val latestFinalized = AtomicLong(50)
    val finalized = awaitRiscvCutoverFinalized(
      vertx = vertx,
      pollingInterval = 10.milliseconds,
      riscvCutoverTimestamp = cutover,
      latestL1FinalizedBlock = { latestFinalized.get().toULong() },
      l2EthClient = l2Chain(200UL),
    )

    Thread.sleep(100)
    assertThat(finalized).isNotDone()

    latestFinalized.set((cutoverBlock - 1UL).toLong())
    assertThat(finalized.get(5, TimeUnit.SECONDS).number).isEqualTo(cutoverBlock - 1UL)
  }

  @Test
  fun `a rollup stage requires the cutover config and, before the cutover, the L1 finalization wait`() {
    validateRiscvRollupTransition(
      rollupConfigured = false,
      riscvCutoverConfigured = false,
      riscvCutoverCrossed = false,
      waitTargetBlockL1Finalization = false,
    )
    validateRiscvRollupTransition(
      rollupConfigured = true,
      riscvCutoverConfigured = true,
      riscvCutoverCrossed = true,
      waitTargetBlockL1Finalization = false,
    )
    validateRiscvRollupTransition(
      rollupConfigured = true,
      riscvCutoverConfigured = true,
      riscvCutoverCrossed = false,
      waitTargetBlockL1Finalization = true,
    )

    assertThatThrownBy {
      validateRiscvRollupTransition(
        rollupConfigured = true,
        riscvCutoverConfigured = true,
        riscvCutoverCrossed = false,
        waitTargetBlockL1Finalization = false,
      )
    }.hasMessageContaining("wait-target-block-l1-finalization=true")
    assertThatThrownBy {
      validateRiscvRollupTransition(
        rollupConfigured = true,
        riscvCutoverConfigured = false,
        riscvCutoverCrossed = false,
        waitTargetBlockL1Finalization = true,
      )
    }.hasMessageContaining("riscv-starting-block-timestamp-inclusive")
  }

  @Test
  fun `the configured limits become rollup and blob triggers`() {
    val full = ConflationConfig.Rollup(
      genesisDataRollingHash = ByteArray(32),
      blobTargetEndBlocks = setOf(10UL),
      conflationsLimit = 5u,
      targetEndBlocks = setOf(20UL),
      dataLimit = 1000UL,
      gasLimit = 2000UL,
      txCountLimit = 3000UL,
    )

    assertThat(
      rollupBlobTriggers(full),
    ).hasExactlyElementsOfTypes(BlobTriggerCalculatorByTargetBlockNumbers::class.java)
    assertThat(rollupTriggers(full)).hasExactlyElementsOfTypes(
      RollupTriggerCalculatorByConflationCount::class.java,
      RollupTriggerCalculatorByTargetBlockNumbers::class.java,
      RollupTriggerCalculatorByDataLimit::class.java,
      RollupTriggerCalculatorByGas::class.java,
      RollupTriggerCalculatorByTxCount::class.java,
    )

    val minimal = ConflationConfig.Rollup(genesisDataRollingHash = ByteArray(32))
    assertThat(rollupBlobTriggers(minimal)).isEmpty()
    assertThat(rollupTriggers(minimal)).isEmpty()
  }

  @Test
  fun `blocks are fetched in order with full transactions`() {
    val blocks = EthApiL2BlocksFetcher(l2Chain(200UL)).fetchBlocks(5UL, 8UL).get()

    assertThat(blocks.map { it.number }).containsExactly(5UL, 6UL, 7UL, 8UL)
    assertThat(blocks.map { it.timestamp }).containsExactly(
      *(5UL..8UL).map { timestampOf(it).epochSeconds.toULong() }.toTypedArray(),
    )
  }

  @Test
  fun `the genesis data rolling hash comes from config`() {
    val hash = ByteArray(32) { 0x22 }

    assertThat(ConfigRiscvGenesisDataRollingHashProvider(hash).getGenesisDataRollingHash().get()).isEqualTo(hash)
    assertThatThrownBy { ConfigRiscvGenesisDataRollingHashProvider(ByteArray(31)) }
      .hasMessageContaining("must be 32 bytes")
  }
}
