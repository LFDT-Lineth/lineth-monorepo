package net.consensys.zkevm.coordinator.blockcreation

import build.linea.s11n.jackson.ethApiObjectMapper
import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import linea.domain.Block
import linea.domain.createBlock
import linea.domain.toEthGetBlockResponse
import linea.jsonrpc.TestingJsonRpcServer
import linea.kotlin.ByteArrayExt
import linea.kotlin.toHexString
import linea.kotlin.toULongFromHex
import linea.log4j.configureLoggers
import linea.web3j.createWeb3jHttpClient
import net.consensys.linea.async.get
import net.consensys.linea.web3j.ExtendedWeb3J
import net.consensys.linea.web3j.ExtendedWeb3JImpl
import net.consensys.zkevm.ethereum.coordination.blockcreation.BlockCreated
import net.consensys.zkevm.ethereum.coordination.blockcreation.BlockCreationListener
import org.apache.logging.log4j.Level
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.kotlin.mock
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicLong
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class BlockCreationMonitorTest {
  private lateinit var log: Logger
  private lateinit var web3jClient: ExtendedWeb3J
  private lateinit var blockCreationListener: BlockCreationListenerDouble
  private var config: BlockCreationMonitor.Config =
    BlockCreationMonitor.Config(
      pollingInterval = 100.milliseconds,
      blocksToFinalization = 2L,
      blocksFetchLimit = 500,
      lastL2BlockNumberToProcessInclusive = null
    )
  private lateinit var vertx: Vertx
  private lateinit var lastProvenBlockNumberProvider: LastProvenBlockNumberProviderDouble
  private lateinit var monitor: BlockCreationMonitor

  private lateinit var fakeL2RpcNode: TestingJsonRpcServer

  private class BlockCreationListenerDouble() : BlockCreationListener {
    val blocksReceived: MutableList<Block> = CopyOnWriteArrayList()

    override fun acceptBlock(blockEvent: BlockCreated): SafeFuture<Unit> {
      blocksReceived.add(blockEvent.block)
      return SafeFuture.completedFuture(Unit)
    }
  }

  private class LastProvenBlockNumberProviderDouble(
    initialValue: ULong
  ) : LastProvenBlockNumberProviderAsync {
    var lastProvenBlock: AtomicLong = AtomicLong(initialValue.toLong())
    override fun getLastProvenBlockNumber(): SafeFuture<Long> {
      return SafeFuture.completedFuture(lastProvenBlock.get())
    }
  }

  fun createBlockCreationMonitor(
    startingBlockNumberExclusive: Long = 99,
    blockCreationListener: BlockCreationListener = this.blockCreationListener,
    config: BlockCreationMonitor.Config = this.config
  ): BlockCreationMonitor {
    return BlockCreationMonitor(
      this.vertx,
      web3jClient,
      startingBlockNumberExclusive = startingBlockNumberExclusive,
      blockCreationListener,
      lastProvenBlockNumberProvider,
      config
    )
  }

  @BeforeEach
  fun beforeEach(vertx: Vertx) {
    configureLoggers(Level.INFO, "test.client.l2.web3j" to Level.TRACE)
    this.vertx = vertx
    log = mock()

    fakeL2RpcNode = TestingJsonRpcServer(
      vertx = vertx,
      recordRequestsResponses = true,
      responseObjectMapper = ethApiObjectMapper
    )
    blockCreationListener = BlockCreationListenerDouble()
    web3jClient = ExtendedWeb3JImpl(
      createWeb3jHttpClient(
        rpcUrl = "http://localhost:${fakeL2RpcNode.boundPort}",
        log = LogManager.getLogger("test.client.l2.web3j")
      )
    )
    lastProvenBlockNumberProvider = LastProvenBlockNumberProviderDouble(99u)
  }

  @AfterEach
  fun afterEach(vertx: Vertx) {
    monitor.stop()
    vertx.close().get()
  }

  fun createBlocks(
    startBlockNumber: ULong,
    numberOfBlocks: Int,
    startBlockHash: ByteArray = ByteArrayExt.random32(),
    startBlockParentHash: ByteArray = ByteArrayExt.random32()
  ): List<Block> {
    var blockHash = startBlockHash
    var parentHash = startBlockParentHash
    return (0..numberOfBlocks).map { i ->
      createBlock(
        number = startBlockNumber + i.toULong(),
        hash = blockHash,
        parentHash = parentHash
      ).also {
        blockHash = ByteArrayExt.random32()
        parentHash = it.hash
      }
    }
  }

  private fun setupFakeExecutionLayerWithBlocks(blocks: List<Block>) {
    fakeL2RpcNode.handle("eth_getBlockByNumber") { request ->
      val blockNumber = ((request.params as List<Any?>)[0] as String).toULongFromHex()
      blocks.find { it.number == blockNumber }?.toEthGetBlockResponse()
    }

    fakeL2RpcNode.handle("eth_blockNumber") { _ ->
      blocks.last().number.toHexString()
    }
  }

  @Test
  fun `should stop fetching blocks after lastBlockNumberInclusiveToProcess`() {
    monitor = createBlockCreationMonitor(
      startingBlockNumberExclusive = 99,
      config = config.copy(lastL2BlockNumberToProcessInclusive = 103u)
    )

    setupFakeExecutionLayerWithBlocks(createBlocks(startBlockNumber = 99u, numberOfBlocks = 20))

    monitor.start()
    await()
      .atMost(20.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(blockCreationListener.blocksReceived).isNotEmpty
        assertThat(blockCreationListener.blocksReceived.last().number).isGreaterThanOrEqualTo(103u)
      }

    // Wait for a while to make sure no more blocks are fetched
    await().atLeast(config.pollingInterval.times(3).toJavaDuration())

    assertThat(blockCreationListener.blocksReceived.last().number).isEqualTo(103UL)
  }

  @Test
  fun `should notify lister only after block is considered final on L2`() {
    monitor = createBlockCreationMonitor(
      startingBlockNumberExclusive = 99,
      config = config.copy(blocksToFinalization = 2, blocksFetchLimit = 500)
    )

    setupFakeExecutionLayerWithBlocks(createBlocks(startBlockNumber = 99u, numberOfBlocks = 200))
    fakeL2RpcNode.handle("eth_blockNumber") { _ -> 105UL.toHexString() }
    // latest eligible conflation is: 105 - 2 = 103, inclusive

    monitor.start()
    await()
      .atMost(20.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(blockCreationListener.blocksReceived).isNotEmpty
        assertThat(blockCreationListener.blocksReceived.last().number).isGreaterThanOrEqualTo(103u)
      }
    // Wait for a while to make sure no more blocks are fetched
    await().atLeast(config.pollingInterval.times(3).toJavaDuration())
    // assert that no more block were sent to the listener
    assertThat(blockCreationListener.blocksReceived.last().number).isEqualTo(103UL)

    // move chain head forward
    fakeL2RpcNode.handle("eth_blockNumber") { _ -> 120UL.toHexString() }

    // assert it resumes conflation
    await()
      .atMost(20.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(blockCreationListener.blocksReceived.last().number).isEqualTo(118UL)
      }
  }

  @Test
  fun `shall retry notify the listener when it throws and keeps block order`() {
    val fakeBuggyLister = object : BlockCreationListener {
      var errorCount = 0
      override fun acceptBlock(blockEvent: BlockCreated): SafeFuture<Unit> {
        return if (blockEvent.block.number == 105UL && errorCount < 3) {
          errorCount++
          throw RuntimeException("Error on block 105")
        } else {
          blockCreationListener.acceptBlock(blockEvent)
        }
      }
    }

    monitor = createBlockCreationMonitor(
      startingBlockNumberExclusive = 99,
      blockCreationListener = fakeBuggyLister,
      config = config.copy(blocksToFinalization = 2, lastL2BlockNumberToProcessInclusive = 112u)
    )

    setupFakeExecutionLayerWithBlocks(createBlocks(startBlockNumber = 99u, numberOfBlocks = 20))

    monitor.start()
    await()
      .atMost(20.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(blockCreationListener.blocksReceived).isNotEmpty
        assertThat(blockCreationListener.blocksReceived.last().number).isGreaterThanOrEqualTo(110u)
      }

    // assert it got block only once and in order
    assertThat(blockCreationListener.blocksReceived.map { it.number }).containsExactly(
      100UL, 101UL, 102UL, 103UL, 104UL, 105UL, 106UL, 107UL, 108UL, 109UL, 110UL
    )
  }

  @Test
  fun `should be resilient to connection failures`() {
    monitor = createBlockCreationMonitor(
      startingBlockNumberExclusive = 99
    )

    setupFakeExecutionLayerWithBlocks(createBlocks(startBlockNumber = 99u, numberOfBlocks = 200))

    monitor.start()
    await()
      .atMost(20.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(blockCreationListener.blocksReceived).isNotEmpty
        assertThat(blockCreationListener.blocksReceived.last().number).isGreaterThanOrEqualTo(103u)
      }
    fakeL2RpcNode.stopHttpServer().get()
    val lastBlockReceived = blockCreationListener.blocksReceived.last().number

    // Wait for a while to make sure no more blocks are fetched
    await().atLeast(config.pollingInterval.times(2).toJavaDuration())
    fakeL2RpcNode.resumeHttpServer().get()
    await()
      .atMost(40.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(blockCreationListener.blocksReceived).isNotEmpty
        assertThat(blockCreationListener.blocksReceived.last().number).isGreaterThan(lastBlockReceived)
      }
  }

  @Test
  fun `should stop when reorg is detected above blocksToFinalization limit - manual intervention necessary`() {
    monitor = createBlockCreationMonitor(
      startingBlockNumberExclusive = 99
    )

    // simulate reorg by changing parent hash of block 105
    val blocks = createBlocks(startBlockNumber = 99u, numberOfBlocks = 20).map { block: Block ->
      if (block.number == 105UL) {
        block.copy(parentHash = ByteArrayExt.random32())
      } else {
        block
      }
    }

    setupFakeExecutionLayerWithBlocks(blocks)

    monitor.start()
    await()
      .atMost(20.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(blockCreationListener.blocksReceived).isNotEmpty
        assertThat(blockCreationListener.blocksReceived.last().number).isGreaterThanOrEqualTo(104UL)
      }

    // Wait for a while to make sure no more blocks are fetched
    await().atLeast(config.pollingInterval.times(3).toJavaDuration())

    assertThat(blockCreationListener.blocksReceived.last().number).isEqualTo(104UL)
  }

  @Test
  fun `should poll in order when response takes longer that polling interval`() {
    monitor = createBlockCreationMonitor(
      startingBlockNumberExclusive = 99,
      config = config.copy(pollingInterval = 100.milliseconds)
    )

    val blocks = createBlocks(startBlockNumber = 99u, numberOfBlocks = 20)
    setupFakeExecutionLayerWithBlocks(blocks)
    fakeL2RpcNode.responsesArtificialDelay = 600.milliseconds

    monitor.start()
    await()
      .atMost(20.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(blockCreationListener.blocksReceived).isNotEmpty
        assertThat(blockCreationListener.blocksReceived.map { it.number }).containsExactly(
          100UL,
          101UL,
          102UL,
          103UL,
          104UL,
          105UL
        )
      }
  }

  @Test
  fun `start allow 2nd call when already started`() {
    monitor = createBlockCreationMonitor(
      startingBlockNumberExclusive = 99
    )
    setupFakeExecutionLayerWithBlocks(createBlocks(startBlockNumber = 99u, numberOfBlocks = 5))
    monitor.start().get()
    monitor.start().get()
  }

  @Test
  fun `should stop fetching blocks when gap is greater than fetch limit and resume upon catchup`() {
    monitor = createBlockCreationMonitor(
      startingBlockNumberExclusive = 99,
      config = config.copy(blocksToFinalization = 0, blocksFetchLimit = 5)
    )

    setupFakeExecutionLayerWithBlocks(createBlocks(startBlockNumber = 99u, numberOfBlocks = 30))
    lastProvenBlockNumberProvider.lastProvenBlock.set(105)

    monitor.start()
    await()
      .atMost(20.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(blockCreationListener.blocksReceived).isNotEmpty
        assertThat(blockCreationListener.blocksReceived.last().number).isGreaterThanOrEqualTo(110UL)
      }

    // Wait for a while to make sure no more blocks are fetched
    await().atLeast(config.pollingInterval.times(3).toJavaDuration())

    // it shall remain at 110
    assertThat(blockCreationListener.blocksReceived.last().number).isEqualTo(110UL)

    // simulate prover catchup
    lastProvenBlockNumberProvider.lastProvenBlock.set(120)

    // assert it resumes conflation
    await()
      .atMost(20.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(blockCreationListener.blocksReceived.last().number).isGreaterThanOrEqualTo(125UL)
      }
  }
}
