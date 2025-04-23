package net.consensys.zkevm.ethereum.coordination.messageanchoring

import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import linea.kotlin.toHexString
import linea.kotlin.toULong
import linea.web3j.gas.EIP1559GasProvider
import net.consensys.linea.contract.L2MessageService
import net.consensys.zkevm.ethereum.signing.ECKeypairSigner
import net.consensys.zkevm.ethereum.signing.ECKeypairSignerAdapter
import org.apache.tuweni.bytes.Bytes32
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.RepeatedTest
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.ArgumentMatchers
import org.mockito.Mockito
import org.mockito.kotlin.any
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever
import org.web3j.crypto.Credentials
import org.web3j.crypto.Hash
import org.web3j.crypto.Keys
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.DefaultBlockParameter
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.methods.response.EthBlock
import org.web3j.protocol.core.methods.response.EthFeeHistory
import org.web3j.protocol.core.methods.response.EthGasPrice
import org.web3j.protocol.core.methods.response.EthSendTransaction
import org.web3j.protocol.core.methods.response.TransactionReceipt
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.*
import java.util.concurrent.TimeUnit
import kotlin.time.Duration.Companion.milliseconds

@ExtendWith(VertxExtension::class)
class L2MessageAnchorerImplTest {
  private val testContractAddress = "0x6d976c9b8ceee705d4fe8699b44e5eb58242f484"
  private val latestBlockNumber = 12345
  private val transactionBlockNumber = 12340
  private val keyPair = Keys.createEcKeyPair()
  private val signer = ECKeypairSigner(keyPair)
  private val pollingInterval = 10.milliseconds
  private val gasEstimationPercentile = 0.5
  private val gasLimit = 100uL
  private val feeHistoryBlockCount = 4u
  private val maxFeePerGasCap = 10000uL
  private val retryCount = 10u
  private val finalisedBlockDistance = latestBlockNumber.minus(transactionBlockNumber).toLong()

  private val txHash = "0xfa41235fcc064e57ab2566d65732a25a24b36ff6edba3cdd5eb482071b435906"

  @RepeatedTest(10)
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun messageAnchoring_returnsTransactionReceipt(vertx: Vertx, testContext: VertxTestContext) {
    val mockReceipt = mock<TransactionReceipt>()
    whenever(mockReceipt.isStatusOK).thenReturn(true)
    whenever(mockReceipt.transactionHash).thenReturn(txHash)
    whenever(mockReceipt.blockNumber).thenReturn(BigInteger.valueOf(transactionBlockNumber.toLong()))

    val l2ClientMock = createMockedWeb3jClient(mockReceipt, transactionBlockNumber, latestBlockNumber, 1337)
    val messageManager = createL2MessageServiceContractWithSimpleKeypairSigner(l2ClientMock)

    val testEvents = createRandomSendMessageEvents(11UL)

    val l2MessageAnchorerImpl =
      L2MessageAnchorerImpl(
        vertx,
        l2ClientMock,
        messageManager,
        L2MessageAnchorerImpl.Config(
          pollingInterval,
          retryCount,
          finalisedBlockDistance
        )
      )

    l2MessageAnchorerImpl.anchorMessages(
      testEvents,
      Bytes32.ZERO.toArray()
    )
      .thenApply {
        testContext
          .verify {
            assertThat(it).isNotNull
            assertThat(it.blockNumber).isEqualTo(mockReceipt.blockNumber)
            assertThat(it.transactionHash).isEqualTo(mockReceipt.transactionHash)
          }
          .completeNow()
      }
  }

  private fun createL2MessageServiceContractWithSimpleKeypairSigner(
    l2Web3jClient: Web3j
  ): L2MessageService {
    val signerAdapter = ECKeypairSignerAdapter(signer, keyPair.publicKey)
    val credentials = Credentials.create(signerAdapter)
    return L2MessageService.load(
      testContractAddress,
      l2Web3jClient,
      credentials,
      EIP1559GasProvider(
        l2Web3jClient,
        EIP1559GasProvider.Config(gasLimit, maxFeePerGasCap, feeHistoryBlockCount, gasEstimationPercentile)
      )
    )
  }

  private fun createMockedWeb3jClient(
    expectedTransactionReceipt: TransactionReceipt,
    txBlockNumber: Int,
    currentBlockNumber: Int,
    chainId: Int
  ): Web3j {
    val web3jClient = mock<Web3j>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    val ethBlock = mock<EthBlock>()
    val block = mock<EthBlock.Block>()
    whenever(ethBlock.block).thenReturn(block)
    whenever(block.number).thenReturn(BigInteger.valueOf(txBlockNumber.toLong()))
      .thenReturn(BigInteger.valueOf(currentBlockNumber.toLong()))

    whenever(web3jClient.ethGetBlockByNumber(any(), any()).sendAsync())
      .thenAnswer { SafeFuture.completedFuture(ethBlock) }

    whenever(
      web3jClient
        .ethFeeHistory(
          ArgumentMatchers.eq(4),
          ArgumentMatchers.eq(DefaultBlockParameter.valueOf("latest")),
          ArgumentMatchers.eq(listOf(gasEstimationPercentile))
        )
        .sendAsync()
    )
      .thenAnswer {
        val feeHistoryResponse = EthFeeHistory()
        val feeHistory = EthFeeHistory.FeeHistory()
        feeHistory.setReward(mutableListOf(mutableListOf("0x1000")))
        feeHistory.setBaseFeePerGas(mutableListOf("0x100"))
        feeHistory.setOldestBlock(BigInteger.valueOf(currentBlockNumber.toLong() - 1).toULong().toHexString())
        feeHistory.gasUsedRatio = listOf(1.0)
        feeHistoryResponse.result = feeHistory
        SafeFuture.completedFuture(feeHistoryResponse)
      }
    whenever(web3jClient.ethGasPrice().sendAsync()).thenAnswer {
      val gasPriceResponse = EthGasPrice()
      gasPriceResponse.result = "0x100"
      SafeFuture.completedFuture(gasPriceResponse)
    }
    val sendTransactionResponse = EthSendTransaction()
    val expectedTransactionHash = txHash
    sendTransactionResponse.result = expectedTransactionHash
    whenever(web3jClient.ethSendRawTransaction(any())).thenAnswer {
      val hashToReturn = Hash.sha3(it.arguments[0] as String)
      sendTransactionResponse.result = hashToReturn
      val requestMock = mock<Request<*, EthSendTransaction>>()
      whenever(requestMock.send()).thenReturn(sendTransactionResponse)
      requestMock
    }
    whenever(web3jClient.ethGetTransactionReceipt(any()).send().transactionReceipt)
      .thenReturn(Optional.of(expectedTransactionReceipt))
    whenever(web3jClient.ethChainId().send().chainId)
      .thenReturn(BigInteger.valueOf(chainId.toLong()))

    return web3jClient
  }
}
