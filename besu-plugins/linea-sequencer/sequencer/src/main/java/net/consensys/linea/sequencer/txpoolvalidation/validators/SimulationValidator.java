/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package net.consensys.linea.sequencer.txpoolvalidation.validators;

import static net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator.ModuleLineCountResult.MODULE_NOT_DEFINED;
import static net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator.ModuleLineCountResult.TX_MODULE_LINE_COUNT_OVERFLOW;
import static net.consensys.linea.zktracer.Fork.LONDON;

import java.math.BigInteger;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import java.util.Optional;

import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.config.LineaTransactionPoolValidatorConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.jsonrpc.JsonRpcRequestBuilder;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.modulelimit.ModuleLimitsValidationResult;
import net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator;
import net.consensys.linea.zktracer.ZkTracer;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.data.TransactionSimulationResult;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.TransactionSimulationService;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;

/**
 * Validator that checks if transaction simulation completes successfully, including line counting.
 * This check can be enabled/disabled independently for transactions received via API or P2P.
 */
@Slf4j
public class SimulationValidator implements PluginTransactionPoolValidator {
  private final BlockchainService blockchainService;
  private final TransactionSimulationService transactionSimulationService;
  private final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf;
  private final Map<String, Integer> moduleLineLimitsMap;
  private final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration;
  private final Optional<JsonRpcManager> rejectedTxJsonRpcManager;

  public SimulationValidator(
      final BlockchainService blockchainService,
      final TransactionSimulationService transactionSimulationService,
      final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf,
      final Map<String, Integer> moduleLineLimitsMap,
      final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration,
      final Optional<JsonRpcManager> rejectedTxJsonRpcManager) {
    this.blockchainService = blockchainService;
    this.transactionSimulationService = transactionSimulationService;
    this.txPoolValidatorConf = txPoolValidatorConf;
    this.moduleLineLimitsMap = moduleLineLimitsMap;
    this.l1L2BridgeConfiguration = l1L2BridgeConfiguration;
    this.rejectedTxJsonRpcManager = rejectedTxJsonRpcManager;
  }

  @Override
  public Optional<String> validateTransaction(
      final Transaction transaction, final boolean isLocal, final boolean hasPriority) {

    final boolean isLocalAndApiEnabled =
        isLocal && txPoolValidatorConf.txPoolSimulationCheckApiEnabled();
    final boolean isRemoteAndP2pEnabled =
        !isLocal && txPoolValidatorConf.txPoolSimulationCheckP2pEnabled();
    if (isRemoteAndP2pEnabled || isLocalAndApiEnabled) {
      log.atTrace()
          .setMessage(
              "Starting simulation validation for tx with hash={}, isLocal={}, hasPriority={}")
          .addArgument(transaction::getHash)
          .addArgument(isLocal)
          .addArgument(hasPriority)
          .log();

      final ModuleLineCountValidator moduleLineCountValidator =
          new ModuleLineCountValidator(moduleLineLimitsMap);
      final var pendingBlockHeader = transactionSimulationService.simulatePendingBlockHeader();

      final var zkTracer = createZkTracer(pendingBlockHeader, blockchainService.getChainId().get());
      final var maybeSimulationResults =
          transactionSimulationService.simulate(
              transaction, Optional.empty(), pendingBlockHeader, zkTracer, false, true);

      ModuleLimitsValidationResult moduleLimitResult =
          moduleLineCountValidator.validate(zkTracer.getModulesLineCount());

      logSimulationResult(
          transaction, isLocal, hasPriority, maybeSimulationResults, moduleLimitResult);

      if (moduleLimitResult.getResult() != ModuleLineCountValidator.ModuleLineCountResult.VALID) {
        final String reason = handleModuleOverLimit(transaction, moduleLimitResult);
        reportRejectedTransaction(transaction, reason);
        return Optional.of(reason);
      }

      if (maybeSimulationResults.isPresent()) {
        final var simulationResult = maybeSimulationResults.get();
        if (simulationResult.isInvalid()) {
          final String errMsg =
              "Invalid transaction"
                  + simulationResult.getInvalidReason().map(ir -> ": " + ir).orElse("");
          log.debug(errMsg);
          return Optional.of(errMsg);
        }
      }
    } else {
      log.atTrace()
          .setMessage(
              "Simulation validation not enabled for tx with hash={}, isLocal={}, hasPriority={}")
          .addArgument(transaction::getHash)
          .addArgument(isLocal)
          .addArgument(hasPriority)
          .log();
    }

    return Optional.empty();
  }

  private void reportRejectedTransaction(final Transaction transaction, final String reason) {
    rejectedTxJsonRpcManager.ifPresent(
        jsonRpcManager -> {
          final String jsonRpcCall =
              JsonRpcRequestBuilder.generateSaveRejectedTxJsonRpc(
                  jsonRpcManager.getNodeType(),
                  transaction,
                  Instant.now(),
                  Optional.empty(), // block number is not available
                  reason,
                  List.of());
          jsonRpcManager.submitNewJsonRpcCallAsync(jsonRpcCall);
        });
  }

  private void logSimulationResult(
      final Transaction transaction,
      final boolean isLocal,
      final boolean hasPriority,
      final Optional<TransactionSimulationResult> maybeSimulationResults,
      final ModuleLimitsValidationResult moduleLimitResult) {
    log.atTrace()
        .setMessage(
            "Result of simulation validation for tx with hash={}, isLocal={}, hasPriority={}, is {}, module line counts {}")
        .addArgument(transaction::getHash)
        .addArgument(isLocal)
        .addArgument(hasPriority)
        .addArgument(maybeSimulationResults)
        .addArgument(moduleLimitResult)
        .log();
  }

  private ZkTracer createZkTracer(
      final ProcessableBlockHeader pendingBlockHeader, BigInteger chainId) {
    var zkTracer = new ZkTracer(LONDON, l1L2BridgeConfiguration, chainId);
    zkTracer.traceStartConflation(1L);
    zkTracer.traceStartBlock(pendingBlockHeader, pendingBlockHeader.getCoinbase());
    return zkTracer;
  }

  private String handleModuleOverLimit(
      Transaction transaction, ModuleLimitsValidationResult moduleLimitResult) {
    if (moduleLimitResult.getResult() == MODULE_NOT_DEFINED) {
      String moduleNotDefinedMsg =
          String.format(
              "Module %s does not exist in the limits file.", moduleLimitResult.getModuleName());
      log.error(moduleNotDefinedMsg);
      return moduleNotDefinedMsg;
    }
    if (moduleLimitResult.getResult() == TX_MODULE_LINE_COUNT_OVERFLOW) {
      String txOverflowMsg =
          String.format(
              "Transaction %s line count for module %s=%s is above the limit %s",
              transaction.getHash(),
              moduleLimitResult.getModuleName(),
              moduleLimitResult.getModuleLineCount(),
              moduleLimitResult.getModuleLineLimit());
      log.warn(txOverflowMsg);
      log.trace("Transaction details: {}", transaction);
      return txOverflowMsg;
    }
    return "Internal Error: do not know what to do with result: " + moduleLimitResult.getResult();
  }
}
