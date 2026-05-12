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
import net.consensys.linea.rpc.methods.LineaGetForcedTransactionInclusionStatus;
import net.consensys.linea.rpc.methods.LineaSendForcedRawTransaction;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;

/**
 * Plugin that registers the forced transaction RPC endpoints:
 *
 * <ul>
 *   <li>linea_sendForcedRawTransaction - Submit forced transactions with highest block building
 *       priority
 *   <li>linea_getForcedTransactionInclusionStatus - Query the inclusion status of a forced
 *       transaction
 * </ul>
 */
@Slf4j
@AutoService(BesuPlugin.class)
public class LineaForcedTransactionEndpointsPlugin extends AbstractLineaRequiredPlugin {

  private LineaSendForcedRawTransaction sendForcedRawTransactionMethod;
  private LineaGetForcedTransactionInclusionStatus getInclusionStatusMethod;

  @Override
  public void doRegister(final ServiceManager serviceManager) {
    sendForcedRawTransactionMethod = new LineaSendForcedRawTransaction();
    getInclusionStatusMethod = new LineaGetForcedTransactionInclusionStatus();

    rpcEndpointService.registerRPCEndpoint(
        sendForcedRawTransactionMethod.getNamespace(),
        sendForcedRawTransactionMethod.getName(),
        sendForcedRawTransactionMethod::execute);

    rpcEndpointService.registerRPCEndpoint(
        getInclusionStatusMethod.getNamespace(),
        getInclusionStatusMethod.getName(),
        getInclusionStatusMethod::execute);

    log.info("Registered forced transaction RPC endpoints");
  }

  @Override
  public void doStart() {
    sendForcedRawTransactionMethod.init(forcedTransactionPoolService);
    getInclusionStatusMethod.init(forcedTransactionPoolService);

    log.info("Started forced transaction endpoints plugin, pool service initialized");
  }
}
