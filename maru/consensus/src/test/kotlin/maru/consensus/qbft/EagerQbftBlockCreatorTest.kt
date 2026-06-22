/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package maru.consensus.qbft

import linea.testing.besu.BesuFactory
import linea.testing.besu.BesuTransactionsHelper
import maru.consensus.ValidatorProvider
import maru.consensus.qbft.adapters.QbftBlockHeaderAdapter
import maru.consensus.qbft.adapters.toBeaconBlock
import maru.consensus.qbft.adapters.toBeaconBlockHeader
import maru.consensus.state.FinalizationState
import maru.core.BeaconBlock
import maru.core.BeaconState
import maru.core.EMPTY_HASH
import maru.core.GENESIS_EXECUTION_PAYLOAD
import maru.core.HashUtil
import maru.core.SealedBeaconBlock
import maru.core.Validator
import maru.core.ext.DataGenerators
import maru.core.ext.metrics.TestMetrics
import maru.database.BeaconChain
import maru.executionlayer.client.PragueWeb3JJsonRpcExecutionLayerEngineApiClient
import maru.executionlayer.manager.ExecutionLayerManager
import maru.executionlayer.manager.JsonRpcExecutionLayerManager
import maru.executionlayer.manager.LatestBlockMetadata
import maru.executionlayer.mappers.Mappers.toDomain
import maru.serialization.rlp.bodyRoot
import maru.serialization.rlp.headerHash
import maru.serialization.rlp.stateRoot
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.consensus.common.bft.ConsensusRoundIdentifier
import org.hyperledger.besu.consensus.common.bft.blockcreation.ProposerSelector
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockCreator
import org.hyperledger.besu.consensus.qbft.core.types.QbftBlockHeader
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.hyperledger.besu.tests.acceptance.dsl.node.ThreadBesuNodeRunner
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.ClusterConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.Mockito.verify
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.reset
import org.mockito.kotlin.whenever
import org.web3j.protocol.core.DefaultBlockParameter
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3JClient
import tech.pegasys.teku.ethereum.executionclient.web3j.Web3jClientBuilder
import tech.pegasys.teku.infrastructure.async.SafeFuture.completedFuture
import tech.pegasys.teku.infrastructure.time.SystemTimeProvider
import java.time.Clock
import java.time.Duration
import java.time.Instant
import java.time.temporal.ChronoUnit
import kotlin.random.Random
import kotlin.time.Duration.Companion.milliseconds

class EagerQbftBlockCreatorTest {
  private lateinit var cluster: Cluster
  private lateinit var besuInstance: BesuNode
  private lateinit var ethApiClient: Web3JClient
  private val proposerSelector = Mockito.mock(ProposerSelector::class.java)
  private val validatorProvider = Mockito.mock(ValidatorProvider::class.java)
  private val beaconChain = Mockito.mock(BeaconChain::class.java)
  private val clock = Mockito.mock(Clock::class.java)
  private val validator = Validator(Random.nextBytes(20))
  private val feeRecipient = Random.nextBytes(20)
  private val prevRandaoProvider = { _: ULong, _: ByteArray -> Bytes32.random().toArray() }
  private lateinit var executionLayerManager: ExecutionLayerManager
  private val validatorSet = (DataGenerators.randomValidators() + validator).toSortedSet()

  @BeforeEach
  fun beforeEach() {
    cluster = Cluster(
      ClusterConfigurationBuilder().build(),
      NetConditions(NetTransactions()),
      ThreadBesuNodeRunner(),
    )
    besuInstance = BesuFactory.buildTestBesu().also {
      cluster.start(it)
    }
    ethApiClient = Web3jClientBuilder()
      .endpoint(besuInstance.engineRpcUrl().get())
      .timeout(Duration.ofMinutes(1))
      .timeProvider(SystemTimeProvider.SYSTEM_TIME_PROVIDER)
      .executionClientEventsPublisher { }
      .build()
    reset(
      proposerSelector,
      validatorProvider,
      beaconChain,
    )
    executionLayerManager = createExecutionLayerManager()
  }

  /*
   * Creates EagerQbftBlockCreator instance
   * Runs a block building attempt to ensure that Besu's state isn't a clean slate
   * Sets the mock up according to the generated genesis beacon block
   */
  private fun setup(
    executionLayerManager: ExecutionLayerManager,
    sealedGenesisBeaconBlock: SealedBeaconBlock,
    mainBlockCreator: QbftBlockCreator,
    sequence: Long,
    round: Int,
  ): EagerQbftBlockCreator {
    whenever(
      beaconChain.getSealedBeaconBlock(sealedGenesisBeaconBlock.beaconBlock.beaconBlockHeader.hash()),
    ).thenReturn(
      sealedGenesisBeaconBlock,
    )
    whenever(proposerSelector.selectProposerForRound(ConsensusRoundIdentifier(sequence, round))).thenReturn(
      Address.wrap(
        Bytes.wrap
          (validator.address),
      ),
    )
    whenever(
      validatorProvider.getValidatorsAfterBlock(sequence.toULong() - 1U),
    ).thenReturn(completedFuture(validatorSet))

    val genesisBlockHash = sealedGenesisBeaconBlock.beaconBlock.beaconBlockBody.executionPayload.blockHash
    val eagerQbftBlockCreator =
      EagerQbftBlockCreator(
        manager = executionLayerManager,
        delegate = mainBlockCreator,
        finalizationStateProvider = {
          FinalizationState(
            genesisBlockHash,
            genesisBlockHash,
          )
        },
        prevRandaoProvider = prevRandaoProvider,
        feeRecipient = feeRecipient,
        config = EagerQbftBlockCreator.Config(
          minBlockBuildTime = 500.milliseconds,
        ),
        beaconChain = beaconChain,
      )
    return eagerQbftBlockCreator
  }

  @Test
  fun `can create a non empty block with new timestamp`() {
    val genesisExecutionPayload =
      ethApiClient.eth1Web3j
        .ethGetBlockByNumber(
          DefaultBlockParameter.valueOf("earliest"),
          true,
        ).send()
        .block
        .toDomain()
    val parentBlock =
      BeaconBlock(
        beaconBlockHeader = DataGenerators.randomBeaconBlockHeader(
          0U,
        ),
        beaconBlockBody = DataGenerators
          .randomBeaconBlockBody()
          .copy(executionPayload = genesisExecutionPayload),
      )
    val sealedGenesisBeaconBlock = SealedBeaconBlock(parentBlock, emptySet())
    val parentHeader = QbftBlockHeaderAdapter(sealedGenesisBeaconBlock.beaconBlock.beaconBlockHeader)

    val mainBlockCreator = createDelayedBlockCreator(round = 1)
    val eagerQbftBlockCreator =
      setup(executionLayerManager, sealedGenesisBeaconBlock, mainBlockCreator, sequence = 1, round = 1)
    createProposalToBeRejected(
      clock = clock,
      genesisBlockHash = sealedGenesisBeaconBlock.beaconBlock.beaconBlockBody.executionPayload.blockHash,
      mainBlockCreator = mainBlockCreator,
      parentHeader = parentHeader,
    )

    setNextSecondMillis(0)
    // Try to create an new non-empty block instead of a non-empty proposal
    val acceptedBlockTimestamp = (clock.millis() / 1000)
    val acceptedBlock = eagerQbftBlockCreator.createBlock(acceptedBlockTimestamp, parentHeader)
    val acceptedBeaconBlock = acceptedBlock.block().toBeaconBlock()

    // block header fields
    val createdBlockHeader = acceptedBeaconBlock.beaconBlockHeader
    assertThat(createdBlockHeader.number).isEqualTo(1UL)
    assertThat(createdBlockHeader.round).isEqualTo(1U)
    assertThat(createdBlockHeader.timestamp).isEqualTo(acceptedBlockTimestamp.toULong())
    assertThat(createdBlockHeader.proposer).isEqualTo(validator)

    // block header roots
    val stateRoot =
      HashUtil.stateRoot(
        BeaconState(
          acceptedBeaconBlock.beaconBlockHeader.copy(stateRoot = EMPTY_HASH),
          validatorSet,
        ),
      )
    assertThat(
      createdBlockHeader.bodyRoot,
    ).isEqualTo(
      HashUtil.bodyRoot(acceptedBeaconBlock.beaconBlockBody),
    )
    assertThat(createdBlockHeader.stateRoot).isEqualTo(stateRoot)
    assertThat(createdBlockHeader.parentRoot).isEqualTo(parentHeader.toBeaconBlockHeader().hash())
    assertThat(
      acceptedBeaconBlock.beaconBlockHeader.hash(),
    ).isEqualTo(HashUtil.headerHash(acceptedBeaconBlock.beaconBlockHeader))

    // block body fields
    val createdBlockBody = acceptedBeaconBlock.beaconBlockBody
    assertThat(
      createdBlockBody.prevCommitSeals,
    ).isEqualTo(
      sealedGenesisBeaconBlock.commitSeals,
    )
    assertThat(createdBlockBody.executionPayload.timestamp).isEqualTo(acceptedBlockTimestamp.toULong())
    assertThat(createdBlockBody.executionPayload.transactions).isNotEmpty
  }

  @Test
  fun `clamps next block timestamp strictly above EL head timestamp on genesis parent`() {
    val latestExecutionLayerBlock =
      ethApiClient.eth1Web3j
        .ethGetBlockByNumber(
          DefaultBlockParameter.valueOf("latest"),
          true,
        ).send()
        .block
        .toDomain()
    val genesisBeaconBlock =
      BeaconBlock(
        beaconBlockHeader = DataGenerators.randomBeaconBlockHeader(
          0U,
        ),
        beaconBlockBody = DataGenerators
          .randomBeaconBlockBody()
          .copy(executionPayload = GENESIS_EXECUTION_PAYLOAD),
      )
    val sealedGenesisBeaconBlock = SealedBeaconBlock(genesisBeaconBlock, emptySet())
    val parentHeader = QbftBlockHeaderAdapter(sealedGenesisBeaconBlock.beaconBlock.beaconBlockHeader)

    val spyExecutionLayerManager = Mockito.spy(executionLayerManager)
    val mainBlockCreator = createDelayedBlockCreator(round = 0, manager = spyExecutionLayerManager)
    val eagerQbftBlockCreator =
      setup(spyExecutionLayerManager, sealedGenesisBeaconBlock, mainBlockCreator, sequence = 1, round = 0)

    setNextSecondMillis(0)
    val proposedTimestamp = clock.millis() / 1000
    Mockito
      .doReturn(
        completedFuture(
          LatestBlockMetadata(
            blockHash = latestExecutionLayerBlock.blockHash,
            timestamp = proposedTimestamp.toULong(),
          ),
        ),
      ).whenever(spyExecutionLayerManager)
      .getLatestBlockMetadata()

    val acceptedBlock = eagerQbftBlockCreator.createBlock(proposedTimestamp, parentHeader)
    val acceptedHeader = acceptedBlock.block().toBeaconBlock().beaconBlockHeader

    val expectedClampedTimestamp = (proposedTimestamp + 1).toULong()
    assertThat(acceptedHeader.timestamp).isEqualTo(expectedClampedTimestamp)
    verify(spyExecutionLayerManager).setHeadAndStartBlockBuilding(
      headHash = eq(latestExecutionLayerBlock.blockHash),
      safeHash = any(),
      finalizedHash = any(),
      nextBlockTimestamp = any(),
      feeRecipient = any(),
      prevRandao = any(),
    )
  }

  @Test
  fun `uses latest blockhash when parent block is genesis`() {
    val latestExecutionLayerBlock =
      ethApiClient.eth1Web3j
        .ethGetBlockByNumber(
          DefaultBlockParameter.valueOf("latest"),
          true,
        ).send()
        .block
        .toDomain()
    val genesisBeaconBlock =
      BeaconBlock(
        beaconBlockHeader = DataGenerators.randomBeaconBlockHeader(
          0U,
        ), // Genesis block has number 0
        beaconBlockBody = DataGenerators
          .randomBeaconBlockBody()
          .copy(executionPayload = GENESIS_EXECUTION_PAYLOAD),
      )
    val sealedGenesisBeaconBlock = SealedBeaconBlock(genesisBeaconBlock, emptySet())
    val parentHeader = QbftBlockHeaderAdapter(sealedGenesisBeaconBlock.beaconBlock.beaconBlockHeader)

    val spyExecutionLayerManager = Mockito.spy(executionLayerManager)
    val mainBlockCreator = createDelayedBlockCreator(round = 0, manager = spyExecutionLayerManager)
    val eagerQbftBlockCreator =
      setup(spyExecutionLayerManager, sealedGenesisBeaconBlock, mainBlockCreator, sequence = 1, round = 0)

    setNextSecondMillis(0)
    val acceptedBlockTimestamp = (clock.millis() / 1000)
    eagerQbftBlockCreator.createBlock(acceptedBlockTimestamp, parentHeader)

    // Verify that getLatestBlockMetadata() was called since parent block number is 0 (genesis):
    // EagerQbftBlockCreator needs both the EL head hash and its timestamp to clamp
    // payloadAttrs.timestamp strictly above the EL head's timestamp.
    verify(spyExecutionLayerManager).getLatestBlockMetadata()
    verify(spyExecutionLayerManager).setHeadAndStartBlockBuilding(
      headHash = eq(latestExecutionLayerBlock.blockHash),
      safeHash = any(),
      finalizedHash = any(),
      nextBlockTimestamp = any(),
      feeRecipient = any(),
      prevRandao = any(),
    )
  }

  private fun createProposalToBeRejected(
    clock: Clock,
    genesisBlockHash: ByteArray,
    mainBlockCreator: QbftBlockCreator,
    parentHeader: QbftBlockHeader,
  ) {
    val rejectedBlockTimestamp = (clock.millis() / 1000) + 1
    executionLayerManager
      .setHeadAndStartBlockBuilding(
        headHash = genesisBlockHash,
        safeHash = genesisBlockHash,
        finalizedHash = genesisBlockHash,
        nextBlockTimestamp = rejectedBlockTimestamp.toULong(),
        feeRecipient = validator.address,
      ).get()
    val transaction = BesuTransactionsHelper().createTransfers(1u)
    besuInstance.execute(transaction)
    Thread.sleep(1000)
    val rejectedBlock = mainBlockCreator.createBlock(rejectedBlockTimestamp, parentHeader)
    val rejectedBlockTransactions =
      rejectedBlock
        .block()
        .toBeaconBlock()
        .beaconBlockBody.executionPayload.transactions
    assertThat(
      rejectedBlockTransactions,
    ).isNotEmpty
  }

  private fun createExecutionLayerManager(): ExecutionLayerManager {
    val engineApiClient =
      Web3jClientBuilder()
        .endpoint(besuInstance.engineRpcUrl().get())
        .timeout(Duration.ofMinutes(1))
        .timeProvider(SystemTimeProvider.SYSTEM_TIME_PROVIDER)
        .executionClientEventsPublisher { }
        .build()
    return JsonRpcExecutionLayerManager(
      PragueWeb3JJsonRpcExecutionLayerEngineApiClient(
        web3jClient = engineApiClient,
        metricsFacade = TestMetrics.TestMetricsFacade,
      ),
    )
  }

  private fun createDelayedBlockCreator(
    round: Int,
    manager: ExecutionLayerManager = executionLayerManager,
  ): DelayedQbftBlockCreator =
    DelayedQbftBlockCreator(
      manager = manager,
      proposerSelector = proposerSelector,
      validatorProvider = validatorProvider,
      beaconChain = beaconChain,
      round = round,
    )

  /*
   * Sets the clock to return the next second with the given milliseconds
   */
  private fun setNextSecondMillis(secondMillis: Int) {
    require(secondMillis in 0..999) { "secondMillis must be between 0 and 999" }

    val currentMillis = System.currentTimeMillis()
    val currentSecond = Instant.ofEpochMilli(currentMillis).truncatedTo(ChronoUnit.SECONDS)
    val nextSecond = currentSecond.plusSeconds(1).plusMillis(secondMillis.toLong())
    whenever(clock.millis()).thenAnswer {
      nextSecond.toEpochMilli()
    }
  }
}
