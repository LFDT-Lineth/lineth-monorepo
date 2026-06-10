/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package linea.plugin.acc.test

import linea.plugin.acc.test.FakeChainSecurityPolicyTxValidatorPlugin.Companion.PLUGIN_NAME
import linea.plugin.acc.test.FakeChainSecurityPolicyTxValidatorPlugin.Companion.blockByHash
import linea.plugin.acc.test.FakeChainSecurityPolicyTxValidatorPlugin.Companion.blockBySender
import linea.plugin.acc.test.FakeChainSecurityPolicyTxValidatorPlugin.Companion.reset
import linea.security.ChainSecurityPolicy
import linea.txselection.LineaTransactionSelectionResult
import linea.txselection.LineaTransactionSelectionResult.chainSecurityRuleViolated
import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.plugin.BesuPlugin
import org.hyperledger.besu.plugin.ServiceManager
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader
import org.hyperledger.besu.plugin.data.TransactionProcessingResult
import org.hyperledger.besu.plugin.data.TransactionSelectionResult
import org.hyperledger.besu.plugin.services.TransactionSelectionService
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelectorFactory
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext
import java.util.concurrent.ConcurrentHashMap

/**
 * Acceptance-test fixture plugin that registers a [ChainSecurityPolicy] BesuService and a
 * [PluginTransactionSelector] that can block transactions by hash or sender address.
 *
 * Use [blockByHash] / [blockBySender] to populate the blocklists before a test, and [reset] to
 * clear them between tests.
 *
 * Activate by adding [PLUGIN_NAME] to the test's `requestedPlugins`.
 */
class FakeChainSecurityPolicyTxValidatorPlugin : BesuPlugin {

  private lateinit var transactionSelectionService: TransactionSelectionService
  private lateinit var serviceManager: ServiceManager

  override fun register(serviceManager: ServiceManager) {
    this.serviceManager = serviceManager
    transactionSelectionService =
      serviceManager
        .getService(TransactionSelectionService::class.java)
        .orElseThrow { RuntimeException("TransactionSelectionService not found in ServiceManager") }
  }

  override fun start() {
    val chainSecurityPolicy = serviceManager
      .getService(ChainSecurityPolicy::class.java)
      .orElseThrow { RuntimeException("ChainSecurityPolicy not found in ServiceManager") }
    transactionSelectionService.registerPluginTransactionSelectorFactory(
      object : PluginTransactionSelectorFactory {
        override fun create(
          pendingBlockHeader: ProcessableBlockHeader,
          selectorsStateManager: SelectorsStateManager,
        ): PluginTransactionSelector = BlocklistTransactionSelector(chainSecurityPolicy)
      },
    )
  }

  override fun stop() {}

  private class BlocklistTransactionSelector(
    val chainSecurityPolicy: ChainSecurityPolicy,
  ) : PluginTransactionSelector {

    override fun evaluateTransactionPreProcessing(
      evaluationContext: TransactionEvaluationContext,
    ): TransactionSelectionResult {
      val tx = evaluationContext.pendingTransaction.transaction
      if (chainSecurityPolicy.shallForceIncludeTransaction(evaluationContext)) {
        return TransactionSelectionResult.SELECTED
      }
      if (blockedHashes.contains(tx.hash.bytes)) return TX_BLOCKED_BY_SECURITY_POLICY
      if (blockedSenders.contains(tx.sender.bytes)) return TX_BLOCKED_BY_SECURITY_POLICY
      return TransactionSelectionResult.SELECTED
    }

    override fun evaluateTransactionPostProcessing(
      evaluationContext: TransactionEvaluationContext,
      processingResult: TransactionProcessingResult,
    ): TransactionSelectionResult = TransactionSelectionResult.SELECTED
  }

  companion object {
    const val PLUGIN_NAME = "FakeChainSecurityPolicyTxValidatorPlugin"

    val TX_BLOCKED_BY_SECURITY_POLICY: LineaTransactionSelectionResult =
      chainSecurityRuleViolated("Blocked by FakeChainSecurityPolicyPlugin")

    val blockedHashes: MutableSet<Bytes> = ConcurrentHashMap.newKeySet()
    val blockedSenders: MutableSet<Bytes> = ConcurrentHashMap.newKeySet()

    fun blockByHash(hash: Bytes) {
      blockedHashes.add(hash)
    }

    fun blockBySender(address: Bytes) {
      blockedSenders.add(address)
    }

    fun blockBySender(address: String) {
      blockedSenders.add(Bytes.fromHexString(address))
    }

    fun reset() {
      blockedHashes.clear()
      blockedSenders.clear()
    }
  }
}
