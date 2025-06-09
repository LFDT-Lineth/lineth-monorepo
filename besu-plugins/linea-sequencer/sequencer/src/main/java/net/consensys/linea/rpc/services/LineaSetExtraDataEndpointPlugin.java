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
import net.consensys.linea.extradata.LineaExtraDataHandler;
import net.consensys.linea.rpc.methods.LineaSetExtraData;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;

/** Registers RPC endpoints. This class provides RPC endpoints under the 'linea' namespace. */
@AutoService(BesuPlugin.class)
@Slf4j
public class LineaSetExtraDataEndpointPlugin extends AbstractLineaRequiredPlugin {
  private LineaSetExtraData lineaSetExtraDataMethod;

  /**
   * Register the RPC service.
   *
   * @param serviceManager the ServiceManager to be used.
   */
  @Override
  public void doRegister(final ServiceManager serviceManager) {

    lineaSetExtraDataMethod = new LineaSetExtraData(rpcEndpointService);

    rpcEndpointService.registerRPCEndpoint(
        lineaSetExtraDataMethod.getNamespace(),
        lineaSetExtraDataMethod.getName(),
        lineaSetExtraDataMethod::execute);
  }

  @Override
  public void beforeExternalServices() {
    super.beforeExternalServices();
    lineaSetExtraDataMethod.init(
        new LineaExtraDataHandler(rpcEndpointService, profitabilityConfiguration()));
  }

  @Override
  public void doStart() {
    // no-op
  }
}
