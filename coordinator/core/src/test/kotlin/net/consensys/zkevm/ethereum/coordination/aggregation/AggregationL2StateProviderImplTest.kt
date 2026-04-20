package net.consensys.zkevm.ethereum.coordination.aggregation

import linea.contract.l2.FakeL2MessageService
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.ethapi.FakeEthApiClient
import linea.persistence.ftx.FakeForcedTransactionsDao
import linea.persistence.ftx.ForcedTransactionRecordFactory
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import kotlin.random.Random
import kotlin.time.Instant

class AggregationL2StateProviderImplTest {
  private lateinit var ethApiClient: FakeEthApiClient
  private lateinit var messageService: FakeL2MessageService
  private lateinit var forcedTransactionsDao: FakeForcedTransactionsDao
  private lateinit var provider: AggregationL2StateProviderImpl

  @BeforeEach
  fun setUp() {
    ethApiClient = FakeEthApiClient()
    // deploy block = 1 so block 0 is before deployment
    messageService = FakeL2MessageService(contractDeployBlock = 1uL)
    forcedTransactionsDao = FakeForcedTransactionsDao()
    provider = AggregationL2StateProviderImpl(ethApiClient, messageService, forcedTransactionsDao)
  }

  @Test
  fun `returns genesis anchored message and genesis ftx state for block 0`() {
    val state = provider.getAggregationL2State(0L).get()

    assertThat(state.parentAggregationLastL1RollingHashMessageNumber).isEqualTo(0uL)
    assertThat(state.parentAggregationLastL1RollingHash).isEqualTo(AggregationL2StateProviderImpl.GENESIS_ZERO_HASH)
    assertThat(state.parentAggregationLastFtxNumber).isEqualTo(0uL)
    assertThat(state.parentAggregationLastFtxRollingHash).isEqualTo(AggregationL2StateProviderImpl.GENESIS_ZERO_HASH)
  }

  @Test
  fun `returns genesis anchored message when block is before contract deployment`() {
    val blockNumber = 0L // contractDeployBlock = 1, so block 0 is before deployment
    forcedTransactionsDao.save(
      ForcedTransactionRecordFactory.createForcedTransactionRecord(
        ftxNumber = 1uL,
        simulatedExecutionBlockNumber = blockNumber.toULong(),
      ),
    ).get()

    val state = provider.getAggregationL2State(blockNumber).get()

    assertThat(state.parentAggregationLastL1RollingHashMessageNumber).isEqualTo(0uL)
    assertThat(state.parentAggregationLastL1RollingHash).isEqualTo(AggregationL2StateProviderImpl.GENESIS_ZERO_HASH)
  }

  @Test
  fun `returns anchored message info for block at or after contract deployment`() {
    val blockNumber = 10L
    val messageNumber = 3uL
    val rollingHash = Random.nextBytes(32)
    ethApiClient.setLatestBlockTag(blockNumber.toULong())
    messageService.setLastAnchoredL1Message(messageNumber, rollingHash)

    val state = provider.getAggregationL2State(blockNumber).get()

    assertThat(state.parentAggregationLastL1RollingHashMessageNumber).isEqualTo(messageNumber)
    assertThat(state.parentAggregationLastL1RollingHash).isEqualTo(rollingHash)
  }

  @Test
  fun `returns genesis ftx state when no forced transactions exist`() {
    val blockNumber = 10L
    ethApiClient.setLatestBlockTag(blockNumber.toULong())

    val state = provider.getAggregationL2State(blockNumber).get()

    assertThat(state.parentAggregationLastFtxNumber).isEqualTo(0uL)
    assertThat(state.parentAggregationLastFtxRollingHash).isEqualTo(AggregationL2StateProviderImpl.GENESIS_ZERO_HASH)
  }

  @Test
  fun `returns highest forced transaction at or below aggEndBlockNumber`() {
    val blockNumber = 10L
    val ftxRollingHash = Random.nextBytes(32)
    ethApiClient.setLatestBlockTag(blockNumber.toULong())

    forcedTransactionsDao.save(
      ForcedTransactionRecordFactory.createForcedTransactionRecord(
        ftxNumber = 1uL,
        simulatedExecutionBlockNumber = 5uL,
      ),
    ).get()
    forcedTransactionsDao.save(
      ForcedTransactionRecordFactory.createForcedTransactionRecord(
        ftxNumber = 2uL,
        simulatedExecutionBlockNumber = 10uL,
        ftxRollingHash = ftxRollingHash,
      ),
    ).get()
    // This ftx is above blockNumber and must be excluded
    forcedTransactionsDao.save(
      ForcedTransactionRecordFactory.createForcedTransactionRecord(
        ftxNumber = 3uL,
        simulatedExecutionBlockNumber = 15uL,
      ),
    ).get()

    val state = provider.getAggregationL2State(blockNumber).get()

    assertThat(state.parentAggregationLastFtxNumber).isEqualTo(2uL)
    assertThat(state.parentAggregationLastFtxRollingHash).isEqualTo(ftxRollingHash)
  }

  @Test
  fun `returns genesis ftx state when all forced transactions are above blockNumber`() {
    val blockNumber = 5L
    ethApiClient.setLatestBlockTag(blockNumber.toULong())
    forcedTransactionsDao.save(
      ForcedTransactionRecordFactory.createForcedTransactionRecord(
        ftxNumber = 1uL,
        simulatedExecutionBlockNumber = 10uL,
      ),
    ).get()

    val state = provider.getAggregationL2State(blockNumber).get()

    assertThat(state.parentAggregationLastFtxNumber).isEqualTo(0uL)
    assertThat(state.parentAggregationLastFtxRollingHash).isEqualTo(AggregationL2StateProviderImpl.GENESIS_ZERO_HASH)
  }

  @Test
  fun `block timestamp is derived from the eth block at the given block number`() {
    val blockNumber = 5L
    ethApiClient.setLatestBlockTag(blockNumber.toULong())

    val state = provider.getAggregationL2State(blockNumber).get()
    val block = ethApiClient.ethFindBlockByNumberFullTxs(blockNumber.toULong().toBlockParameter()).get()!!

    assertThat(state.parentAggregationLastBlockTimestamp)
      .isEqualTo(Instant.fromEpochSeconds(block.timestamp.toLong()))
  }
}
