/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.plugin.acc.test

import org.hyperledger.besu.datatypes.Hash
import org.hyperledger.besu.plugin.BesuPlugin
import org.hyperledger.besu.plugin.ServiceManager
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader
import org.hyperledger.besu.plugin.data.TransactionProcessingResult
import org.hyperledger.besu.plugin.data.TransactionSelectionResult
import org.hyperledger.besu.plugin.services.RpcEndpointService
import org.hyperledger.besu.plugin.services.TransactionSelectionService
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelector
import org.hyperledger.besu.plugin.services.txselection.PluginTransactionSelectorFactory
import org.hyperledger.besu.plugin.services.txselection.SelectorsStateManager
import org.hyperledger.besu.plugin.services.txselection.TransactionEvaluationContext
import java.util.concurrent.ConcurrentHashMap

/**
 * Acceptance-test-only plugin that observes transaction selection outcomes without affecting them.
 *
 * Registers a [PluginTransactionSelector] whose sole purpose is to record which transactions were
 * not selected and why. The recorded rejections can be queried via the companion object so that
 * acceptance tests can assert the exact rejection reason instead of relying on indirect signals
 * such as receipt absence.
 *
 * Besu's [TransactionSelectionService] maintains a list of factories (each registered by a
 * different plugin) and composes all resulting selectors via an internal
 * [AggregatedPluginTransactionSelector]. Because [RecordingTransactionSelector] always returns
 * [TransactionSelectionResult.SELECTED] from its evaluation methods, it does not influence which
 * transactions are chosen; it only observes the final outcome through
 * [PluginTransactionSelector.onTransactionNotSelected].
 *
 * Lives in main source so it is available on the classpath when the Besu node starts in CI (e.g.
 * when tests run in parallel forks or with different classpath ordering than local).
 */
class RecordingTransactionSelectorPlugin : BesuPlugin {

  private lateinit var transactionSelectionService: TransactionSelectionService
  private val rejections: ConcurrentHashMap<Hash, TransactionSelectionResult> = ConcurrentHashMap()

  override fun register(serviceManager: ServiceManager) {
    transactionSelectionService =
      serviceManager
        .getService(TransactionSelectionService::class.java)
        .orElseThrow { RuntimeException("TransactionSelectionService not found in ServiceManager") }

    serviceManager.getService(RpcEndpointService::class.java).ifPresent { rpcEndpointService ->
      rpcEndpointService.registerRPCEndpoint("test", "getRejectionReason") { request: PluginRpcRequest ->
        val txHashHex = request.params[0] as String
        val txHash = Hash.fromHexString(txHashHex)
        rejections[txHash]?.toString()
      }
    }
  }

  override fun start() {
    transactionSelectionService.registerPluginTransactionSelectorFactory(
      object : PluginTransactionSelectorFactory {
        override fun create(
          pendingBlockHeader: ProcessableBlockHeader,
          selectorsStateManager: SelectorsStateManager,
        ): PluginTransactionSelector = RecordingTransactionSelector()
      },
    )
  }

  override fun stop() {
  }

  private inner class RecordingTransactionSelector : PluginTransactionSelector {

    override fun evaluateTransactionPreProcessing(
      evaluationContext: TransactionEvaluationContext,
    ): TransactionSelectionResult = TransactionSelectionResult.SELECTED

    override fun evaluateTransactionPostProcessing(
      evaluationContext: TransactionEvaluationContext,
      processingResult: TransactionProcessingResult,
    ): TransactionSelectionResult {
      // Reaching post-processing means this pass evaluated the transaction all the way through
      // without any selector rejecting it, i.e. it is being selected for the block. Drop any
      // rejection recorded by an earlier pass: block building runs many selection passes over the
      // node's lifetime, and a transaction that is ultimately selected must not keep reporting a
      // stale rejection from a pass that was cancelled, timed out, or otherwise abandoned.
      //
      // This is the authoritative positive signal, and is what makes the recording self-correcting.
      // Enumerating every transient not-selected outcome in onTransactionNotSelected cannot be made
      // complete (Besu keeps adding/wrapping them); observing "it was selected" can.
      rejections.remove(evaluationContext.pendingTransaction.transaction.hash)
      return TransactionSelectionResult.SELECTED
    }

    override fun onTransactionNotSelected(
      evaluationContext: TransactionEvaluationContext,
      transactionSelectionResult: TransactionSelectionResult,
    ) {
      val txHash = evaluationContext.pendingTransaction.transaction.hash
      // Only substantive rejections are worth recording. Transient/scheduling outcomes
      // (SELECTION_CANCELLED, the *_TIMEOUT family, a penalized EXECUTION_INTERRUPTED) say
      // "block building gave up", not "this transaction was rejected on its merits", and Besu
      // itself warns they are not reliable: handleTransactionSelected reports
      // BLOCK_SELECTION_TIMEOUT for a transaction that already passed every check when the timeout
      // fires before commit.
      if (isTransientSchedulingOutcome(transactionSelectionResult)) {
        return
      }
      // A substantive rejection always wins, including over one recorded by an earlier pass.
      rejections[txHash] = transactionSelectionResult
    }

    private fun isTransientSchedulingOutcome(result: TransactionSelectionResult): Boolean =
      result in TRANSIENT_SCHEDULING_RESULTS ||
        (
          result.penalize() &&
            result.maybeInvalidReason().orElse(null) == EXECUTION_INTERRUPTED_REASON
          )
  }

  companion object {
    /**
     * The `TransactionInvalidReason.EXECUTION_INTERRUPTED` name, produced when a block build's time
     * budget interrupts a transaction's execution. Referenced by name because that enum lives in
     * Besu's internal (non-plugin-api) module.
     */
    private const val EXECUTION_INTERRUPTED_REASON: String = "EXECUTION_INTERRUPTED"

    /**
     * Outcomes that mean "block building ran out of time / was superseded", not "this transaction
     * was rejected on its merits".
     *
     * Besu wraps the underlying reason when a build times out, e.g. it reports
     * `BLOCK_SELECTION_TIMEOUT (original result INVALID_PENALIZED(EXECUTION_INTERRUPTED))`. The
     * wrapper does not carry `penalize()` or the invalid reason, so matching on those alone misses
     * it; the wrapper results have to be listed explicitly.
     */
    private val TRANSIENT_SCHEDULING_RESULTS: Set<TransactionSelectionResult> = setOf(
      TransactionSelectionResult.SELECTION_CANCELLED,
      TransactionSelectionResult.BLOCK_SELECTION_TIMEOUT,
      TransactionSelectionResult.BLOCK_SELECTION_TIMEOUT_INVALID_TX,
      TransactionSelectionResult.PLUGIN_SELECTION_TIMEOUT,
      TransactionSelectionResult.PLUGIN_SELECTION_TIMEOUT_INVALID_TX,
      TransactionSelectionResult.TX_EVALUATION_TOO_LONG,
      TransactionSelectionResult.INVALID_TX_EVALUATION_TOO_LONG,
    )
  }
}
