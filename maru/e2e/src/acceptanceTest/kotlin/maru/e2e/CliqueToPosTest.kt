/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.e2e

import com.fasterxml.jackson.databind.ObjectMapper
import com.palantir.docker.compose.DockerComposeRule
import com.palantir.docker.compose.configuration.DockerComposeFiles
import com.palantir.docker.compose.configuration.ProjectName
import com.palantir.docker.compose.connection.waiting.HealthChecks
import linea.kotlin.toULong
import maru.app.MaruApp
import maru.config.ApiEndpointConfig
import maru.config.FollowersConfig
import maru.core.EMPTY_HASH
import maru.extensions.encodeHex
import maru.extensions.fromHexToByteArray
import net.consensys.linea.async.toSafeFuture
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.kotlin.await
import org.junit.jupiter.api.AfterAll
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeAll
import org.junit.jupiter.api.Disabled
import org.junit.jupiter.api.Order
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.methods.response.EthBlock
import tech.pegasys.teku.infrastructure.async.SafeFuture
import testutils.Web3jTransactionsHelper
import testutils.maru.MaruFactory
import testutils.maru.awaitTillMaruHasPeers
import java.io.File
import java.math.BigInteger
import java.net.URI
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.StandardCopyOption
import java.util.UUID
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.minutes
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@Disabled("flaky and we don't need it anymore")
class CliqueToPosTest {
  companion object {
    private val dockerComposeFilePaths =
      mutableListOf<String>(Path.of("./../docker/compose.yaml").toString())
    private val qbftCluster =
      DockerComposeRule
        .Builder()
        .pullOnStartup(true)
        .files(DockerComposeFiles.from(*dockerComposeFilePaths.toTypedArray()))
        .projectName(ProjectName.fromString("maru-e2e-" + UUID.randomUUID().toString().take(8)))
        .waitingForService("sequencer", HealthChecks.toHaveAllPortsOpen())
        .build()
    private var forksTimestamps = emptyMap<String, ULong>()
    private var shanghaiTimestamp: ULong = 0UL
    private var cancunTimestamp: ULong = 0UL
    private var pragueTimestamp: ULong = 0UL
    private var ttd: ULong = 0UL
    private lateinit var maruFactory: MaruFactory

    @TempDir
    private lateinit var sequencerMaruTmpDir: File

    private val genesisDir = File("../docker/initialization")
    private val transactionsHelper = Web3jTransactionsHelper(TestEnvironment.sequencerL2Client)
    private val log: Logger = LogManager.getLogger(CliqueToPosTest::class.java)
    private const val VALIDATOR_PRIVATE_KEY_WITH_PREFIX =
      "0x080212201dd171cec7e2995408b5513004e8207fe88d6820aeff0d82463b3e41df251aae"

    // Instantiated in the networkCanBeSwitched test, used in the following tests
    private lateinit var maruSequencer: MaruApp

    // Will only be used in the sync from scratch tests
    private var maruFollower: MaruApp? = null

    private fun parseForks(forks: List<String>): Map<String, ULong> {
      val objectMapper = ObjectMapper()
      val genesisTree = objectMapper.readTree(File(genesisDir, "genesis-besu.json"))
      return forks
        .map { forkId ->
          val switchTime = genesisTree.at("/config/$forkId").asLong()
          forkId to switchTime.toULong()
        }.associate { it }
    }

    private fun deleteGenesisFiles() {
      val maruGenesis = File(genesisDir, "genesis-maru.json")
      val besuGenesis = File(genesisDir, "genesis-besu.json")
      val gethGenesis = File(genesisDir, "genesis-geth.json")
      val nethermindGenesis = File(genesisDir, "genesis-nethermind.json")
      maruGenesis.delete()
      besuGenesis.delete()
      gethGenesis.delete()
      nethermindGenesis.delete()
    }

    @BeforeAll
    @JvmStatic
    fun beforeAll() {
      deleteGenesisFiles()
      qbftCluster.before()
      forksTimestamps = parseForks(listOf("pragueTime", "cancunTime", "shanghaiTime", "terminalTotalDifficulty"))
      shanghaiTimestamp = forksTimestamps["shanghaiTime"]!!
      cancunTimestamp = forksTimestamps["cancunTime"]!!
      pragueTimestamp = forksTimestamps["pragueTime"]!!
      ttd = forksTimestamps["terminalTotalDifficulty"]!!
      maruFactory = MaruFactory(
        validatorPrivateKey = VALIDATOR_PRIVATE_KEY_WITH_PREFIX.fromHexToByteArray(),
        shanghaiTimestamp = shanghaiTimestamp,
        cancunTimestamp = cancunTimestamp,
        pragueTimestamp = pragueTimestamp,
        ttd = ttd,
      )
    }

    @AfterAll
    @JvmStatic
    fun afterAll() {
      if (::maruSequencer.isInitialized) {
        maruSequencer.stop().get()
        maruSequencer.close()
      }
      qbftCluster.after()

      TestEnvironment.allClients.values.forEach { web3j ->
        web3j.shutdown()
      }
      log.info("Test is complete")
    }

    @JvmStatic
    fun followerNodes(): List<Arguments> =
      TestEnvironment.followerExecutionClientsPostMerge.map {
        Arguments.of(it.key, it.value)
      }

    private fun getBlockByNumber(
      blockNumber: Long,
      retreiveTransactions: Boolean = false,
      ethClient: Web3j = TestEnvironment.sequencerL2Client,
    ): EthBlock.Block? =
      ethClient
        .ethGetBlockByNumber(
          DefaultBlockParameter.valueOf(BigInteger.valueOf(blockNumber)),
          retreiveTransactions,
        ).send()
        .block

    private fun buildValidatorMaruWithMultipleFollowers(): MaruApp =
      maruFactory.buildSwitchableTestMaruValidatorWithP2pPeering(
        ethereumJsonRpcUrl = "http://localhost:8545",
        engineApiRpc = "http://localhost:8550",
        dataDir = sequencerMaruTmpDir.toPath(),
        followers = FollowersConfig(
          mapOf(
            "follower-besu" to ApiEndpointConfig(URI.create("http://localhost:9550").toURL()),
            "follower-erigon" to
              ApiEndpointConfig(
                URI.create("http://localhost:11551").toURL(),
                jwtSecretPath = TestEnvironment.JWT_CONFIG_PATH,
              ),
            "follower-nethermind" to
              ApiEndpointConfig(
                URI.create("http://localhost:10550").toURL(),
                jwtSecretPath = TestEnvironment.JWT_CONFIG_PATH,
              ),
            "follower-geth" to
              ApiEndpointConfig(
                URI.create("http://localhost:8561").toURL(),
                jwtSecretPath = TestEnvironment.JWT_CONFIG_PATH,
              ),
          ),
        ),
      )

    private var runCounter = 0u
    private const val LAST_CLIQUE_BLOCK_NUMBER = 5L
  }

  private fun saveLogs(path: Path) {
    qbftCluster.dockerExecutable().execute("ps").inputStream.use {
      Files.copy(
        it,
        Path.of("$path/containers.txt"),
        StandardCopyOption.REPLACE_EXISTING,
      )
    }
    val containerShortNames =
      TestEnvironment.allClients
        .map { it.key }
        .toMutableList()
    containerShortNames.forEach { containerShortName ->
      qbftCluster
        .dockerExecutable()
        .execute("logs", containerShortName)
        .inputStream
        .use {
          Files.copy(
            it,
            Path.of("$path/$containerShortName.log"),
            StandardCopyOption.REPLACE_EXISTING,
          )
        }
    }
  }

  @AfterEach
  fun tearDown() {
    if (maruFollower != null) {
      maruFollower!!.stop()
      maruFollower!!.close()
    }

    val logsDestination = Path.of("docker_logs/run_$runCounter")
    logsDestination.toFile().mkdirs()
    saveLogs(logsDestination)
    runCounter += 1u
  }

  @Order(1)
  @Test
  fun networkCanBeSwitched() {
    maruSequencer = buildValidatorMaruWithMultipleFollowers()
    maruSequencer.start()
    val preCancunTransactions = 10
    sendCliqueAndParisTransactions(preCancunTransactions)
    everyoneArePeered()
    val lastCliqueBlock = getBlockByNumber(LAST_CLIQUE_BLOCK_NUMBER)!!
    assertThat(lastCliqueBlock.totalDifficulty.toLong()).isEqualTo(LAST_CLIQUE_BLOCK_NUMBER * 2L + 1L)
    val parisBlock = getBlockByNumber(LAST_CLIQUE_BLOCK_NUMBER + 1)!!
    assertThat(parisBlock.difficulty.toLong()).isEqualTo(0L)

    val shanghaiTransactions = 4
    waitTillTimestamp(shanghaiTimestamp, "shanghaiTime")
    log.info("Sequencer has switched to Shanghai")
    repeat(shanghaiTransactions) {
      transactionsHelper.run { sendArbitraryTransaction().waitForInclusion() }
    }

    val cancunTransactions = 4
    waitTillTimestamp(cancunTimestamp, "cancunTime")
    log.info("Sequencer has switched to Cancun")
    repeat(cancunTransactions) {
      transactionsHelper.run { sendArbitraryTransaction().waitForInclusion() }
    }

    val pragueTransactions = 4
    waitTillTimestamp(pragueTimestamp, "pragueTime")
    log.info("Sequencer has switched to Prague")
    repeat(pragueTransactions) {
      transactionsHelper.run { sendArbitraryTransaction().waitForInclusion() }
    }

    val firstShanghaiBlock = getBlockByNumber(preCancunTransactions.toLong() + 1)!!
    assertThat(firstShanghaiBlock.timestamp.toULong())
      .isGreaterThanOrEqualTo(shanghaiTimestamp)
      .isLessThan(cancunTimestamp)

    val firstCancunBlock = getBlockByNumber(preCancunTransactions.toLong() + shanghaiTransactions + 1)!!
    assertThat(firstCancunBlock.timestamp.toULong())
      .isGreaterThanOrEqualTo(cancunTimestamp)
      .isLessThan(pragueTimestamp)

    val firstPragueBlock =
      getBlockByNumber(preCancunTransactions.toLong() + shanghaiTransactions + cancunTransactions + 1)!!
    assertThat(firstPragueBlock.timestamp.toULong()).isGreaterThanOrEqualTo(pragueTimestamp)

    val resultingBlockNumber = preCancunTransactions + shanghaiTransactions + cancunTransactions + pragueTransactions
    assertNodeBlockHeight(TestEnvironment.sequencerL2Client, resultingBlockNumber.toLong())

    waitForAllNodesToBeInSyncToMatch()
  }

  // TODO: Explore parallelization of this test
  @Order(2)
  @ParameterizedTest
  @MethodSource("followerNodes")
  fun syncFromScratch(
    nodeName: String,
    engineApiConfig: ApiEndpointConfig,
  ) {
    val nodeEthereumClient = TestEnvironment.followerClientsPostMerge[nodeName]!!
    restartNodeFromScratch(nodeName, nodeEthereumClient)
    log.info("Container $nodeName restarted")

    val awaitCondition =
      if (nodeName.contains("nethermind")) {
        await
          .pollInterval(10.seconds.toJavaDuration())
          .timeout(60.seconds.toJavaDuration())
      } else {
        await
          .pollInterval(1.seconds.toJavaDuration())
          .timeout(30.seconds.toJavaDuration())
      }

    val followerDataDir = Files.createTempDirectory("maru-$nodeName")
    followerDataDir.toFile().deleteOnExit()
    maruFollower = maruFactory.buildTestMaruFollowerWithConsensusSwitch(
      engineApiConfig = engineApiConfig,
      ethereumApiConfig = engineApiConfig,
      dataDir = followerDataDir,
      validatorPortForStaticPeering = maruSequencer.p2pPort(),
      desyncTolerance = 0UL,
    )

    maruFollower!!.start()

    maruFollower!!.awaitTillMaruHasPeers(1u)
    maruSequencer.awaitTillMaruHasPeers(1u)
    awaitCondition
      .timeout(1.minutes.toJavaDuration())
      .pollInterval(15.seconds.toJavaDuration())
      .ignoreExceptions()
      .alias(nodeName)
      .untilAsserted {
        if (nodeName.contains("erigon") || nodeName.contains("nethermind")) {
          // For some reason Erigon and Nethermind need a restart after PoS transition
          restartNodeKeepingState(nodeName, nodeEthereumClient)
        }

        assertNodeBlockHeight(nodeEthereumClient)
        assertNodeBlockPrevRandao(nodeEthereumClient)
      }
  }

  private fun waitTillTimestamp(
    timestamp: ULong,
    timestampFork: String,
  ) {
    val unixTimestamp = currentTimestamp()
    log.info("Waiting ${timestamp - unixTimestamp} seconds for the $timestampFork at timestamp $timestamp")
    await
      .timeout(2.minutes.toJavaDuration())
      .pollInterval(500.milliseconds.toJavaDuration())
      .untilAsserted {
        val unixTimestamp = currentTimestamp()
        log.debug("Waiting ${timestamp - unixTimestamp} seconds for the $timestampFork at timestamp $timestamp")
        assertThat(unixTimestamp).isGreaterThanOrEqualTo(timestamp)
      }
  }

  private fun currentTimestamp(): ULong = (System.currentTimeMillis() / 1000).toULong()

  private fun restartNodeFromScratch(
    nodeName: String,
    nodeEthereumClient: Web3j,
  ) {
    qbftCluster.docker().rm(nodeName)
    qbftCluster.dockerCompose().up()
    awaitExpectedBlockNumberAfterStartup(nodeName, nodeEthereumClient)
  }

  private fun restartNodeKeepingState(
    nodeName: String,
    nodeEthereumClient: Web3j,
  ) {
    log.debug("Restarting $nodeName keeping state")
    val container = qbftCluster.containers().container(nodeName)
    container.stop()
    container.start()
    awaitExpectedBlockNumberAfterStartup(nodeName, nodeEthereumClient)
  }

  private fun awaitExpectedBlockNumberAfterStartup(
    nodeName: String,
    ethereumClient: Web3j,
  ) {
    val expectedBlockNumber =
      when {
        nodeName.contains("erigon") -> 5L
        nodeName.contains("nethermind") -> 5L
        else -> 0L
      }
    await
      .pollInterval(1.seconds.toJavaDuration())
      .timeout(30.seconds.toJavaDuration())
      .ignoreExceptions()
      .alias(nodeName)
      .untilAsserted {
        log.debug("Waiting till the node $nodeName is up and synced")
        assertThat(
          ethereumClient
            .ethBlockNumber()
            .send()
            .blockNumber
            .toLong(),
        ).isGreaterThanOrEqualTo(expectedBlockNumber)
          .withFailMessage("Node is unexpectedly synced after restart! Was its state flushed?")
      }
  }

  private fun sendCliqueAndParisTransactions(cliqueAndParisTransactions: Int) {
    val sequencerBlock = TestEnvironment.sequencerL2Client.ethBlockNumber().send()
    if (sequencerBlock.blockNumber >= BigInteger.valueOf(cliqueAndParisTransactions.toLong())) {
      return
    }
    repeat(cliqueAndParisTransactions) { transactionsHelper.run { sendArbitraryTransaction().waitForInclusion() } }
  }

  private fun assertNodeBlockHeight(
    web3j: Web3j,
    expectedBlockNumber: Long =
      TestEnvironment.sequencerL2Client
        .ethBlockNumber()
        .send()
        .blockNumber
        .toLong(),
  ) {
    val targetNodeBlockHeight = web3j.ethBlockNumber().send().blockNumber
    assertThat(targetNodeBlockHeight).isEqualTo(expectedBlockNumber)
  }

  private fun assertNodeBlockPrevRandao(
    web3j: Web3j,
    lastPreMergeBlockNumber: Long = 5L,
  ) {
    val lastPostMergeBlockNumber =
      TestEnvironment.sequencerL2Client
        .ethBlockNumber()
        .send()
        .blockNumber
        .toLong()
    var lastMixHash: String? = null
    (lastPreMergeBlockNumber..lastPostMergeBlockNumber).forEach {
      val mixHash =
        web3j
          .ethGetBlockByNumber(
            DefaultBlockParameter.valueOf(it.toBigInteger()),
            false,
          ).send()
          .block.mixHash
      if (it == lastPreMergeBlockNumber) {
        assertThat(mixHash).isEqualTo(EMPTY_HASH.encodeHex())
      } else {
        assertThat(mixHash).isNotEqualTo(lastMixHash)
      }
      lastMixHash = mixHash
    }
  }

  private fun waitForAllNodesToBeInSyncToMatch() {
    // Send a transaction so that the Besu follower triggers a backward sync to sync to head.
    // Besu doesn't adjust the pivot block during the initial sync and may end sync with a block below head.
    transactionsHelper.run { sendArbitraryTransaction().waitForInclusion() }

    data class NodeHead(
      val node: String,
      val blockHeight: Long,
      val blockTimestamp: Long,
    )

    await
      .pollInterval(1.seconds.toJavaDuration())
      .timeout(1.minutes.toJavaDuration())
      .untilAsserted {
        transactionsHelper.run { sendArbitraryTransaction().waitForInclusion() }

        val expectedMinBlockHeight =
          TestEnvironment.sequencerL2Client
            .ethBlockNumber()
            .send()
            .blockNumber
            .toLong()

        val blockHeights =
          TestEnvironment.clientsSyncablePreMergeAndPostMerge.entries
            .map { entry ->
              entry.value
                // this complex logic is necessary because besu has a bug
                // eth_getBlockByNumber(latest) sometimes returns genesis block in follower nodes
                .ethBlockNumber()
                .sendAsync()
                .toSafeFuture()
                .thenCompose { bn ->
                  entry.value
                    .ethGetBlockByNumber(DefaultBlockParameter.valueOf(bn.blockNumber), false)
                    .sendAsync()
                    .thenApply {
                      NodeHead(
                        entry.key,
                        bn.blockNumber.toLong(),
                        it.block.timestamp.toLong(),
                      )
                    }
                }
            }.let { SafeFuture.collectAll(it.stream()).get() }

        val nodesOutOfSync = blockHeights.filter { it.blockHeight < expectedMinBlockHeight }

        assertThat(nodesOutOfSync)
          .withFailMessage {
            "Nodes out of sync:" +
              "\nforks=$forksTimestamps" +
              "\nexpectedMinBlockHeight= $expectedMinBlockHeight, " +
              "\nout of sync nodes: $nodesOutOfSync"
          }.isEmpty()
      }
  }

  private fun everyoneArePeered() {
    log.info("Call add peer on all nodes and wait for peering to happen.")
    await.timeout(2.minutes.toJavaDuration()).untilAsserted {
      TestEnvironment.preMergeFollowerClients.forEach {
        try {
          it.value
            .adminAddPeer(
              "enode://14408801a444dafc44afbccce2eb755f902aed3b5743fed787b3c790e021fef28b8c827ed896aa4e8fb46e2" +
                "2bd67c39f994a73768b4b382f8597b0d44370e15d@11.11.11.101:30303",
            ).send()
        } catch (e: Exception) {
          if (it.key.contains("nethermind")) {
            log.debug("Nethermind returns response to admin_addPeer that is incompatible with Web3J")
          } else {
            throw e
          }
        }
        val peersResult =
          it.value
            .adminPeers()
            .send()
            .result
        val peers = peersResult.size
        log.info("Peers from node ${it.key}: $peers")
        assertThat(peers).withFailMessage("${it.key} isn't peered! Peers: $peersResult").isGreaterThan(0)
      }
    }
  }
}
