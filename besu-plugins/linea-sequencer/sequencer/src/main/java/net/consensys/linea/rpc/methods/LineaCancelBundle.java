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
package net.consensys.linea.rpc.methods;

import java.util.UUID;
import java.util.concurrent.atomic.AtomicInteger;

import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.bundles.BundlePoolService;
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.parameters.JsonRpcParameter;
import org.hyperledger.besu.plugin.services.exception.PluginRpcEndpointException;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;

@Slf4j
public class LineaCancelBundle {
  private static final AtomicInteger LOG_SEQUENCE = new AtomicInteger();
  private final JsonRpcParameter parameterParser = new JsonRpcParameter();
  private BundlePoolService bundlePool;

  public LineaCancelBundle init(BundlePoolService bundlePoolService) {
    this.bundlePool = bundlePoolService;
    return this;
  }

  public String getNamespace() {
    return "linea";
  }

  public String getName() {
    return "cancelBundle";
  }

  public Boolean execute(final PluginRpcRequest request) {
    // sequence id for correlating error messages in logs:
    final int logId = log.isDebugEnabled() ? LOG_SEQUENCE.incrementAndGet() : -1;
    try {
      final UUID replacementUUID = parameterParser.required(request.getParams(), 0, UUID.class);

      return bundlePool.remove(replacementUUID);

    } catch (final Exception e) {
      log.atError()
          .setMessage("[{}] failed to parse linea_cancelBundle request")
          .addArgument(logId)
          .setCause(e)
          .log();
      throw new PluginRpcEndpointException(
          new LineaSendBundle.LineaSendBundleError("malformed linea_cancelBundle json param"));
    }
  }
}
