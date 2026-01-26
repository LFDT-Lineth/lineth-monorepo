/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package linea.plugin.acc.test

import com.fasterxml.jackson.core.JsonProcessingException
import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.databind.node.ObjectNode
import linea.plugin.acc.test.tests.web3j.generated.AcceptanceTestToken
import linea.plugin.acc.test.tests.web3j.generated.BLS12_MAP_FP_TO_G1
import linea.plugin.acc.test.tests.web3j.generated.DummyAdder
import linea.plugin.acc.test.tests.web3j.generated.EcAdd
import linea.plugin.acc.test.tests.web3j.generated.EcMul
import linea.plugin.acc.test.tests.web3j.generated.EcPairing
import linea.plugin.acc.test.tests.web3j.generated.EcRecover
import linea.plugin.acc.test.tests.web3j.generated.ExcludedPrecompiles
import linea.plugin.acc.test.tests.web3j.generated.LogEmitter
import linea.plugin.acc.test.tests.web3j.generated.ModExp
import linea.plugin.acc.test.tests.web3j.generated.MulmodExecutor
import linea.plugin.acc.test.tests.web3j.generated.RevertExample
import linea.plugin.acc.test.tests.web3j.generated.SimpleStorage
import linea.plugin.acc.test.utils.MemoryAppender
import net.consensys.linea.metrics.LineaMetricCategory.PRICING_CONF
import net.consensys.linea.metrics.LineaMetricCategory.SEQUENCER_LIVENESS
import net.consensys.linea.metrics.LineaMetricCategory.SEQUENCER_PROFITABILITY
import net.consensys.linea.metrics.LineaMetricCategory.TX_POOL_PROFITABILITY
import org.apache.commons.lang3.RandomStringUtils
import org.apache.tuweni.bytes.Bytes
import org.apache.tuweni.bytes.Bytes32
import org.apache.tuweni.units.bigints.UInt32
import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.datatypes.Address
import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.ethereum.core.ImmutableMiningConfiguration
import org.hyperledger.besu.ethereum.eth.transactions.ImmutableTransactionPoolConfiguration
import org.hyperledger.besu.ethereum.eth.transactions.TransactionPoolConfiguration
import org.hyperledger.besu.metrics.prometheus.MetricsConfiguration
import org.hyperledger.besu.plugin.services.metrics.MetricCategory
import org.hyperledger.besu.tests.acceptance.dsl.AcceptanceTestBase
import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.hyperledger.besu.tests.acceptance.dsl.condition.txpool.TxPoolConditions
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.hyperledger.besu.tests.acceptance.dsl.node.RunnableNode
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.BesuNodeConfigurationBuilder
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.NodeConfigurationFactory
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.genesis.GenesisConfigurationFactory.CliqueOptions
import org.hyperledger.besu.tests.acceptance.dsl.transaction.txpool.TxPoolTransactions
import org.hyperledger.besu.util.number.PositiveNumber
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.BeforeEach
import org.web3j.crypto.Credentials
import org.web3j.crypto.RawTransaction
import org.web3j.crypto.TransactionEncoder
import org.web3j.protocol.Web3j
import org.web3j.protocol.exceptions.TransactionException
import org.web3j.tx.RawTransactionManager
import org.web3j.tx.gas.DefaultGasProvider
import org.web3j.tx.response.PollingTransactionReceiptProcessor
import org.web3j.tx.response.TransactionReceiptProcessor
import java.io.IOException
import java.math.BigInteger
import java.net.URI
import java.net.http.HttpClient
import java.net.http.HttpRequest
import java.net.http.HttpResponse
import java.util.*

/** Base class for plugin tests. */
abstract class LineaPluginTestBase : AcceptanceTestBase() {

  companion object {
    const val MAX_CALLDATA_SIZE = 1188 // contract has a call data size of 1160
    val MAX_TX_GAS_LIMIT: Int = DefaultGasProvider.GAS_LIMIT.toInt()
    const val CHAIN_ID = 1337L
    const val BLOCK_PERIOD_SECONDS = 5
    val DEFAULT_LINEA_CLIQUE_OPTIONS = CliqueOptions(
      BLOCK_PERIOD_SECONDS,
      CliqueOptions.DEFAULT.epochLength(),
      false,
    )
    val DEFAULT_REQUESTED_PLUGINS = listOf(
      "LineaExtraDataPlugin",
      "LineaEstimateGasEndpointPlugin",
      "LineaSetExtraDataEndpointPlugin",
      "LineaTransactionPoolValidatorPlugin",
      "LineaTransactionSelectorPlugin",
      "LineaBundleEndpointsPlugin",
      "ForwardBundlesPlugin",
      "LineaTransactionValidatorPlugin",
    )

    private val HTTP_CLIENT: HttpClient = HttpClient.newHttpClient()

    @JvmStatic
    fun getResourcePath(resource: String): String {
      return Objects.requireNonNull(LineaPluginTestBase::class.java.getResource(resource)).path
    }
  }

  @BeforeEach
  protected open fun setup() {
    minerNode = createCliqueNodeWithExtraCliOptionsAndRpcApis(
      "miner1",
      getCliqueOptions(),
      getTestCliOptions(),
      setOf("LINEA", "MINER", "PLUGINS"),
      false,
      DEFAULT_REQUESTED_PLUGINS,
    )
    minerNode.transactionPoolConfiguration = ImmutableTransactionPoolConfiguration.builder()
      .from(TransactionPoolConfiguration.DEFAULT)
      .noLocalPriority(true)
      .build()
    cluster.start(minerNode)
  }

  protected open fun getTestCliOptions(): List<String> {
    return TestCommandLineOptionsBuilder().build()
  }

  protected open fun getCliqueOptions(): CliqueOptions {
    return DEFAULT_LINEA_CLIQUE_OPTIONS
  }

  @AfterEach
  protected open fun stop() {
    cluster.stop()
    cluster.close()
    MemoryAppender.reset()
  }

  protected open fun maybeCustomGenesisExtraData(): Optional<Bytes32> {
    return Optional.empty()
  }

  protected fun createCliqueNodeWithExtraCliOptionsAndRpcApis(
    name: String,
    cliqueOptions: CliqueOptions,
    extraCliOptions: List<String>,
    extraRpcApis: Set<String>,
    isEngineRpcEnabled: Boolean,
    requestedPlugins: List<String>,
  ): BesuNode {
    val node = NodeConfigurationFactory()

    val nodeConfBuilder = BesuNodeConfigurationBuilder()
      .name(name)
      .miningConfiguration(
        // enable mining
        // allow for a single iteration to take all the slot time
        // set plugin max selection time to 5% of slot time
        ImmutableMiningConfiguration.builder()
          .poaBlockTxsSelectionMaxTime(
            PositiveNumber.fromInt(cliqueOptions.blockPeriodSeconds() * 1000),
          )
          .pluginBlockTxsSelectionMaxTime(PositiveNumber.fromInt(2))
          .mutableInitValues(
            ImmutableMiningConfiguration.MutableInitValues.builder()
              .isMiningEnabled(true)
              .build(),
          )
          .build(),
      )
      .jsonRpcConfiguration(node.createJsonRpcWithCliqueEnabledConfig(extraRpcApis))
      .webSocketConfiguration(node.createWebSocketEnabledConfig())
      .inProcessRpcConfiguration(node.createInProcessRpcConfiguration(extraRpcApis))
      .devMode(false)
      .jsonRpcTxPool()
      .engineRpcEnabled(isEngineRpcEnabled)
      .genesisConfigProvider { validators ->
        Optional.of(provideGenesisConfig(validators, cliqueOptions))
      }
      .extraCLIOptions(extraCliOptions)
      .metricsConfiguration(
        MetricsConfiguration.builder()
          .enabled(true)
          .port(0)
          .metricCategories(
            setOf(
              PRICING_CONF,
              SEQUENCER_PROFITABILITY,
              TX_POOL_PROFITABILITY,
              SEQUENCER_LIVENESS,
            ),
          )
          .build(),
      )
      .requestedPlugins(requestedPlugins)

    return besu.create(nodeConfBuilder.build())
  }

  protected open fun provideGenesisConfig(
    validators: Collection<RunnableNode>,
    cliqueOptions: CliqueOptions,
  ): String {
    val genesis = GenesisConfigurationFactory.createCliqueGenesisConfig(validators, cliqueOptions).get()

    return maybeCustomGenesisExtraData()
      .map { ed -> setGenesisCustomExtraData(genesis, ed) }
      .orElse(genesis)
  }

  protected fun setGenesisCustomExtraData(genesis: String, customExtraData: Bytes32): String {
    val om = ObjectMapper()
    val root: ObjectNode = try {
      om.readTree(genesis) as ObjectNode
    } catch (e: JsonProcessingException) {
      throw RuntimeException(e)
    }
    val existingExtraData = Bytes.fromHexString(root.get("extraData").asText())
    val updatedExtraData = Bytes.concatenate(customExtraData, existingExtraData.slice(32))
    root.put("extraData", updatedExtraData.toHexString())
    return root.toPrettyString()
  }

  protected fun sendTransactionsWithGivenLengthPayload(
    simpleStorage: SimpleStorage,
    accounts: List<String>,
    web3j: Web3j,
    num: Int,
  ) {
    val contractAddress = simpleStorage.contractAddress
    val txData = simpleStorage.set(RandomStringUtils.secure().nextAlphabetic(num)).encodeFunctionCall()
    val hashes = mutableListOf<String>()

    accounts.forEach { a ->
      val credentials = Credentials.create(a)
      val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID)
      repeat(5) {
        try {
          hashes.add(
            txManager.sendTransaction(
              DefaultGasProvider.GAS_PRICE,
              DefaultGasProvider.GAS_LIMIT,
              contractAddress,
              txData,
              BigInteger.ZERO,
            ).transactionHash,
          )
        } catch (e: IOException) {
          throw RuntimeException(e)
        }
      }
    }

    assertTransactionsInCorrectBlocks(web3j, hashes, num)
  }

  private fun assertTransactionsInCorrectBlocks(web3j: Web3j, hashes: List<String>, num: Int) {
    val txMap = hashMapOf<Long, Int>()
    val receiptProcessor = createReceiptProcessor(web3j)

    // CallData for the transaction for empty String is 68 and grows in steps of 32 with (String
    // size / 32)
    val maxTxs = MAX_CALLDATA_SIZE / (68 + ((num + 31) / 32) * 32)

    // Wait for transaction to be mined and check that there are no more than maxTxs per block
    hashes.forEach { h ->
      val transactionReceipt = try {
        receiptProcessor.waitForTransactionReceipt(h)
      } catch (e: IOException) {
        throw RuntimeException(e)
      } catch (e: TransactionException) {
        throw RuntimeException(e)
      }

      val blockNumber = transactionReceipt.blockNumber.toLong()
      txMap.compute(blockNumber) { _, n -> (n ?: 0) + 1 }

      // make sure that no block contained more than maxTxs
      assertThat(txMap[blockNumber]).isLessThanOrEqualTo(maxTxs)
    }
    // make sure that at least one block has maxTxs
    assertThat(txMap).containsValue(maxTxs)
  }

  protected fun deployLogEmitter(): LogEmitter {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = LogEmitter.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun deploySimpleStorage(): SimpleStorage {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = SimpleStorage.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun deployDummyAdder(): DummyAdder {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = DummyAdder.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun deployMulmodExecutor(): MulmodExecutor {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = MulmodExecutor.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun deployModExp(): ModExp {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = ModExp.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun deployBLS12_MAP_FP_TO_G1(): BLS12_MAP_FP_TO_G1 {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = BLS12_MAP_FP_TO_G1.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun deployEcPairing(): EcPairing {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = EcPairing.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun deployEcAdd(): EcAdd {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = EcAdd.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun deployEcMul(): EcMul {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = EcMul.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun deployEcRecover(): EcRecover {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = EcRecover.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun deployRevertExample(): RevertExample {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = RevertExample.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun deployAcceptanceTestToken(): AcceptanceTestToken {
    val web3j = minerNode.nodeRequests().eth()
    // 1000 AT tokens will be assigned to this account on deploy
    val credentials = accounts.primaryBenefactor.web3jCredentialsOrThrow()
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = AcceptanceTestToken.deploy(web3j, txManager, DefaultGasProvider())
    val contract = deploy.send()
    val balance = contract.balanceOf(accounts.primaryBenefactor.address).send()
    assertThat(balance).isEqualTo(1000)
    return contract
  }

  protected fun deployExcludedPrecompiles(): ExcludedPrecompiles {
    val web3j = minerNode.nodeRequests().eth()
    val credentials = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY)
    val txManager = RawTransactionManager(web3j, credentials, CHAIN_ID, createReceiptProcessor(web3j))

    val deploy = ExcludedPrecompiles.deploy(web3j, txManager, DefaultGasProvider())
    return deploy.send()
  }

  protected fun assertTransactionsMinedInSeparateBlocks(web3j: Web3j, hashes: List<String>) {
    val receiptProcessor = createReceiptProcessor(web3j)

    val blockNumbers = hashSetOf<Long>()
    for (hash in hashes) {
      val receipt = receiptProcessor.waitForTransactionReceipt(hash)
      assertThat(receipt).isNotNull
      val isAdded = blockNumbers.add(receipt.blockNumber.toLong())
      assertThat(isAdded).isEqualTo(true)
    }
  }

  protected fun assertTransactionsMinedInSameBlock(web3j: Web3j, hashes: List<String>) {
    val receiptProcessor = createReceiptProcessor(web3j)
    val blockNumbers = hashes.map { hash ->
      try {
        val receipt = receiptProcessor.waitForTransactionReceipt(hash)
        assertThat(receipt).isNotNull
        receipt.blockNumber.toLong()
      } catch (e: IOException) {
        throw RuntimeException(e)
      } catch (e: TransactionException) {
        throw RuntimeException(e)
      }
    }.toSet()

    assertThat(blockNumbers.size).isEqualTo(1)
  }

  protected fun assertTransactionNotInThePool(hash: String) {
    minerNode.verify(
      TxPoolConditions(TxPoolTransactions())
        .notInTransactionPool(Hash.fromHexString(hash)),
    )
  }

  protected fun getTxPoolContent(): List<Map<String, String>> {
    return minerNode.execute(TxPoolTransactions().txPoolContents)
  }

  private fun createReceiptProcessor(web3j: Web3j): TransactionReceiptProcessor {
    return PollingTransactionReceiptProcessor(
      web3j,
      maxOf(1000L, DEFAULT_LINEA_CLIQUE_OPTIONS.blockPeriodSeconds() * 1000L / 5),
      DEFAULT_LINEA_CLIQUE_OPTIONS.blockPeriodSeconds() * 3,
    )
  }

  protected fun sendTransactionWithGivenLengthPayload(
    account: String,
    web3j: Web3j,
    num: Int,
  ): String {
    val to = Address.fromHexString("fe3b557e8fb62b89f4916b721be55ceb828dbd73").toString()
    val txManager = RawTransactionManager(web3j, Credentials.create(account))

    return txManager.sendTransaction(
      DefaultGasProvider.GAS_PRICE,
      BigInteger.valueOf(MAX_TX_GAS_LIMIT.toLong()),
      to,
      RandomStringUtils.secure().nextAlphabetic(num),
      BigInteger.ZERO,
    ).transactionHash
  }

  protected fun createExtraDataPricingField(
    fixedCostKWei: Long,
    variableCostKWei: Long,
    minGasPriceKWei: Long,
  ): Bytes32 {
    val fixed = UInt32.valueOf(BigInteger.valueOf(fixedCostKWei))
    val variable = UInt32.valueOf(BigInteger.valueOf(variableCostKWei))
    val min = UInt32.valueOf(BigInteger.valueOf(minGasPriceKWei))

    return Bytes32.rightPad(
      Bytes.concatenate(Bytes.of(1.toByte()), fixed.toBytes(), variable.toBytes(), min.toBytes()),
    )
  }

  protected fun getMetricValue(
    category: MetricCategory,
    metricName: String,
    labelValues: List<Map.Entry<String, String>>,
  ): Double {
    val metricsReq = HttpRequest.newBuilder()
      .GET()
      .uri(URI.create(minerNode.metricsHttpUrl().get()))
      .build()

    val respLines = HTTP_CLIENT.send(metricsReq, HttpResponse.BodyHandlers.ofLines())

    val searchString = (
      category.applicationPrefix.orElse("") +
        category.name +
        "_" +
        metricName +
        labelValues.joinToString(",", "{", "}") { lv ->
          "${lv.key}=\"${lv.value}\""
        }
      )

    val foundMetric = respLines.body().filter { line -> line.startsWith(searchString) }.findFirst()

    return foundMetric
      .map { line -> line.substring(searchString.length).trim() }
      .map { it.toDouble() }
      .orElse(Double.NaN)
  }

  protected fun getLog(): String {
    return MemoryAppender.getLog()
  }

  protected fun getAndResetLog(): String {
    val log = MemoryAppender.getLog()
    MemoryAppender.reset()
    return log
  }

  protected fun encodedCallModExp(
    modExp: ModExp,
    sender: Account,
    nonce: Int,
    input: Bytes,
  ): ByteArray {
    val modExpCalldata = modExp.callModExp(input.toArrayUnsafe()).encodeFunctionCall()

    val modExpCall = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.valueOf(nonce.toLong()),
      DefaultGasProvider.GAS_LIMIT,
      modExp.contractAddress,
      BigInteger.ZERO,
      modExpCalldata,
      DefaultGasProvider.GAS_PRICE,
      DefaultGasProvider.GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return TransactionEncoder.signMessage(modExpCall, sender.web3jCredentialsOrThrow())
  }

  protected fun encodedCallEcPairing(
    ecPairing: EcPairing,
    sender: Account,
    nonce: Int,
    input: Bytes,
  ): ByteArray {
    val ecPairingCalldata = ecPairing.callEcPairing(input.toArrayUnsafe()).encodeFunctionCall()

    val ecPairingCall = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.valueOf(nonce.toLong()),
      DefaultGasProvider.GAS_LIMIT,
      ecPairing.contractAddress,
      BigInteger.ZERO,
      ecPairingCalldata,
      DefaultGasProvider.GAS_PRICE,
      DefaultGasProvider.GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return TransactionEncoder.signMessage(ecPairingCall, sender.web3jCredentialsOrThrow())
  }

  protected fun encodedCallEcAdd(
    ecAdd: EcAdd,
    sender: Account,
    nonce: Int,
    input: Bytes,
  ): ByteArray {
    val ecAddCalldata = ecAdd.callEcAdd(input.toArrayUnsafe()).encodeFunctionCall()

    val ecAddCall = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.valueOf(nonce.toLong()),
      DefaultGasProvider.GAS_LIMIT,
      ecAdd.contractAddress,
      BigInteger.ZERO,
      ecAddCalldata,
      DefaultGasProvider.GAS_PRICE,
      DefaultGasProvider.GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return TransactionEncoder.signMessage(ecAddCall, sender.web3jCredentialsOrThrow())
  }

  protected fun encodedCallEcMul(
    ecMul: EcMul,
    sender: Account,
    nonce: Int,
    input: Bytes,
  ): ByteArray {
    val ecMulCalldata = ecMul.callEcMul(input.toArrayUnsafe()).encodeFunctionCall()

    val ecMulCall = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.valueOf(nonce.toLong()),
      DefaultGasProvider.GAS_LIMIT,
      ecMul.contractAddress,
      BigInteger.ZERO,
      ecMulCalldata,
      DefaultGasProvider.GAS_PRICE,
      DefaultGasProvider.GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return TransactionEncoder.signMessage(ecMulCall, sender.web3jCredentialsOrThrow())
  }

  protected fun encodedCallEcRecover(
    ecRecover: EcRecover,
    sender: Account,
    nonce: Int,
    input: Bytes,
  ): ByteArray {
    val ecRecoverCalldata = ecRecover.callEcRecover(input.toArrayUnsafe()).encodeFunctionCall()

    val ecRecoverCall = RawTransaction.createTransaction(
      CHAIN_ID,
      BigInteger.valueOf(nonce.toLong()),
      DefaultGasProvider.GAS_LIMIT,
      ecRecover.contractAddress,
      BigInteger.ZERO,
      ecRecoverCalldata,
      DefaultGasProvider.GAS_PRICE,
      DefaultGasProvider.GAS_PRICE.multiply(BigInteger.TEN).add(BigInteger.ONE),
    )

    return TransactionEncoder.signMessage(ecRecoverCall, sender.web3jCredentialsOrThrow())
  }

  fun asserLogsContain(
    target: String,
    logs: String = getAndResetLog(),
  ) {
    assertThat(logs)
      .withFailMessage { "Expected Besu logs to contain '$target'" }
      .contains(target)
  }
}
