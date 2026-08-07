/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package lineth.sequencer.txpoolvalidation;

import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.Set;
import lineth.bl.TransactionProfitabilityCalculator;
import lineth.config.LineaProfitabilityConfiguration;
import lineth.config.LineaTracerConfiguration;
import lineth.config.LineaTransactionPoolValidatorConfiguration;
import lineth.config.LineaTransactionValidatorConfiguration;
import lineth.jsonrpc.JsonRpcManager;
import lineth.sequencer.txpoolvalidation.validators.CalldataValidator;
import lineth.sequencer.txpoolvalidation.validators.DeniedAddressValidator;
import lineth.sequencer.txpoolvalidation.validators.GasLimitValidator;
import lineth.sequencer.txpoolvalidation.validators.PrecompileAddressValidator;
import lineth.sequencer.txpoolvalidation.validators.ProfitabilityValidator;
import lineth.sequencer.txpoolvalidation.validators.SimulationValidator;
import lineth.sequencer.txpoolvalidation.validators.TraceLineLimitValidator;
import lineth.sequencer.txpoolvalidation.validators.TransactionTypeValidator;
import lineth.sequencer.txselection.InvalidTransactionByLineCountCache;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.plugin.services.BesuConfiguration;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.TransactionSimulationService;
import org.hyperledger.besu.plugin.services.WorldStateService;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidatorFactory;

/** Represents a factory for creating transaction pool validators. */
@Slf4j
public class LineaTransactionPoolValidatorFactory implements PluginTransactionPoolValidatorFactory {

  private final BesuConfiguration besuConfiguration;
  private final BlockchainService blockchainService;
  private final WorldStateService worldStateService;
  private final TransactionSimulationService transactionSimulationService;
  private final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf;
  private final LineaTransactionValidatorConfiguration txValidatorConf;
  private final LineaProfitabilityConfiguration profitabilityConf;
  private final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration;
  private final LineaTracerConfiguration tracerConfiguration;
  private final Optional<JsonRpcManager> rejectedTxJsonRpcManager;
  private final InvalidTransactionByLineCountCache invalidTransactionByLineCountCache;
  private final TransactionProfitabilityCalculator transactionProfitabilityCalculator;
  private final Set<Address> deniedAddresses;
  private final boolean blockTransactionValidatorActive;

  public LineaTransactionPoolValidatorFactory(
      final BesuConfiguration besuConfiguration,
      final BlockchainService blockchainService,
      final WorldStateService worldStateService,
      final TransactionSimulationService transactionSimulationService,
      final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf,
      final LineaTransactionValidatorConfiguration txValidatorConf,
      final LineaProfitabilityConfiguration profitabilityConf,
      final LineaTracerConfiguration tracerConfiguration,
      final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration,
      final Optional<JsonRpcManager> rejectedTxJsonRpcManager,
      final InvalidTransactionByLineCountCache invalidTransactionByLineCountCache,
      final TransactionProfitabilityCalculator transactionProfitabilityCalculator,
      final Set<Address> deniedAddresses,
      final boolean blockTransactionValidatorActive) {
    this.besuConfiguration = besuConfiguration;
    this.blockchainService = blockchainService;
    this.worldStateService = worldStateService;
    this.transactionSimulationService = transactionSimulationService;
    this.txPoolValidatorConf = txPoolValidatorConf;
    this.txValidatorConf = txValidatorConf;
    this.profitabilityConf = profitabilityConf;
    this.tracerConfiguration = tracerConfiguration;
    this.l1L2BridgeConfiguration = l1L2BridgeConfiguration;
    this.rejectedTxJsonRpcManager = rejectedTxJsonRpcManager;
    this.invalidTransactionByLineCountCache = invalidTransactionByLineCountCache;
    this.transactionProfitabilityCalculator = transactionProfitabilityCalculator;
    this.deniedAddresses = deniedAddresses;
    this.blockTransactionValidatorActive = blockTransactionValidatorActive;
  }

  /**
   * Creates a new transaction pool validator, that simply calls in sequence all the actual
   * validators, in a fail-fast mode.
   *
   * @return the new transaction pool validator
   */
  @Override
  public PluginTransactionPoolValidator createTransactionValidator() {
    final List<PluginTransactionPoolValidator> validators = createValidators();

    return (transaction, isLocal, hasPriority) ->
        validators.stream()
            .map(v -> v.validateTransaction(transaction, isLocal, hasPriority))
            .filter(Optional::isPresent)
            .findFirst()
            .map(Optional::get);
  }

  /**
   * Builds the ordered list of pool validators, run in sequence in fail-fast mode.
   *
   * <p>The transaction-type validator is omitted when {@link
   * lineth.sequencer.txvalidation.LineaBlockTransactionValidatorPlugin} is active: its
   * protocol-level rule already validates transaction types at pool admission, so adding the
   * pool-level validator here would validate types twice.
   *
   * @return the validators to run, in order
   */
  List<PluginTransactionPoolValidator> createValidators() {
    final List<PluginTransactionPoolValidator> validators = new ArrayList<>();
    if (!blockTransactionValidatorActive) {
      validators.add(new TransactionTypeValidator(txValidatorConf));
    }
    validators.add(new TraceLineLimitValidator(invalidTransactionByLineCountCache));
    validators.add(new DeniedAddressValidator(deniedAddresses));
    validators.add(new PrecompileAddressValidator());
    validators.add(new GasLimitValidator(txPoolValidatorConf.maxTxGasLimit()));

    if (txPoolValidatorConf.maxTxCalldataSize() != null) {
      validators.add(new CalldataValidator(txPoolValidatorConf.maxTxCalldataSize()));
    }

    validators.add(
        new ProfitabilityValidator(
            besuConfiguration,
            blockchainService,
            profitabilityConf,
            transactionProfitabilityCalculator));
    validators.add(
        new SimulationValidator(
            blockchainService,
            worldStateService,
            transactionSimulationService,
            txPoolValidatorConf,
            tracerConfiguration,
            l1L2BridgeConfiguration,
            rejectedTxJsonRpcManager));

    return validators;
  }
}
