package net.consensys.zkevm.ethereum

import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import com.fasterxml.jackson.module.kotlin.readValue
import kotlinx.datetime.Clock
import linea.domain.BlockParameter
import linea.ethapi.EthApiClient
import linea.kotlin.decodeHex
import linea.kotlin.toULong
import linea.kotlin.toULongFromHex
import linea.web3j.transactionmanager.AsyncFriendlyTransactionManager
import linea.web3j.waitForTxReceipt
import net.consensys.linea.jsonrpc.JsonRpcErrorResponseException
import net.consensys.linea.testing.filesystem.getPathTo
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes
import org.web3j.crypto.Credentials
import org.web3j.protocol.Web3j
import org.web3j.protocol.core.Response
import org.web3j.tx.response.PollingTransactionReceiptProcessor
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.io.File
import java.math.BigInteger
import java.nio.file.Path
import kotlin.concurrent.timer
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds

data class Account(
  private val _privateKey: String,
  private val _address: String,
) {
  val privateKey: String
    get() = _privateKey.replace("0x", "")
  val address: String
    get() = _address.replace("0x", "")
}

data class AccountTransactionManager(
  val account: Account,
  val txManager: AsyncFriendlyTransactionManager,
) {
  val address: String
    get() = account.address
  val privateKey: String
    get() = account.privateKey
}

private val mapper = jacksonObjectMapper()

fun readJsonFile(file: File): Map<String, Any> {
  return mapper.readValue(file)
}

fun readGenesisFileAccounts(genesisJson: Map<String, Any>): List<Account> {
  @Suppress("UNCHECKED_CAST")
  val alloc = genesisJson["alloc"] as Map<String, Map<String, Any>>
  return alloc
    .filterValues { account -> account.containsKey("privateKey") }
    .map { (address, account) -> Account(account["privateKey"]!! as String, address) }
}

fun getTransactionManager(
  web3JClient: Web3j,
  privateKey: String,
): AsyncFriendlyTransactionManager {
  val credentials = Credentials.create(privateKey.replace("0x", ""))
  val receiptPoller = PollingTransactionReceiptProcessor(web3JClient, 100, 4000)
  return AsyncFriendlyTransactionManager(
    web3JClient,
    credentials,
    chainId = -1,
    receiptPoller,
  )
}

private inline val Int.ether get(): BigInteger = BigInteger.valueOf(this.toLong()).multiply(BigInteger.TEN.pow(18))

interface AccountManager {
  val chainId: Long
  fun whaleAccount(): Account
  fun generateAccount(initialBalanceWei: BigInteger = 10.ether): Account
  fun generateAccounts(numberOfAccounts: Int, initialBalanceWei: BigInteger = 10.ether): List<Account>
  fun waitForAccountToHaveBalance(account: Account, timeout: Duration = 8.seconds): SafeFuture<ULong>
  fun getTransactionManager(account: Account): AsyncFriendlyTransactionManager
}

private open class WhaleBasedAccountManager(
  val web3jClient: Web3j,
  val ethApiClient: EthApiClient,
  genesisFile: Path,
  val clock: Clock = Clock.System,
  val testWorkerIdProvider: () -> Long = { ProcessHandle.current().pid() },
  val log: Logger = LogManager.getLogger(WhaleBasedAccountManager::class.java),
) : AccountManager {
  private val whaleAccounts: List<Account>
  final override val chainId: Long

  init {
    val genesisJson = readJsonFile(genesisFile.toFile())
    @Suppress("UNCHECKED_CAST")
    chainId = ((genesisJson["config"] as Map<String, Any>)["chainId"] as Int).toLong()
    whaleAccounts = readGenesisFileAccounts(genesisJson)
  }

  private val txManagers =
    whaleAccounts.map { account -> getTransactionManager(web3jClient, account.privateKey) }

  private fun selectWhaleAccount(): Pair<Account, AsyncFriendlyTransactionManager> {
    val testWorkerId = testWorkerIdProvider.invoke()
    val accountIndex = testWorkerId % whaleAccounts.size
    val whaleAccount = whaleAccounts[accountIndex.toInt()]
    val whaleTxManager = txManagers[accountIndex.toInt()]
    // for faster feedback loop troubleshooting account selection
    // throw RuntimeException(
    //   "pid=${ProcessHandle.current().pid()}, " +
    //     "threadName=${Thread.currentThread().name} threadId=${Thread.currentThread().id} workerId=$testWorkerId " +
    //     "accIndex=$accountIndex/${whaleAccounts.size} whaleAccount=${whaleAccount.privateKey.takeLast(4)}"
    // )
    return Pair(whaleAccount, whaleTxManager)
  }

  override fun whaleAccount(): Account {
    return selectWhaleAccount().first
  }

  override fun generateAccount(initialBalanceWei: BigInteger): Account = generateAccounts(1, initialBalanceWei).first()

  override fun generateAccounts(numberOfAccounts: Int, initialBalanceWei: BigInteger): List<Account> {
    val (whaleAccount, whaleTxManager) = selectWhaleAccount()
    log.debug(
      "Generating accounts: chainId={} numberOfAccounts={} whaleAccount={}",
      chainId,
      numberOfAccounts,
      whaleAccount.address,
    )

    val result = synchronized(whaleTxManager) {
      (0..numberOfAccounts).map {
        val randomPrivKey = Bytes.random(32).toHexString().replace("0x", "")
        val newAccount = Account(randomPrivKey, Credentials.create(randomPrivKey).address)
        val transactionHash = try {
          retry {
            whaleTxManager.sendTransaction(
              /*gasPrice*/
              300_000_000.toBigInteger(),
              /*gasLimit*/
              21000.toBigInteger(),
              newAccount.address,
              "",
              initialBalanceWei,
            )
          }
        } catch (e: Exception) {
          val accountBalance =
            ethApiClient.ethGetBalance(whaleAccount.address.decodeHex(), BlockParameter.Tag.LATEST).get()
          throw RuntimeException(
            "Failed to send funds from accAddress=${whaleAccount.address}, " +
              "accBalance=$accountBalance, " +
              "accPrivKey=0x...${whaleAccount.privateKey.takeLast(8)}, " +
              "error: ${e.message}",
          )
        }
        newAccount to transactionHash
      }
    }
    result.forEach { (account, transactionHash) ->
      log.debug(
        "Waiting for account funding: newAccount={} txHash={} whaleAccount={}",
        account.address,
        transactionHash,
        whaleAccount.address,
      )
      ethApiClient.waitForTxReceipt(
        txHash = transactionHash.decodeHex(),
        expectedStatus = "0x1".toULongFromHex(),
        timeout = 40.seconds,
        pollingInterval = 500.milliseconds,
      )
      if (log.isDebugEnabled) {
        log.debug(
          "Account funded: newAccount={} balance={}wei",
          account.address,
          ethApiClient.ethGetBalance(account.address.decodeHex(), BlockParameter.Tag.LATEST).get(),
        )
      }
    }
    return result.map { it.first }
  }

  override fun getTransactionManager(account: Account): AsyncFriendlyTransactionManager {
    return getTransactionManager(
      web3jClient,
      account.privateKey,
    )
  }

  override fun waitForAccountToHaveBalance(account: Account, timeout: Duration): SafeFuture<ULong> {
    val futureResult = SafeFuture<ULong>()
    val startTime = clock.now()
    timer(
      name = "wait-account-balance-${account.address}",
      daemon = true,
      initialDelay = 0L,
      period = 100L,
    ) {
      val balance = ethApiClient.ethGetBalance(account.address.decodeHex(), BlockParameter.Tag.LATEST).get()
      if (balance > BigInteger.ZERO) {
        this.cancel()
        futureResult.complete(balance.toULong())
      } else if (clock.now() > startTime + timeout) {
        this.cancel()
        futureResult.completeExceptionally(
          RuntimeException(
            "Timed out ${timeout.inWholeSeconds}s waiting for account=${account.address} to have balance",
          ),
        )
      }
    }

    return futureResult
  }
}

object L1AccountManager : AccountManager by WhaleBasedAccountManager(
  web3jClient = Web3jClientManager.l1Client,
  ethApiClient = EthApiClientManager.l1Client,
  genesisFile = getPathTo(System.getProperty("L1_GENESIS", "docker/config/l1-node/el/genesis.json")),
  log = LogManager.getLogger(L1AccountManager::class.java),
)

object L2AccountManager : AccountManager by WhaleBasedAccountManager(
  web3jClient = Web3jClientManager.l2Client,
  ethApiClient = EthApiClientManager.l2Client,
  genesisFile = getPathTo(System.getProperty("L2_GENESIS", "docker/config/linea-local-dev-genesis-PoA-besu.json")),
  log = LogManager.getLogger(L2AccountManager::class.java),
)

fun <R, T : Response<R>> retry(
  timeout: Duration = 30.seconds,
  retryInterval: Duration = 1.seconds,
  action: () -> T,
): R {
  val start = Clock.System.now()
  var response: T? = null
  var latestError: Exception? = null
  do {
    try {
      response = action()
      if (response.hasError()) {
        Thread.sleep(retryInterval.inWholeMilliseconds)
      }
    } catch (e: Exception) {
      latestError = e
      Thread.sleep(retryInterval.inWholeMilliseconds)
    }
  } while (response?.hasError() == true && Clock.System.now() < start + timeout)

  return response?.let {
    if (it.hasError()) {
      throw JsonRpcErrorResponseException(it.error.code, it.error.message, it.error.data)
    } else {
      it.result
    }
  } ?: throw latestError!!
}
