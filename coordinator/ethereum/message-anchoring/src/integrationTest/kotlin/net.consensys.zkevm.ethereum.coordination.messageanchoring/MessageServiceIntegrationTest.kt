package net.consensys.zkevm.ethereum.coordination.messageanchoring

import build.linea.contract.LineaRollupV6
import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import linea.contract.l1.LineaContractVersion
import linea.kotlin.toBigInteger
import linea.kotlin.toULong
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.contract.AsyncFriendlyTransactionManager
import net.consensys.linea.contract.L2MessageService
import net.consensys.linea.contract.LineaRollupAsyncFriendly
import net.consensys.zkevm.coordinator.clients.smartcontract.LineaRollupSmartContractClient
import net.consensys.zkevm.ethereum.ContractsManager
import net.consensys.zkevm.ethereum.Web3jClientManager
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.web3j.abi.EventEncoder
import org.web3j.tx.gas.DefaultGasProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

@ExtendWith(VertxExtension::class)
class MessageServiceIntegrationTest {
  private val log: Logger = LogManager.getLogger(this::class.java)

  private val l2RecipientAddress = "0x03dfa322A95039BB679771346Ee2dBfEa0e2B773"
  private val l1Web3Client = Web3jClientManager.l1Client
  private val l2Web3jClient = Web3jClientManager.l2Client
  private lateinit var l2TransactionManager: AsyncFriendlyTransactionManager

  private val messagePollingInterval = 200.milliseconds
  private val maxScrapingTime = 2.seconds
  private val maxMessagesToAnchor = 5u
  private val blockRangeLoopLimit = 100u
  private val receiptPollingInterval = 500.milliseconds

  private lateinit var l1ContractLegacyClient: LineaRollupAsyncFriendly
  private lateinit var l1ContractClient: LineaRollupSmartContractClient
  private lateinit var l2Contract: L2MessageService

  private fun deployContracts() {
    val l1RollupDeploymentResult = ContractsManager.get()
      .deployLineaRollup(contractVersion = LineaContractVersion.V6)
      .get()
    @Suppress("DEPRECATION")
    l1ContractLegacyClient = l1RollupDeploymentResult.rollupOperatorClientLegacy
    l1ContractClient = l1RollupDeploymentResult.rollupOperatorClient
    val l2MessageServiceDeploymentResult = ContractsManager.get().deployL2MessageService().get()
    l2Contract = ContractsManager.get().connectL2MessageService(
      contractAddress = l2MessageServiceDeploymentResult.contractAddress,
      transactionManager = l2MessageServiceDeploymentResult.anchorerOperator.txManager
    )
    l2TransactionManager = l2MessageServiceDeploymentResult.anchorerOperator.txManager
  }

  @Test
  @Timeout(90, timeUnit = TimeUnit.SECONDS)
  fun `test anchoring with RollingHash`(vertx: Vertx) {
    deployContracts()
    testAnchoredHashesAreReturnedCorrectly(
      l1ContractLegacyClient,
      l2Contract,
      vertx
    )
  }

  fun testAnchoredHashesAreReturnedCorrectly(
    l1Contract: LineaRollupAsyncFriendly,
    l2Contract: L2MessageService,
    vertx: Vertx
  ) {
    val sentMessages = sendMessages(l1Contract)
    val messageAnchoringService = initialiseServices(
      vertx,
      l1Contract.contractAddress,
      l2Contract.contractAddress,
      earliestL1Block = sentMessages.first().l1BlockNumber
    )

    messageAnchoringService.start().get()
    await()
      .atMost(30, TimeUnit.SECONDS)
      .untilAsserted {
        val inboxStatusesFutures = sentMessages.map { message ->
          l2Contract.inboxL1L2MessageStatus(message.messageHash)
            .sendAsync()
            .toSafeFuture()
        }
        val anchoredStatuses = SafeFuture.collectAll(inboxStatusesFutures.stream()).get()
        assertThat(anchoredStatuses).allSatisfy { isAnchoredStatus(it) }
      }
    messageAnchoringService.stop().get()
  }

  private fun isAnchoredStatus(status: BigInteger): Boolean {
    return status == BigInteger.valueOf(1)
  }

  private fun initialiseServices(
    vertx: Vertx,
    l1ContractAddress: String,
    l2ContractAddress: String,
    earliestL1Block: ULong
  ): MessageAnchoringService {
    val l1EventQuerier = L1EventQuerierImpl(
      vertx = vertx,
      config = L1EventQuerierImpl.Config(
        pollingInterval = messagePollingInterval,
        maxEventScrapingTime = maxScrapingTime,
        earliestL1Block = earliestL1Block.toBigInteger(),
        maxMessagesToCollect = maxMessagesToAnchor,
        l1MessageServiceAddress = l1ContractAddress,
        "latest",
        blockRangeLoopLimit = blockRangeLoopLimit
      ),
      l1Web3jClient = l1Web3Client
    )

    val l2Querier = L2QuerierImpl(
      l2Client = l2Web3jClient,
      messageService = l2Contract,
      config = L2QuerierImpl.Config(
        blocksToFinalizationL2 = 0u,
        lastHashSearchWindow = 5u,
        contractAddressToListen = l2ContractAddress
      ),
      vertx = vertx
    )

    val l2MessageAnchorer: L2MessageAnchorer = L2MessageAnchorerImpl(
      vertx = vertx,
      l2Web3j = l2Web3jClient,
      l2Client = l2Contract,
      config = L2MessageAnchorerImpl.Config(
        receiptPollingInterval = receiptPollingInterval,
        maxReceiptRetries = 10u,
        blocksToFinalisation = 0
      )
    )

    return MessageAnchoringService(
      MessageAnchoringService.Config(
        pollingInterval = 500.milliseconds,
        maxMessagesToAnchor
      ),
      vertx,
      l1EventQuerier,
      l2MessageAnchorer,
      l2Querier,
      l1ContractClient,
      L2MessageService.load(
        l2ContractAddress,
        l2Web3jClient,
        l2TransactionManager,
        DefaultGasProvider()
      ),
      l2TransactionManager
    )
  }

  data class MessageSentResult(
    val l1BlockNumber: ULong,
    val messageHash: ByteArray
  )

  private fun sendMessages(contract: LineaRollupAsyncFriendly): List<MessageSentResult> {
    val baseMessageToSend = L1MessageToSend(
      recipient = l2RecipientAddress,
      fee = BigInteger.TEN,
      calldata = ByteArray(0),
      value = BigInteger.valueOf(200001)
    )
    val messagesToSend = listOf(
      baseMessageToSend,
      baseMessageToSend.copy(fee = BigInteger.valueOf(21)),
      baseMessageToSend.copy(value = BigInteger.valueOf(200001)),
      baseMessageToSend.copy(value = BigInteger.valueOf(200001)),
      baseMessageToSend.copy(value = BigInteger.valueOf(200001)),
      baseMessageToSend,
      baseMessageToSend.copy(fee = BigInteger.valueOf(21)),
      baseMessageToSend.copy(value = BigInteger.valueOf(200001)),
      baseMessageToSend.copy(value = BigInteger.valueOf(200001)),
      baseMessageToSend.copy(value = BigInteger.valueOf(200001)),
      baseMessageToSend,
      baseMessageToSend.copy(fee = BigInteger.valueOf(21)),
      baseMessageToSend.copy(value = BigInteger.valueOf(200001)),
      baseMessageToSend.copy(value = BigInteger.valueOf(200001)),
      baseMessageToSend.copy(value = BigInteger.valueOf(200001))
    )

    val futures = messagesToSend.map {
      contract.sendMessage(it.recipient, it.fee, it.calldata, it.value).sendAsync()
        .toSafeFuture()
        .thenApply { transactionReceipt ->
          log.debug("Message has been sent in block {}", transactionReceipt.blockNumber)
          val eventValues = LineaRollupV6.staticExtractEventParameters(
            LineaRollupV6.MESSAGESENT_EVENT,
            transactionReceipt.logs.first { log ->
              log.topics.contains(EventEncoder.encode(LineaRollupV6.MESSAGESENT_EVENT))
            }
          )
          MessageSentResult(
            l1BlockNumber = transactionReceipt.blockNumber.toULong(),
            messageHash = eventValues.indexedValues[2].value as ByteArray
          )
        }
    }
    val emittedEvents: List<MessageSentResult> = SafeFuture.collectAll(futures.stream()).get()

    return emittedEvents
  }
}
