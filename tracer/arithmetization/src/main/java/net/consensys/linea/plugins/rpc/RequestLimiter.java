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

package net.consensys.linea.plugins.rpc;

import java.util.Optional;
import java.util.concurrent.Semaphore;
import java.util.function.Function;
import lombok.Builder;
import org.hyperledger.besu.ethereum.api.jsonrpc.internal.response.RpcErrorType;
import org.hyperledger.besu.plugin.services.exception.PluginRpcEndpointException;
import org.hyperledger.besu.plugin.services.rpc.PluginRpcRequest;
import org.hyperledger.besu.plugin.services.rpc.RpcMethodError;

public class RequestLimiter {

  private final Semaphore semaphore;
  private final RpcMethodError UNAVAILABLE_EXCEPTION = new UnavailableException();

  @Builder
  public RequestLimiter(int concurrentRequestsCount) {
    this.semaphore = new Semaphore(concurrentRequestsCount);
  }

  public <T extends PluginRpcRequest, R> R execute(T request, Function<T, R> processingFunc) {
    if (!semaphore.tryAcquire()) {
      throw new PluginRpcEndpointException(
          UNAVAILABLE_EXCEPTION,
          "Request still in progress, wait until available permits are available.");
    }

    try {
      return processingFunc.apply(request);
    } catch (Exception ex) {
      final Optional<String> message = Optional.ofNullable(ex.getMessage());
      final boolean blockNotFound =
          message.map(s -> s.toLowerCase().contains("block not found")).orElse(false);

      throw new PluginRpcEndpointException(
          blockNotFound ? RpcErrorType.BLOCK_NOT_FOUND : RpcErrorType.PLUGIN_INTERNAL_ERROR,
          ex.getMessage());
    } finally {
      semaphore.release();
    }
  }

  public int availableConcurrentRequestSlots() {
    return semaphore.availablePermits();
  }

  public boolean isNodeAtMaxCapacity() {
    return availableConcurrentRequestSlots() == 0;
  }
}
