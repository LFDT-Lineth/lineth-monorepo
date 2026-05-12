/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
 */

package net.consensys.linea.rpc.services;

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

  private LineaEstimateGas lineaEstimateGasMethod;

  /**
   * Register the RPC service.
   *
   * @param serviceManager the BesuContext to be used.
   */
  @Override
  public void doRegister(final ServiceManager serviceManager) {
    TransactionSimulationService transactionSimulationService =
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
  public void doStart() {
    if (l1L2BridgeSharedConfiguration().equals(LineaL1L2BridgeSharedConfiguration.TEST_DEFAULT)) {
      throw new IllegalArgumentException("L1L2 bridge settings have not been defined.");
    }
    lineaEstimateGasMethod.init(
        lineaRpcConfiguration(),
        transactionPoolValidatorConfiguration(),
        profitabilityConfiguration(),
        l1L2BridgeSharedConfiguration(),
        tracerConfiguration(),
        worldStateService,
        transactionProfitabilityCalculator);
  }
}
