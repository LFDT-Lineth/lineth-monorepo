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
