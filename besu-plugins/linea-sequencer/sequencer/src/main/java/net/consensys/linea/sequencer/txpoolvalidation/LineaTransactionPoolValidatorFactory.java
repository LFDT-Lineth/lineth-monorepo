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

package net.consensys.linea.sequencer.txpoolvalidation;

import java.util.Arrays;
import java.util.Map;
import java.util.Optional;
import java.util.Set;

import net.consensys.linea.config.LineaProfitabilityConfiguration;
import net.consensys.linea.config.LineaTransactionPoolValidatorConfiguration;
import net.consensys.linea.jsonrpc.JsonRpcManager;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.sequencer.txpoolvalidation.validators.AllowedAddressValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.CalldataValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.GasLimitValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.ProfitabilityValidator;
import net.consensys.linea.sequencer.txpoolvalidation.validators.SimulationValidator;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.plugin.services.BesuConfiguration;
import org.hyperledger.besu.plugin.services.BlockchainService;
import org.hyperledger.besu.plugin.services.TransactionSimulationService;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidator;
import org.hyperledger.besu.plugin.services.txvalidator.PluginTransactionPoolValidatorFactory;

/** Represents a factory for creating transaction pool validators. */
public class LineaTransactionPoolValidatorFactory implements PluginTransactionPoolValidatorFactory {

  private final BesuConfiguration besuConfiguration;
  private final BlockchainService blockchainService;
  private final TransactionSimulationService transactionSimulationService;
  private final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf;
  private final LineaProfitabilityConfiguration profitabilityConf;
  private final Set<Address> denied;
  private final Map<String, Integer> moduleLineLimitsMap;
  private final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration;
  private final Optional<JsonRpcManager> rejectedTxJsonRpcManager;

  public LineaTransactionPoolValidatorFactory(
      final BesuConfiguration besuConfiguration,
      final BlockchainService blockchainService,
      final TransactionSimulationService transactionSimulationService,
      final LineaTransactionPoolValidatorConfiguration txPoolValidatorConf,
      final LineaProfitabilityConfiguration profitabilityConf,
      final Set<Address> deniedAddresses,
      final Map<String, Integer> moduleLineLimitsMap,
      final LineaL1L2BridgeSharedConfiguration l1L2BridgeConfiguration,
      final Optional<JsonRpcManager> rejectedTxJsonRpcManager) {
    this.besuConfiguration = besuConfiguration;
    this.blockchainService = blockchainService;
    this.transactionSimulationService = transactionSimulationService;
    this.txPoolValidatorConf = txPoolValidatorConf;
    this.profitabilityConf = profitabilityConf;
    this.denied = deniedAddresses;
    this.moduleLineLimitsMap = moduleLineLimitsMap;
    this.l1L2BridgeConfiguration = l1L2BridgeConfiguration;
    this.rejectedTxJsonRpcManager = rejectedTxJsonRpcManager;
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
          new AllowedAddressValidator(denied),
          new GasLimitValidator(txPoolValidatorConf),
          new CalldataValidator(txPoolValidatorConf),
          new ProfitabilityValidator(besuConfiguration, blockchainService, profitabilityConf),
          new SimulationValidator(
              blockchainService,
              transactionSimulationService,
              txPoolValidatorConf,
              moduleLineLimitsMap,
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
}
