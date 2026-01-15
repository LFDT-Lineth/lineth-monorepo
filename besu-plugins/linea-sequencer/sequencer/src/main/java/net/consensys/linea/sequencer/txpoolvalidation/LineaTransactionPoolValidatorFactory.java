/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.sequencer.txpoolvalidation;

import java.util.Arrays;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.atomic.AtomicReference;
import net.consensys.linea.bl.TransactionProfitabilityCalculator;
import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.config.LineaTracerConfiguration;
import net.consensys.linea.config.LineaTransactionPoolValidatorConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.txpoolvalidation.validators.AllowedAddressValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.CalldataValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.GasLimitValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.ProfitabilityValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.SimulationValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.TraceLineLimitValidator;
import net.consensys.linea.sequencer.txselection.InvalidTransactionByLineCountCache;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.plugin.services.BesuConfiguration;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.TransactionSimulationService;
import org.hyperledger.besu.plugin.services.WorldStateService;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidatorFactory;

/** Represents a factory for creating transaction pool validators. */
public class LineaTransactionPoolValidatorFactory implements PluginTransactionPoolValidatorFactory {

  private final BesuConfiguration besuConfiguration;
  private final BlockchainService blockchainService;
  private final WorldStateService worldStateService;
  private final TransactionSimulationService transactionSimulationService;
  private final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf;
  private final LineaProfitabilityConfiguration profitabilityConf;
  private final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration;
  private final LineaTracerConfiguration tracerConfiguration;
  private final Optional<JsonRpcManager> rejectedTxJsonRpcManager;
  private final InvalidTransactionByLineCountCache invalidTransactionByLineCountCache;
  private final TransactionProfitabilityCalculator transactionProfitabilityCalculator;

  private final AtomicReference<Set<Address>> deniedAddresses;

  public LineaTransactionPoolValidatorFactory(
      final BesuConfiguration besuConfiguration,
      final BlockchainService blockchainService,
      final WorldStateService worldStateService,
      final TransactionSimulationService transactionSimulationService,
      final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf,
      final LineaProfitabilityConfiguration profitabilityConf,
      final LineaTracerConfiguration tracerConfiguration,
      final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration,
      final Optional<JsonRpcManager> rejectedTxJsonRpcManager,
      final InvalidTransactionByLineCountCache invalidTransactionByLineCountCache,
      final TransactionProfitabilityCalculator transactionProfitabilityCalculator) {
    this.besuConfiguration = besuConfiguration;
    this.blockchainService = blockchainService;
    this.worldStateService = worldStateService;
    this.transactionSimulationService = transactionSimulationService;
    this.txPoolValidatorConf = txPoolValidatorConf;
    this.profitabilityConf = profitabilityConf;
    this.tracerConfiguration = tracerConfiguration;
    this.l1L2BridgeConfiguration = l1L2BridgeConfiguration;
    this.rejectedTxJsonRpcManager = rejectedTxJsonRpcManager;
    this.invalidTransactionByLineCountCache = invalidTransactionByLineCountCache;
    this.transactionProfitabilityCalculator = transactionProfitabilityCalculator;

    this.deniedAddresses = new AtomicReference<>(txPoolValidatorConf.deniedAddresses());
  }

  /**
   * Creates a new transaction pool validator, that simply calls in sequence all the actual
   * validators, in a fail-fast mode.
   *
   * @return the new transaction pool validator
   */
  @Override
  public PluginTransactionPoolValidator createTransactionValidator() {
    final var validators =
        new PluginTransactionPoolValidator[] {
          new TraceLineLimitValidator(invalidTransactionByLineCountCache),
          new AllowedAddressValidator(deniedAddresses),
          new GasLimitValidator(txPoolValidatorConf.maxTxGasLimit()),
          new CalldataValidator(txPoolValidatorConf.maxTxCalldataSize()),
          new ProfitabilityValidator(
              besuConfiguration,
              blockchainService,
              profitabilityConf,
              transactionProfitabilityCalculator),
          new SimulationValidator(
              blockchainService,
              worldStateService,
              transactionSimulationService,
              txPoolValidatorConf,
              tracerConfiguration,
              l1L2BridgeConfiguration,
              rejectedTxJsonRpcManager)
        };

    return (transaction, isLocal, hasPriority) ->
        Arrays.stream(validators)
            .map(v -> v.validateTransaction(transaction, isLocal, hasPriority))
            .filter(Optional::isPresent)
            .findFirst()
            .map(Optional::get);
  }

  public void setDeniedAddresses(final Set<Address> deniedAddresses) {
    this.deniedAddresses.set(deniedAddresses);
  }
}
