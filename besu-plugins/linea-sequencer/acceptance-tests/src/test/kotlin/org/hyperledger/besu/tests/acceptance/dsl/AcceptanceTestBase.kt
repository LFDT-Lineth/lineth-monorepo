/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package org.hyperledger.besu.tests.acceptance.dsl

import org.assertj.core.api.Assertions.assertThat
import org.hyperledger.besu.datatypes.Wei
import org.hyperledger.besu.ethereum.api.util.DomainObjectDecodeUtils
import org.hyperledger.besu.tests.acceptance.dsl.account.Account
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Amount
import org.hyperledger.besu.tests.acceptance.dsl.blockchain.Blockchain
import org.hyperledger.besu.tests.acceptance.dsl.condition.admin.AdminConditions
import org.hyperledger.besu.tests.acceptance.dsl.condition.bft.BftConditions
import org.hyperledger.besu.tests.acceptance.dsl.condition.clique.CliqueConditions
import org.hyperledger.besu.tests.acceptance.dsl.condition.eth.EthConditions
import org.hyperledger.besu.tests.acceptance.dsl.condition.login.LoginConditions
import org.hyperledger.besu.tests.acceptance.dsl.condition.net.NetConditions
import org.hyperledger.besu.tests.acceptance.dsl.condition.perm.PermissioningConditions
import org.hyperledger.besu.tests.acceptance.dsl.condition.process.ExitedWithCode
import org.hyperledger.besu.tests.acceptance.dsl.condition.txpool.TxPoolConditions
import org.hyperledger.besu.tests.acceptance.dsl.condition.web3.Web3Conditions
import org.hyperledger.besu.tests.acceptance.dsl.contract.ContractVerifier
import org.hyperledger.besu.tests.acceptance.dsl.node.BesuNode
import org.hyperledger.besu.tests.acceptance.dsl.node.cluster.Cluster
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.BesuNodeFactory
import org.hyperledger.besu.tests.acceptance.dsl.node.configuration.permissioning.PermissionedNodeBuilder
import org.hyperledger.besu.tests.acceptance.dsl.transaction.account.AccountTransactions
import org.hyperledger.besu.tests.acceptance.dsl.transaction.admin.AdminTransactions
import org.hyperledger.besu.tests.acceptance.dsl.transaction.bft.BftTransactions
import org.hyperledger.besu.tests.acceptance.dsl.transaction.clique.CliqueTransactions
import org.hyperledger.besu.tests.acceptance.dsl.transaction.contract.ContractTransactions
import org.hyperledger.besu.tests.acceptance.dsl.transaction.eth.EthTransactions
import org.hyperledger.besu.tests.acceptance.dsl.transaction.miner.MinerTransactions
import org.hyperledger.besu.tests.acceptance.dsl.transaction.net.NetTransactions
import org.hyperledger.besu.tests.acceptance.dsl.transaction.perm.PermissioningTransactions
import org.hyperledger.besu.tests.acceptance.dsl.transaction.txpool.TxPoolTransactions
import org.hyperledger.besu.tests.acceptance.dsl.transaction.web3.Web3Transactions
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Tag
import org.junit.jupiter.api.extension.ExtendWith
import org.slf4j.LoggerFactory
import org.web3j.protocol.core.DefaultBlockParameter
import java.io.BufferedReader
import java.io.InputStreamReader
import java.lang.ProcessBuilder.Redirect
import java.math.BigInteger
import java.nio.charset.StandardCharsets.UTF_8
import java.util.concurrent.Executors
import java.util.concurrent.atomic.AtomicInteger

/** Base class for acceptance tests. */
@ExtendWith(AcceptanceTestBaseTestWatcher::class)
@Tag("AcceptanceTest")
abstract class AcceptanceTestBase {

  protected val accounts: Accounts
  protected val accountTransactions: AccountTransactions
  protected val admin: AdminConditions
  protected val adminTransactions: AdminTransactions
  protected val blockchain: Blockchain
  protected val clique: CliqueConditions
  protected val cliqueTransactions: CliqueTransactions
  protected val cluster: Cluster
  protected val contractVerifier: ContractVerifier
  protected val contractTransactions: ContractTransactions
  protected val eth: EthConditions
  protected val ethTransactions: EthTransactions
  protected val bftTransactions: BftTransactions
  protected val bft: BftConditions
  protected val login: LoginConditions
  protected val net: NetConditions
  protected val besu: BesuNodeFactory
  protected val perm: PermissioningConditions
  protected val permissionedNodeBuilder: PermissionedNodeBuilder
  protected val permissioningTransactions: PermissioningTransactions
  protected val minerTransactions: MinerTransactions
  protected val web3: Web3Conditions
  protected val txPoolConditions: TxPoolConditions
  protected val txPoolTransactions: TxPoolTransactions
  protected val exitedSuccessfully: ExitedWithCode
  protected lateinit var minerNode: BesuNode

  private val outputProcessorExecutor = Executors.newCachedThreadPool()

  companion object {
    private val log = LoggerFactory.getLogger(AcceptanceTestBase::class.java)
  }

  init {
    ethTransactions = EthTransactions()
    accounts = Accounts(ethTransactions)
    adminTransactions = AdminTransactions()
    cliqueTransactions = CliqueTransactions()
    bftTransactions = BftTransactions()
    accountTransactions = AccountTransactions(accounts)
    permissioningTransactions = PermissioningTransactions()
    contractTransactions = ContractTransactions()
    minerTransactions = MinerTransactions()

    blockchain = Blockchain(ethTransactions)
    clique = CliqueConditions(ethTransactions, cliqueTransactions)
    eth = EthConditions(ethTransactions)
    bft = BftConditions(bftTransactions)
    login = LoginConditions()
    net = NetConditions(NetTransactions())
    cluster = Cluster(net)
    perm = PermissioningConditions(permissioningTransactions)
    admin = AdminConditions(adminTransactions)
    web3 = Web3Conditions(Web3Transactions())
    besu = BesuNodeFactory()
    txPoolTransactions = TxPoolTransactions()
    txPoolConditions = TxPoolConditions(txPoolTransactions)
    contractVerifier = ContractVerifier(accounts.primaryBenefactor)
    permissionedNodeBuilder = PermissionedNodeBuilder()
    exitedSuccessfully = ExitedWithCode(0)
  }

  @AfterEach
  fun tearDownAcceptanceTestBase() {
    reportMemory()
    cluster.close()
  }

  /** Report memory usage after test execution. */
  fun reportMemory() {
    val os = System.getProperty("os.name")
    val command = when {
      os.contains("Linux") -> arrayOf("/usr/bin/top", "-n", "1", "-o", "%MEM", "-b", "-c", "-w", "180")
      os.contains("Mac") -> arrayOf("/usr/bin/top", "-l", "1", "-o", "mem", "-n", "20")
      else -> null
    }

    if (command != null) {
      log.info("Memory usage at end of test:")
      val processBuilder = ProcessBuilder(*command)
        .redirectErrorStream(true)
        .redirectInput(Redirect.INHERIT)

      try {
        val memInfoProcess = processBuilder.start()
        outputProcessorExecutor.execute { printOutput(memInfoProcess) }
        memInfoProcess.waitFor()
        log.debug("Memory info process exited with code {}", memInfoProcess.exitValue())
      } catch (e: Exception) {
        log.warn("Error running memory information process", e)
      }
    } else {
      log.info("Don't know how to report memory for OS {}", os)
    }
  }

  private fun printOutput(process: Process) {
    try {
      BufferedReader(InputStreamReader(process.inputStream, UTF_8)).use { reader ->
        reader.lineSequence().forEach { line ->
          log.info(line)
        }
      }
    } catch (e: Exception) {
      log.warn("Failed to read output from memory information process: ", e)
    }
  }

  protected fun createAccounts(numAccounts: Int, initialBalanceEther: Int): List<Account> {
    val newAccounts = (1..numAccounts)
      .map { accounts.createAccount("Account$it") }

    val senderAccount = accounts.primaryBenefactor
    val fundingAccNonce = AtomicInteger(
      minerNode.nodeRequests()
        .eth()
        .ethGetTransactionCount(
          senderAccount.address,
          DefaultBlockParameter.valueOf("LATEST"),
        )
        .send()
        .transactionCount
        .toInt(),
    )

    val founderBalance = ethTransactions.getBalance(senderAccount).execute(minerNode.nodeRequests())
    val transferGasPrice = Amount.wei(BigInteger.valueOf(20_000_000_000L)) // 20Gwei
    val requiredBalance = Wei.fromEth((initialBalanceEther * numAccounts).toLong())
      .asBigInteger
      .add(
        transferGasPrice.value
          .toBigInteger()
          .multiply(BigInteger.valueOf(21000))
          .multiply(BigInteger.valueOf(numAccounts.toLong())),
      )

    assertThat(founderBalance).isGreaterThanOrEqualTo(requiredBalance)

    val transfers = newAccounts.map { account ->
      val nonce = fundingAccNonce.getAndIncrement()
      val tx = accountTransactions.createTransfer(
        senderAccount,
        account,
        initialBalanceEther,
        BigInteger.valueOf(nonce.toLong()),
      )

      val decodedTx = DomainObjectDecodeUtils.decodeRawTransaction(tx.signedTransactionData())
      val nodeTxHash = tx.execute(minerNode.nodeRequests())
      assertThat(decodedTx.sender.toString()).isEqualTo(senderAccount.address)
      nodeTxHash
    }

    transfers.forEach { txHash ->
      minerNode.verify(eth.expectSuccessfulTransactionReceipt(txHash.toString()))
    }

    newAccounts.forEach { account ->
      minerNode.verify(account.balanceEquals(initialBalanceEther))
    }

    return newAccounts
  }
}
