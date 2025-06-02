package net.consensys.zkevm.load

import net.consensys.zkevm.load.model.CreateWallets.createWallets
import net.consensys.zkevm.load.model.EthConnectionImpl
import net.consensys.zkevm.load.model.ExecutionDetails
import net.consensys.zkevm.load.model.JSON
import net.consensys.zkevm.load.model.SmartContractCalls
import net.consensys.zkevm.load.model.TransactionDetail
import net.consensys.zkevm.load.model.Wallet
import net.consensys.zkevm.load.model.inner.BatchMint
import net.consensys.zkevm.load.model.inner.CallContractReference
import net.consensys.zkevm.load.model.inner.CallExistingContract
import net.consensys.zkevm.load.model.inner.ContractCall
import net.consensys.zkevm.load.model.inner.CreateContract
import net.consensys.zkevm.load.model.inner.GenericCall
import net.consensys.zkevm.load.model.inner.MethodAndParameter
import net.consensys.zkevm.load.model.inner.Mint
import net.consensys.zkevm.load.model.inner.NEW
import net.consensys.zkevm.load.model.inner.Request
import net.consensys.zkevm.load.model.inner.RoundRobinMoneyTransfer
import net.consensys.zkevm.load.model.inner.Scenario
import net.consensys.zkevm.load.model.inner.ScenarioDefinition
import net.consensys.zkevm.load.model.inner.SelfTransactionWithPayload
import net.consensys.zkevm.load.model.inner.SelfTransactionWithRandomPayload
import net.consensys.zkevm.load.model.inner.TransferOwnerShip
import org.slf4j.Logger
import org.slf4j.LoggerFactory
import org.web3j.crypto.Credentials
import org.web3j.protocol.core.methods.request.Transaction
import org.web3j.protocol.core.methods.response.EthBlock.TransactionObject
import org.web3j.utils.Numeric
import java.io.FileReader
import java.io.IOException
import java.io.InputStreamReader
import java.math.BigInteger
import java.nio.file.Paths
import java.security.InvalidAlgorithmParameterException
import java.security.NoSuchAlgorithmException
import java.security.NoSuchProviderException
import java.security.SecureRandom
import java.text.SimpleDateFormat
import java.util.stream.LongStream

class TestExecutor(request: String, pk: String) {
  private val walletsFunding: WalletsFunding
  private val smartContractCalls: SmartContractCalls
  private val sourceWallet: Wallet
  private val test: Request
  private val executionDetails: ExecutionDetails
  private val ethConnection: EthConnectionImpl
  private val numberGenerator = SecureRandom()

  init {
    test = getRequest(request)
    executionDetails = ExecutionDetails(test)
    ethConnection = EthConnectionImpl(test.context.url)
    sourceWallet = Wallet(pk, -1, ethConnection.getNonce(Credentials.create(pk).address))

    walletsFunding = WalletsFunding(ethConnection, sourceWallet)
    smartContractCalls = SmartContractCalls(ethConnection)
  }

  private fun getRequest(request: String): Request {
    val builder = JSON.createGson()
    val resource = this.javaClass.classLoader.getResource(request)
    if (resource != null) {
      return Request.translate(
        builder.create().fromJson(
          InputStreamReader(resource.openStream()),
          net.consensys.zkevm.load.swagger.Request::class.java,
        ),
      )
    } else {
      val file = Paths.get(request)
      val reader = FileReader(file.toFile())
      return Request.translate(
        builder.create()
          .fromJson(reader, net.consensys.zkevm.load.swagger.Request::class.java),
      )
    }
  }

  companion object {
    val logger: Logger = LoggerFactory.getLogger(TestExecutor::class.java)
  }

  @Throws(Exception::class)
  fun test() {
    for (i: Int in 0 until test.context.nbOfExecutions) {
      try {
        test(i)
      } catch (t: Throwable) {
        logger.error("Catching throwable: {}. Resuming with next test occurrence if there is one.", t)
      }
    }
  }

  @Throws(Exception::class)
  fun test(i: Int) {
    val formater = SimpleDateFormat("dd/MM/yyyy hh:mm:ss")
    logger.info("[TIME] System time:{}, test run:{}", formater.format(System.currentTimeMillis()), i)
    val calls = expand(test.calls)
    val contractAddresses: MutableMap<String, String> = HashMap()
    // deploy contracts that can be referenced in the test.
    contractAddresses.putAll(prepareContracts(test.context.contracts, test.context.chainId))

    // transfer funds to the wallets we create
    val walletsForCalls: MutableList<Map<Int, Wallet>> = ArrayList()
    for (call in calls) {
      walletsForCalls.add(prepareWallets(call, test.context.chainId))
    }

    // set nonce to wallet that was used for fund initialization
    sourceWallet.theoreticalNonce.set(ethConnection.getNonce(sourceWallet.address))

    // generate the transactions
    val txs: MutableMap<Wallet, List<TransactionDetail>> = HashMap()
    for (index in calls.indices) {
      val call = calls[index]
      val walletMap = walletsForCalls[index]
      val newTxs =
        prepareTransactions(call, test.context.chainId, walletMap, contractAddresses)
      executionDetails.addTestTxs(newTxs)
      merge(txs, newTxs)
    }

    val blockNumberBeforeSending = ethConnection.getCurrentEthBlockNumber()
    // send the transactions
    val targetNonce = ethConnection.sendAllTransactions(txs)
    walletsFunding.waitForTransactions(targetNonce)
    val blockAfterSending = ethConnection.getCurrentEthBlockNumber()

    val blocks =
      LongStream.range(blockNumberBeforeSending.toLong() + 1, blockAfterSending.toLong() + 1)
        .mapToObj { blockId ->
          ethConnection.getEthGetBlockByNumber(blockId)
        }.toList()

    val txHashToBlockId = blocks.stream().map { b ->
      b.block.transactions.stream().map { tx ->
        {
          ((tx.get() as TransactionObject).hash) to b.block.number
        }
      }.toList()
    }.toList().flatten().associate { f -> f.invoke() }

    executionDetails.mapToBlocks(txHashToBlockId)
    println(JSON.createGson().create().toJson(executionDetails))
  }

  private fun expand(calls: List<ScenarioDefinition>?): List<Scenario> {
    val result: MutableList<Scenario> = ArrayList()
    calls?.forEach { call ->
      for (i in 1..call.nbOfExecution) {
        result.add(call.scenario)
      }
    }

    return result
  }

  private fun merge(
    txs: MutableMap<Wallet, List<TransactionDetail>>,
    walletListMap: Map<Wallet, List<TransactionDetail>>,
  ) {
    walletListMap.forEach { (k: Wallet, v: List<TransactionDetail>) ->
      if (txs.containsKey(k)) {
        val l: MutableList<TransactionDetail> = ArrayList()
        l.addAll(txs[k]!!)
        l.addAll(v)
        txs[k] = l
      } else {
        txs[k] = v
      }
    }
  }

  @Throws(IOException::class, InterruptedException::class)
  private fun prepareContracts(
    contracts: List<CreateContract>?,
    chainId: Int,
  ): Map<String, String> {
    val contractAdresses: MutableMap<String, String> = HashMap()
    for (contract in contracts!!) {
      contractAdresses[contract.name] =
        smartContractCalls.createContract(sourceWallet, contract, chainId, executionDetails)
      logger.info(
        "[CONTRACT] contract {} created with address {} and owner {}",
        contract.name,
        contractAdresses[contract.name],
        sourceWallet,
      )
    }
    return contractAdresses
  }

  @Throws(Exception::class)
  private fun prepareWallets(scenario: Scenario, chainId: Int): Map<Int, Wallet> {
    return when (scenario) {
      is RoundRobinMoneyTransfer -> {
        val transactionForEstimation = Transaction.createEtherTransaction(
          /* from = */
          sourceWallet.address,
          /* nonce = */
          sourceWallet.theoreticalNonceValue,
          /* gasPrice = */
          null,
          /* gasLimit = */
          null,
          /* to = */
          Numeric.prependHexPrefix(sourceWallet.address),
          /* value = */
          RoundRobinMoneyTransfer.valueToTransfer,
        )
        val (gasPrice, gasLimit) = ethConnection.estimateGasPriceAndLimit(transactionForEstimation)

        prepareWallets(
          nbWallets = scenario.nbWallets,
          nbTransferPerWallets = scenario.nbTransfers,
          chainId = chainId,
          gasPerCall = gasLimit,
          gasPricePerCall = gasPrice,
          valuePerCall = RoundRobinMoneyTransfer.valueToTransfer,
        )
      }

      is SelfTransactionWithPayload -> {
        if (scenario.wallet == NEW) {
          val transactionForEstimation = Transaction(
            /* from = */
            sourceWallet.address,
            /* nonce = */
            sourceWallet.theoreticalNonceValue,
            /* gasPrice = */
            null,
            /* gasLimit = */
            null,
            /* to = */
            Numeric.prependHexPrefix(sourceWallet.address),
            /* value = */
            null,
            /* data = */
            Numeric.toHexString(scenario.payload.toByteArray()),
          )

          val (gasPrice, gasLimit) = ethConnection.estimateGasPriceAndLimit(transactionForEstimation)

          prepareWallets(
            nbWallets = scenario.nbWallets,
            nbTransferPerWallets = scenario.nbTransfers,
            chainId = chainId,
            gasPerCall = gasLimit,
            gasPricePerCall = gasPrice,
          )
        } else {
          java.util.Map.of(-1, sourceWallet)
        }
      }

      is SelfTransactionWithRandomPayload -> {
        if (scenario.wallet == NEW) {
          val payload = Util.generateRandomPayloadOfSize(scenario.payloadSize)
          val transactionForEstimation = Transaction(
            /* from = */
            sourceWallet.address,
            /* nonce = */
            sourceWallet.theoreticalNonceValue,
            /* gasPrice = */
            null,
            /* gasLimit = */
            null,
            /* to = */
            Numeric.prependHexPrefix(sourceWallet.address),
            /* value = */
            null,
            /* data = */
            Numeric.toHexString(payload.toByteArray()),
          )

          val (gasPrice, gasLimit) = ethConnection.estimateGasPriceAndLimit(transactionForEstimation)

          prepareWallets(
            nbWallets = scenario.nbWallets,
            nbTransferPerWallets = scenario.nbTransfers,
            chainId = chainId,
            gasPerCall = gasLimit,
            gasPricePerCall = gasPrice,
          )
        } else {
          java.util.Map.of(-1, sourceWallet)
        }
      }

      is ContractCall -> {
        if (scenario.wallet == NEW) {
          // TODO: gas estimation for funding more precise
          val gasPrice = ethConnection.ethGasPrice()
          prepareWallets(
            nbWallets = 1,
            nbTransferPerWallets = scenario.contract.nbCalls(),
            chainId = chainId,
            gasPerCall = scenario.gasLimit(),
            gasPricePerCall = gasPrice,
          )
        } else {
          java.util.Map.of()
        }
      }

      else -> {
        throw IllegalStateException("Unexpected value: $scenario")
      }
    }
  }

  @Throws(Exception::class)
  private fun prepareTransactions(
    scenario: Scenario?,
    chainId: Int,
    walletMap: Map<Int, Wallet>,
    contractAddresses: Map<String, String>,
  ): Map<Wallet, List<TransactionDetail>> {
    return when (scenario) {
      is RoundRobinMoneyTransfer -> {
        generateTxs(scenario.nbTransfers, walletMap, chainId)
      }

      is SelfTransactionWithPayload -> {
        generateTxsWithPayload(
          scenario.payload,
          chainId,
          walletMap,
          scenario.nbTransfers,
        )
      }

      is SelfTransactionWithRandomPayload -> {
        generateTxWithRandomPayload(
          scenario.payloadSize,
          chainId,
          walletMap,
          scenario.nbTransfers,
        )
      }

      is ContractCall -> {
        prepareCalls(scenario, chainId, contractAddresses, walletMap)
      }

      else -> throw IllegalStateException("Unexpected value for scenarioType: $scenario")
    }
  }

  @Throws(IOException::class)
  private fun generateTxWithRandomPayload(
    payloadSize: Int,
    chainId: Int,
    walletMap: Map<Int, Wallet>,
    nbTransfers: Int,
  ): Map<Wallet, List<TransactionDetail>> {
    return walletsFunding.generateTxWithRandomPayload(walletMap, payloadSize, chainId, nbTransfers)
  }

  @Throws(IOException::class)
  private fun generateTxsWithPayload(
    payload: String,
    chainId: Int,
    walletMap: Map<Int, Wallet>,
    nbTransfers: Int,
  ): Map<Wallet, List<TransactionDetail>> {
    return walletsFunding.generateTxsWithPayload(walletMap, payload, chainId, nbTransfers)
  }

  @Throws(IOException::class, InterruptedException::class)
  private fun prepareCalls(
    call: ContractCall,
    chainId: Int,
    contractAddresses: Map<String, String>,
    walletMap: Map<Int, Wallet>,
  ): Map<Wallet, List<TransactionDetail>> {
    return when (val contractType = call.contract) {
      is CallExistingContract -> {
        prepareCallToExistingContract(
          call.wallet(sourceWallet, walletMap),
          contractType.contractAddress,
          contractType.methodAndParameters,
          chainId,
        )
      }

      is CreateContract -> {
        prepareContractCreation(
          call.wallet(sourceWallet, walletMap),
          contractType,
          chainId,
        )
      }

      is CallContractReference -> {
        val address = contractAddresses[contractType.contractName]
        prepareCallToExistingContract(
          call.wallet(sourceWallet, walletMap),
          address!!,
          contractType.methodAndParameters,
          chainId,
        )
      }

      else -> throw IllegalStateException("Unexpected value: $contractType")
    }
  }

  @Throws(IOException::class, InterruptedException::class)
  private fun prepareCallToExistingContract(
    wallet: Wallet,
    contractAddress: String,
    methodAndParameters: MethodAndParameter?,
    chainId: Int,
  ): Map<Wallet, List<TransactionDetail>> {
    return when (methodAndParameters) {
      is GenericCall -> {
        val encodedFunction = smartContractCalls.genericCall(
          methodAndParameters.methodName,
          methodAndParameters.parameters,
        )
        smartContractCalls.getRequests(
          contractAddress,
          wallet,
          encodedFunction,
          methodAndParameters.nbOfTimes,
          chainId,
        )
      }

      is Mint -> {
        val encodedFunction = smartContractCalls.mint(
          methodAndParameters.address,
          methodAndParameters.amount.toLong(),
        )
        smartContractCalls.getRequests(
          contractAddress,
          wallet,
          encodedFunction,
          methodAndParameters.nbOfTimes,
          chainId,
        )
      }

      is BatchMint -> {
        val encodedFunction = smartContractCalls.batchMint(
          methodAndParameters.address,
          methodAndParameters.amount.toLong(),
        )
        smartContractCalls.getRequests(
          contractAddress,
          wallet,
          encodedFunction,
          methodAndParameters.nbOfTimes,
          chainId,
        )
      }

      is TransferOwnerShip -> {
        val encodedFunction =
          smartContractCalls.transferOwnership(methodAndParameters.destinationAddress)
        smartContractCalls.getRequests(
          contractAddress,
          wallet,
          encodedFunction,
          methodAndParameters.nbOfTimes,
          chainId,
        )
      }

      else -> throw IllegalStateException("Unexpected value: $methodAndParameters")
    }
  }

  @Throws(IOException::class, InterruptedException::class)
  private fun prepareContractCreation(
    wallet: Wallet,
    contract: CreateContract,
    chainId: Int,
  ): Map<Wallet, List<TransactionDetail>> {
    return java.util.Map.of(
      wallet,
      listOf(
        smartContractCalls.getCreateContractTransaction(
          wallet,
          contract.byteCode,
          chainId,
        ),
      ),
    )
  }

  @Throws(
    InvalidAlgorithmParameterException::class,
    NoSuchAlgorithmException::class,
    NoSuchProviderException::class,
    IOException::class,
    InterruptedException::class,
  )
  private fun prepareWallets(
    nbWallets: Int,
    nbTransferPerWallets: Int,
    chainId: Int,
    gasPerCall: BigInteger,
    gasPricePerCall: BigInteger,
    valuePerCall: BigInteger = BigInteger.ZERO,
  ): Map<Int, Wallet> {
    val wallets: Map<Int, Wallet> = createWallets(nbWallets)
    executionDetails.addInitialization(
      walletsFunding.initializeWallets(
        wallets = wallets,
        nbTransactions = nbTransferPerWallets,
        sourceWallet = sourceWallet,
        chainId = chainId,
        gasPerCall = gasPerCall,
        gasPricePerCall = gasPricePerCall,
        valuePerCall = valuePerCall,
      ),
    )
    return wallets
  }

  @Throws(Exception::class)
  private fun generateTxs(
    nbTransferPerWallets: Int,
    wallets: Map<Int, Wallet>,
    chainId: Int,
  ): Map<Wallet, List<TransactionDetail>> {
    return walletsFunding.generateTransactions(
      wallets,
      RoundRobinMoneyTransfer.valueToTransfer,
      nbTransferPerWallets,
      chainId,
    )
  }
}
