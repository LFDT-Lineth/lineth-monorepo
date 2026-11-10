package linea.contract

import io.reactivex.Flowable
import io.vertx.core.Context
import io.vertx.core.Vertx
import linea.kotlin.toBigInteger
import linea.web3j.domain.Eip4844Transaction
import linea.web3j.gas.AtomicContractEIP1559GasProvider
import linea.web3j.gas.EIP1559GasFees
import linea.web3j.gas.EIP4844GasFees
import linea.web3j.gas.EIP4844GasProvider
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import lineth.vertx.vertxTestOptions
import net.consensys.linea.contract.Web3JContractAsyncHelper
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.web3j.abi.datatypes.Function
import org.web3j.crypto.Credentials
import org.web3j.crypto.TransactionDecoder
import org.web3j.crypto.transaction.type.Transaction1559
import org.web3j.protocol.Web3j
import org.web3j.protocol.Web3jService
import org.web3j.protocol.core.BatchRequest
import org.web3j.protocol.core.BatchResponse
import org.web3j.protocol.core.Request
import org.web3j.protocol.core.Response
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.protocol.websocket.events.Notification
import org.web3j.service.TxSignServiceImpl
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.math.BigInteger
import java.util.concurrent.CompletableFuture
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.TimeUnit

class Web3JContractAsyncHelperTest {
  private val chainId = 1337L
  private val contractAddress = "0x" + "aa".repeat(20)
  private val function = Function("someFunction", emptyList(), emptyList())
  private val eip1559Fees = EIP1559GasFees(maxPriorityFeePerGas = 7uL, maxFeePerGas = 11uL)
  private val eip4844Fees = EIP4844GasFees(eip1559Fees, maxFeePerBlobGas = 13uL)

  private lateinit var vertx: Vertx
  private lateinit var web3jService: FakeWeb3jService
  private lateinit var gasProvider: TimerCompletedGasProvider
  private lateinit var helper: Web3JContractAsyncHelper

  @BeforeEach
  fun setUp() {
    vertx = Vertx.vertx(vertxTestOptions)
    web3jService = FakeWeb3jService()
    val web3j = Web3j.build(web3jService)
    gasProvider = TimerCompletedGasProvider(vertx, chainId, eip4844Fees)
    helper = Web3JContractAsyncHelper(
      contractAddress = contractAddress,
      web3j = web3j,
      transactionManager = AsyncFriendlyTransactionManager(
        web3j,
        // obviously fake key, test only
        TxSignServiceImpl(Credentials.create("0x" + "11".repeat(32))),
        -1L,
      ),
      contractGasProvider = gasProvider,
      smartContractErrors = emptyMap(),
      useEthEstimateGas = false,
    )
  }

  @AfterEach
  fun tearDown() {
    vertx.close()
  }

  private fun <T> runOnEventLoop(action: () -> SafeFuture<T>): T {
    val result = SafeFuture<T>()
    vertx.runOnContext {
      action().propagateTo(result)
    }
    return result.get(10, TimeUnit.SECONDS)
  }

  @Test
  fun `sendTransactionAsync resolves EIP-1559 fees without blocking the event loop`() {
    runOnEventLoop { SafeFuture.of(helper.sendTransactionAsync(function, BigInteger.ZERO)) }

    assertThat(gasProvider.asyncCallsOnEventLoop).containsExactly(true)
    val sentTx = TransactionDecoder.decode(web3jService.sentRawTransactions.single())
    val sentTxFees = sentTx.transaction as Transaction1559
    assertThat(sentTxFees.maxPriorityFeePerGas).isEqualTo(eip1559Fees.maxPriorityFeePerGas.toBigInteger())
    assertThat(sentTxFees.maxFeePerGas).isEqualTo(eip1559Fees.maxFeePerGas.toBigInteger())
  }

  @Test
  fun `executeBlobEthCall resolves EIP-4844 fees without blocking the event loop`() {
    runOnEventLoop { helper.executeBlobEthCall(function, listOf(ByteArray(10)), gasPriceCaps = null) }

    assertThat(gasProvider.asyncCallsOnEventLoop).containsExactly(true)
    val ethCallTx = web3jService.ethCallTransactions.single() as Eip4844Transaction
    assertThat(ethCallTx.maxFeePerBlobGas).isEqualTo("0xd")
  }

  @Test
  fun `sendBlobCarryingTransactionAndGetTxHash resolves EIP-4844 fees without blocking the event loop`() {
    val txHash = runOnEventLoop {
      helper.sendBlobCarryingTransactionAndGetTxHash(function, listOf(ByteArray(10)), gasPriceCaps = null)
    }

    assertThat(txHash).isEqualTo(FakeWeb3jService.TX_HASH)
    assertThat(gasProvider.asyncCallsOnEventLoop).containsExactly(true)
    assertThat(web3jService.sentRawTransactions).hasSize(1)
  }

  // fees complete on a Vert.x timer, like the L1 client's retry backoff
  private class TimerCompletedGasProvider(
    private val vertx: Vertx,
    private val chainId: Long,
    private val fees: EIP4844GasFees,
  ) : AtomicContractEIP1559GasProvider, EIP4844GasProvider {
    val asyncCallsOnEventLoop = CopyOnWriteArrayList<Boolean>()

    private fun <T> completeOnTimer(value: T): SafeFuture<T> {
      asyncCallsOnEventLoop.add(Context.isOnEventLoopThread())
      val future = SafeFuture<T>()
      vertx.setTimer(10) { future.complete(value) }
      return future
    }

    override fun getEIP1559GasFeesAsync(): SafeFuture<EIP1559GasFees> = completeOnTimer(fees.eip1559GasFees)

    override fun getEIP4844GasFeesAsync(): SafeFuture<EIP4844GasFees> = completeOnTimer(fees)

    override fun getEIP1559GasFees(): EIP1559GasFees = throw AssertionError("blocking fee lookup")

    override fun getEIP4844GasFees(): EIP4844GasFees = throw AssertionError("blocking fee lookup")

    override fun getMaxFeePerGas(): BigInteger = throw AssertionError("blocking fee lookup")

    override fun getMaxPriorityFeePerGas(): BigInteger = throw AssertionError("blocking fee lookup")

    override fun getChainId(): Long = chainId

    override fun getGasLimit(transaction: Transaction?): BigInteger = BigInteger.valueOf(1_000_000)

    @Deprecated("Deprecated in Java")
    override fun getGasLimit(): BigInteger = BigInteger.valueOf(1_000_000)

    @Deprecated("Deprecated in Java")
    override fun getGasPrice(): BigInteger = throw UnsupportedOperationException()
  }

  private class FakeWeb3jService : Web3jService {
    companion object {
      const val TX_HASH = "0x" + "bb"
    }

    val sentRawTransactions = CopyOnWriteArrayList<String>()
    val ethCallTransactions = CopyOnWriteArrayList<Any>()

    private fun result(request: Request<*, *>): Any {
      return when (request.method) {
        "eth_getTransactionCount" -> "0x0"
        "eth_call" -> {
          ethCallTransactions.add(request.params[0])
          "0x"
        }
        "eth_sendRawTransaction" -> {
          sentRawTransactions.add(request.params[0] as String)
          TX_HASH
        }
        else -> throw UnsupportedOperationException("unexpected request ${request.method}")
      }
    }

    override fun <T : Response<*>> sendAsync(request: Request<*, *>, responseType: Class<T>): CompletableFuture<T> {
      @Suppress("UNCHECKED_CAST")
      val response = responseType.getDeclaredConstructor().newInstance() as Response<Any>
      response.result = result(request)
      @Suppress("UNCHECKED_CAST")
      return CompletableFuture.completedFuture(response as T)
    }

    override fun <T : Response<*>> send(request: Request<*, *>, responseType: Class<T>): T =
      sendAsync(request, responseType).get()

    override fun sendBatch(batchRequest: BatchRequest): BatchResponse = throw UnsupportedOperationException()

    override fun sendBatchAsync(batchRequest: BatchRequest): CompletableFuture<BatchResponse> =
      throw UnsupportedOperationException()

    override fun <T : Notification<*>> subscribe(
      request: Request<*, *>,
      unsubscribeMethod: String,
      responseType: Class<T>,
    ): Flowable<T> = throw UnsupportedOperationException()

    override fun close() {}
  }
}
