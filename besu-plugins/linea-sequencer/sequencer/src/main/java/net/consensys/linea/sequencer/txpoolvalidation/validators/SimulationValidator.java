/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package net.consensys.linea.sequencer.txpoolvalidation.validators;

import static net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator.ModuleLineCountResult.MODULE_NOT_DEFINED;
import static net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator.ModuleLineCountResult.TX_MODULE_LINE_COUNT_OVERFLOW;
import static net.consensys.linea.zktracer.Fork.LONDON;
import static org.hyperledger.besu.plugin.services.TransactionSimulationService.SimulationParameters.ALLOW_FUTURE_NONCE;

import java.math.BigInteger;
import java.time.Instant;
import java.util.EnumSet;
import java.util.List;
import java.util.Optional;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.config.LineaTransactionPoolValidatorConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.jsonrpc.JsonRpcRequestBuilder;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.modulelimit.ModuleLimitsValidationResult;
import net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator;
import net.consensys.linea.zktracer.LineCountingTracer;
import net.consensys.linea.zktracer.ZkCounter;
import net.consensys.linea.zktracer.ZkTracer;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;
import org.hyperledger.besu.plugin.data.TransactionSimulationResult;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.TransactionSimulationService;
import org.hyperledger.besu.plugin.services.WorldStateService;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;

/**
 * Validator that checks if transaction simulation completes successfully, including line counting.
 * This check can be enabled/disabled independently for transactions received via API or P2P.
 */
@Slf4j
public class SimulationValidator implements PluginTransactionPoolValidator {
  private final BlockchainService blockchainService;
  private final WorldStateService worldStateService;
  private final TransactionSimulationService transactionSimulationService;
  private final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf;
  private final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration;
  private final LineaTracerConfiguration tracerConfiguration;
  private final Optional<JsonRpcManager> rejectedTxJsonRpcManager;

  public SimulationValidator(
      final BlockchainService blockchainService,
      final WorldStateService worldStateService,
      final TransactionSimulationService transactionSimulationService,
      final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf,
      final LineaTracerConfiguration tracerConfiguration,
      final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration,
      final Optional<JsonRpcManager> rejectedTxJsonRpcManager) {
    this.blockchainService = blockchainService;
    this.worldStateService = worldStateService;
    this.transactionSimulationService = transactionSimulationService;
    this.txPoolValidatorConf = txPoolValidatorConf;
    this.l1L2BridgeConfiguration = l1L2BridgeConfiguration;
    this.tracerConfiguration = tracerConfiguration;
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
          new ModuleLineCountValidator(tracerConfiguration.moduleLimitsMap());
      final var pendingBlockHeader = transactionSimulationService.simulatePendingBlockHeader();

      final var lineCountingTracer =
          createLineCountingTracer(pendingBlockHeader, blockchainService.getChainId().get());
      final var maybeSimulationResults =
          transactionSimulationService.simulate(
              transaction,
              Optional.empty(),
              pendingBlockHeader,
              lineCountingTracer,
              EnumSet.of(ALLOW_FUTURE_NONCE));

      ModuleLimitsValidationResult moduleLimitResult =
          moduleLineCountValidator.validate(lineCountingTracer.getModulesLineCount());

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

  private LineCountingTracer createLineCountingTracer(
      final ProcessableBlockHeader pendingBlockHeader, BigInteger chainId) {
    var lineCountingTracer =
        tracerConfiguration.isLimitless()
            ? new ZkCounter(l1L2BridgeConfiguration)
            : new ZkTracer(LONDON, l1L2BridgeConfiguration, chainId);
    lineCountingTracer.traceStartConflation(1L);
    lineCountingTracer.traceStartBlock(
        worldStateService.getWorldView(), pendingBlockHeader, pendingBlockHeader.getCoinbase());
    return lineCountingTracer;
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
