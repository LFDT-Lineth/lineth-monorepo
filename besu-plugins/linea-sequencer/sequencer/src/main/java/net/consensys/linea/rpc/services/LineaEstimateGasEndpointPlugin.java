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

package net.consensys.linea.rpc.services;

import static net.consensys.linea.sequencer.modulelimit.ModuleLineCountValidator.createLimitModules;

import com.google.auto.service.AutoService;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.AbstractLineaRequiredPlugin;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.rpc.methods.LineaEstimateGas;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.TransactionSimulationService;

/** Registers RPC endpoints. This class provides RPC endpoints under the 'linea' namespace. */
@AutoService(BesuPlugin.class)
@Slf4j
public class LineaEstimateGasEndpointPlugin extends AbstractLineaRequiredPlugin {

  private TransactionSimulationService transactionSimulationService;
  private LineaEstimateGas lineaEstimateGasMethod;

  /**
   * Register the RPC service.
   *
   * @param serviceManager the BesuContext to be used.
   */
  @Override
  public void doRegister(final ServiceManager serviceManager) {
    transactionSimulationService =
        serviceManager
            .getService(TransactionSimulationService.class)
            .orElseThrow(
                () ->
                    new RuntimeException(
                        "Failed to obtain TransactionSimulatorService from the ServiceManager."));

    lineaEstimateGasMethod =
        new LineaEstimateGas(
            besuConfiguration, transactionSimulationService, blockchainService, rpcEndpointService);

    rpcEndpointService.registerRPCEndpoint(
        lineaEstimateGasMethod.getNamespace(),
        lineaEstimateGasMethod.getName(),
        lineaEstimateGasMethod::execute);
  }

  @Override
  public void beforeExternalServices() {
    super.beforeExternalServices();
    lineaEstimateGasMethod.init(
        lineaRpcConfiguration(),
        transactionPoolValidatorConfiguration(),
        profitabilityConfiguration(),
        createLimitModules(tracerConfiguration()),
        l1L2BridgeSharedConfiguration());
  }

  @Override
  public void doStart() {
    if (l1L2BridgeSharedConfiguration().equals(LineaL1L2BridgeSharedConfiguration.TEST_DEFAULT)) {
      throw new IllegalArgumentException("L1L2 bridge settings have not been defined.");
    }
  }
}
