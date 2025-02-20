package net.consensys.zkevm.ethereum.coordination.messageanchoring

import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.domain.BlockParameter
import linea.kotlin.toBigInteger
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.contract.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.EIP1559GasProvider
import net.consensys.linea.contract.L2MessageService
import net.consensys.linea.contract.LineaRollupAsyncFriendly
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.ethereum.ContractsManager
import net.consensys.zkevm.ethereum.Web3jClientManager
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import org.web3j.protocol.Web3j
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.ExecutionException
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
class L2MessageAnchorerIntegrationTest {
  private lateinit var testLineaRollupContractAddress: String
  private val l2RecipientAddress = "0x03dfa322A95039BB679771346Ee2dBfEa0e2B773"
  private val blockRangeLoopLimit = 100u

  private val gasLimit = 2_500_000uL
  private val feeHistoryBlockCount = 4u
  private val feeHistoryRewardPercentile = 15.0
  private val maxFeePerGasCap = 10000uL
  private lateinit var l1Web3jClient: Web3j
  private lateinit var l2Web3jClient: Web3j
  private lateinit var l2TransactionManager: AsyncFriendlyTransactionManager
  private lateinit var l2Contract: L2MessageService
  private lateinit var l1ContractLegacyClient: LineaRollupAsyncFriendly
  private lateinit var l1ContractClient: LineaRollupSmartContractClient

  private val messageAnchorerConfig = L2MessageAnchorerImpl.Config(
    receiptPollingInterval = 200.milliseconds,
    maxReceiptRetries = 100u,
    blocksToFinalisation = 0
  )
  private lateinit var l2MessageAnchorer: L2MessageAnchorerImpl
  private var l1ContractDeploymentBlockNumber: ULong = 0u

  @BeforeEach
  fun beforeEach(
    vertx: Vertx
  ) {
    val deploymentResult = ContractsManager.get().deployRollupAndL2MessageService().get()
    testLineaRollupContractAddress = deploymentResult.lineaRollup.contractAddress
    l1ContractDeploymentBlockNumber = deploymentResult.lineaRollup.contractDeploymentBlockNumber
    l1Web3jClient = Web3jClientManager.l1Client
    l2Web3jClient = Web3jClientManager.l2Client
    l2TransactionManager = deploymentResult.l2MessageService.anchorerOperator.txManager
    @Suppress("DEPRECATION")
    l1ContractLegacyClient = deploymentResult.lineaRollup.rollupOperatorClientLegacy
    l1ContractClient = deploymentResult.lineaRollup.rollupOperatorClient

    val eip1559GasProvider = EIP1559GasProvider(
      l2Web3jClient,
      EIP1559GasProvider.Config(
        gasLimit = gasLimit,
        maxFeePerGasCap = maxFeePerGasCap,
        feeHistoryBlockCount = feeHistoryBlockCount,
        feeHistoryRewardPercentile = feeHistoryRewardPercentile
      )
    )

    l2Contract = ContractsManager.get().connectL2MessageService(
      contractAddress = deploymentResult.l2MessageService.contractAddress,
      web3jClient = l2Web3jClient,
      transactionManager = deploymentResult.l2MessageService.anchorerOperator.txManager,
      gasProvider = eip1559GasProvider,
      smartContractErrors = mapOf("3b174434" to "MessageHashesListLengthHigherThanOneHundred")
    )

    l2MessageAnchorer = L2MessageAnchorerImpl(
      vertx,
      l2Web3jClient,
      l2Contract,
      messageAnchorerConfig
    )
  }

  @Test
  @Timeout(1, timeUnit = TimeUnit.MINUTES)
  fun `all hashes found are anchored`(vertx: Vertx, testContext: VertxTestContext) {
    val baseMessageToSend =
      L1MessageToSend(
        l2RecipientAddress,
        BigInteger.TEN,
        ByteArray(0),
        BigInteger.valueOf(100001)
      )
    val messagesToSend = listOf(
      baseMessageToSend,
      baseMessageToSend.copy(fee = BigInteger.valueOf(11)),
      baseMessageToSend.copy(value = BigInteger.valueOf(100001))
    )
    SafeFuture.collectAll(
      messagesToSend.map { message ->
        l1ContractLegacyClient
          .sendMessage(message.recipient, message.fee, message.calldata, message.value).sendAsync()
          .toSafeFuture()
      }.stream()
    ).get()

    val l1QuerierImpl = L1EventQuerierImpl(
      vertx,
      L1EventQuerierImpl.Config(
        pollingInterval = 200.milliseconds,
        maxEventScrapingTime = 2.seconds,
        earliestL1Block = l1ContractDeploymentBlockNumber.toBigInteger(),
        maxMessagesToCollect = 100u,
        l1MessageServiceAddress = testLineaRollupContractAddress,
        finalized = "latest",
        blockRangeLoopLimit = blockRangeLoopLimit
      ),
      l1Web3jClient = l1Web3jClient
    )

    l1QuerierImpl.getSendMessageEventsForAnchoredMessage(messageHash = null)
      .thenApply { events ->
        val rollingHash = l1ContractClient.getMessageRollingHash(
          blockParameter = BlockParameter.Tag.LATEST,
          messageNumber = events.last().messageNumber.toLong()
        ).get()
        l2MessageAnchorer.anchorMessages(events, rollingHash)
          .thenPeek {
            testContext.verify {
              val expectedLastAnchoredMessageNumber = events.last().messageNumber.toBigInteger()
              assertThat(l2Contract.lastAnchoredL1MessageNumber().send()).isEqualTo(expectedLastAnchoredMessageNumber)
              assertThat(l2Contract.l1RollingHashes(expectedLastAnchoredMessageNumber).send())
                .isEqualTo(rollingHash)
            }.completeNow()
          }
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(20, timeUnit = TimeUnit.SECONDS)
  fun `anchor messages gas estimation returns informative error`() {
    val exception = assertThrows<ExecutionException> {
      l2MessageAnchorer.anchorMessages(createRandomSendMessageEvents(101UL), Bytes32.random().toArray()).get()
    }
    assertThat(exception.message).contains(
      "3b174434",
      "MessageHashesListLengthHigherThanOneHundred",
      "Execution reverted"
    )
  }
}
