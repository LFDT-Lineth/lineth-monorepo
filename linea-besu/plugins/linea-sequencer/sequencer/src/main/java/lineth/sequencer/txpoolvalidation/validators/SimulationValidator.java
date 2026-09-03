/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */
package lineth.sequencer.txpoolvalidation.validators;

import static lineth.sequencer.modulelimit.ModuleLineCountValidator.ModuleLineCountResult.MODULE_NOT_DEFINED;
import static lineth.sequencer.modulelimit.ModuleLineCountValidator.ModuleLineCountResult.TX_MODULE_LINE_COUNT_OVERFLOW;
import static org.hyperledger.besu.plugin.services.TransactionSimulationService.SimulationParameters.ALLOW_FUTURE_NONCE;

import java.time.Instant;
import java.util.EnumSet;
import java.util.List;
import java.util.Optional;
import lineth.config.LineaTracerConfiguration;
import lineth.config.LineaTransactionPoolValidatorConfiguration;
import lineth.jsonrpc.JsonRpcManager;
import lineth.jsonrpc.JsonRpcRequestBuilder;
import lineth.sequencer.modulelimit.ModuleLimitsValidationResult;
import lineth.sequencer.modulelimit.ModuleLineCountValidator;
import lineth.sequencer.tracing.LineaTracerFactory;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.zktracer.LineCountingTracer;
import net.consensys.linea.zktracer.exceptions.TracingExceptions;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.tracing.OperationTracer;
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
  private final WorldStateService worldStateService;
  private final TransactionSimulationService transactionSimulationService;
  private final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf;
  private final LineaTracerConfiguration tracerConfiguration;
  private final LineaTracerFactory tracerFactory;
  private final Optional<JsonRpcManager> rejectedTxJsonRpcManager;

  public SimulationValidator(
      final BlockchainService blockchainService,
      final WorldStateService worldStateService,
      final TransactionSimulationService transactionSimulationService,
      final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf,
      final LineaTracerConfiguration tracerConfiguration,
      final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration,
      final Optional<JsonRpcManager> rejectedTxJsonRpcManager) {
    this.worldStateService = worldStateService;
    this.transactionSimulationService = transactionSimulationService;
    this.txPoolValidatorConf = txPoolValidatorConf;
    this.tracerConfiguration = tracerConfiguration;
    this.tracerFactory =
        LineaTracerFactory.fromBlockchainService(
            blockchainService, tracerConfiguration, l1L2BridgeConfiguration);
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
      final var lineCountingTracer = tracerFactory.create(pendingBlockHeader.getTimestamp());
      lineCountingTracer.ifPresent(tracer -> initializeTracer(tracer, pendingBlockHeader));
      final OperationTracer operationTracer =
          lineCountingTracer
              .<OperationTracer>map(tracer -> tracer)
              .orElse(OperationTracer.NO_TRACING);
      final var maybeSimulationResults =
          transactionSimulationService.simulate(
              transaction,
              Optional.empty(),
              pendingBlockHeader,
              operationTracer,
              EnumSet.of(ALLOW_FUTURE_NONCE));

      final Optional<ModuleLimitsValidationResult> moduleLimitResult;
      if (lineCountingTracer.isPresent()) {
        try {
          moduleLimitResult =
              Optional.of(
                  moduleLineCountValidator.validate(
                      lineCountingTracer.get().getModulesLineCount()));
        } catch (TracingExceptions e) {
          log.warn(
              "Tracer failed during simulation of tx {}: {}",
              transaction.getHash(),
              e.getMessage());
          return Optional.of("Tracer error during simulation: " + e.getMessage());
        }
      } else {
        moduleLimitResult = Optional.empty();
      }

      logSimulationResult(
          transaction, isLocal, hasPriority, maybeSimulationResults, moduleLimitResult);

      if (moduleLimitResult.isPresent()
          && moduleLimitResult.get().getResult()
              != ModuleLineCountValidator.ModuleLineCountResult.VALID) {
        final String reason = handleModuleOverLimit(transaction, moduleLimitResult.orElseThrow());
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
      final Optional<ModuleLimitsValidationResult> moduleLimitResult) {
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

  private void initializeTracer(
      final LineCountingTracer lineCountingTracer,
      final ProcessableBlockHeader pendingBlockHeader) {
    lineCountingTracer.traceStartConflation(1L);
    lineCountingTracer.traceStartBlock(
        worldStateService.getWorldView(), pendingBlockHeader, pendingBlockHeader.getCoinbase());
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
