package lineth.ftx.conflation

import io.vertx.core.Vertx
import linea.contract.events.FactoryForcedTransactionAddedEvent
import linea.ethapi.EthLogsSearcherImpl
import linea.ethapi.FakeEthApiClient
import linea.forcedtx.ForcedTransactionInclusionResult
import lineth.persistence.ForcedTransactionRecord
import lineth.persistence.ftx.FakeForcedTransactionsDao
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import kotlin.random.Random
import kotlin.time.Instant

class InvalidityProofAssemblerTest {
  private val contractAddress = "0x" + "aa".repeat(20)
  private lateinit var vertx: Vertx
  private lateinit var l1Client: FakeEthApiClient
  private lateinit var ftxDao: FakeForcedTransactionsDao
  private lateinit var assembler: InvalidityProofAssembler

  @BeforeEach
  fun setUp() {
    vertx = Vertx.vertx()
    l1Client = FakeEthApiClient()
    ftxDao = FakeForcedTransactionsDao()
    assembler = InvalidityProofAssembler(
      invalidityProofClient = mock(),
      stateManagerClient = mock(),
      accountProofClient = mock(),
      ethApiLogsSearcher = EthLogsSearcherImpl(vertx = vertx, ethApiClient = l1Client),
      ftxDao = ftxDao,
      tracesClient = mock(),
      contractAddress = contractAddress,
      l1EventSearchMaxBlockRange = 10_000u,
    )
  }

  @AfterEach
  fun tearDown() {
    vertx.close()
  }

  @Test
  fun `ftxNumber 1 returns a zeroed rolling hash without any lookup`() {
    assertThat(assembler.getPrevFtxRollingHash(ftxNumber = 1UL).get())
      .isEqualTo(ByteArray(32))
  }

  @Test
  fun `reads the previous ftx rolling hash from the DB without querying the chain`() {
    val prevRollingHash = Random.nextBytes(32)
    ftxDao.save(ftxRecord(ftxNumber = 11UL, rollingHash = prevRollingHash)).get()
    // No logs are seeded on-chain: had it fallen through to eth_getLogs it would have thrown.

    assertThat(assembler.getPrevFtxRollingHash(ftxNumber = 12UL).get())
      .isEqualTo(prevRollingHash)
  }

  @Test
  fun `falls back to an on-chain search when the previous ftx was pruned from the DB`() {
    val prevRollingHash = Random.nextBytes(32)
    // DB is empty (previous ftx already finalized and pruned); the event is still on-chain.
    l1Client.setLogs(
      listOf(
        FactoryForcedTransactionAddedEvent.createEthLog(
          l1BlockNumber = 5_000UL,
          contractAddress = contractAddress,
          forcedTransactionNumber = 11UL,
          forcedTransactionRollingHash = prevRollingHash,
        ),
      ),
    )
    l1Client.setLatestBlockTag(6_000UL)

    assertThat(assembler.getPrevFtxRollingHash(ftxNumber = 12UL).get())
      .isEqualTo(prevRollingHash)
  }

  @Test
  fun `throws when the previous ftx is neither in the DB nor on-chain`() {
    l1Client.setLatestBlockTag(6_000UL)

    assertThatThrownBy { assembler.getPrevFtxRollingHash(ftxNumber = 12UL).get() }
      .hasRootCauseInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining("No ForcedTransactionAdded event found for ftx=11")
  }

  private fun ftxRecord(ftxNumber: ULong, rollingHash: ByteArray): ForcedTransactionRecord =
    ForcedTransactionRecord(
      ftxNumber = ftxNumber,
      inclusionResult = ForcedTransactionInclusionResult.BadNonce,
      simulatedExecutionBlockNumber = 100UL,
      simulatedExecutionBlockTimestamp = Instant.parse("2025-04-01T00:00:00Z"),
      ftxBlockNumberDeadline = 200UL,
      ftxRollingHash = rollingHash,
      ftxRlp = Random.nextBytes(16),
      proofStatus = ForcedTransactionRecord.ProofStatus.UNREQUESTED,
    )
}
