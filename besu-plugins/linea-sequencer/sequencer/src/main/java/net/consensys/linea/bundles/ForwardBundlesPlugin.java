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

package net.consensys.linea.bundles;

import java.net.URL;
import java.time.Duration;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;

import com.google.auto.service.AutoService;
import net.consensys.linea.AbstractLineaRequiredPlugin;
import net.consensys.linea.utils.PriorityThreadPoolExecutor;
import okhttp3.OkHttpClient;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;

@AutoService(BesuPlugin.class)
public class ForwardBundlesPlugin extends AbstractLineaRequiredPlugin {

  @Override
  public void doRegister(final ServiceManager serviceManager) {}

  @Override
  public void doStart() {
    final var config = bundleConfiguration();
    final var forwardUrls = config.forwardUrls();
    if (!forwardUrls.isEmpty()) {
      final var rpcClient = createRpcClient(config.timeoutMillis());
      final var retryScheduler = createRetryScheduler();
      forwardUrls.stream()
          .map(
              url ->
                  new BundleForwarder(
                      config,
                      createExecutor(url),
                      retryScheduler,
                      blockchainService,
                      rpcClient,
                      url))
          .peek(bundlePoolService::subscribeTransactionBundleAdded)
          .toList();
    }
  }

  private OkHttpClient createRpcClient(final int timeoutMillis) {
    return new OkHttpClient.Builder()
        .retryOnConnectionFailure(false)
        .callTimeout(Duration.ofMillis(timeoutMillis))
        .build();
  }

  private PriorityThreadPoolExecutor createExecutor(final URL recipientUrl) {
    return new PriorityThreadPoolExecutor(
        0,
        1,
        10,
        TimeUnit.MINUTES,
        Thread.ofVirtual().name("BundleForwarder[" + recipientUrl.toString() + "]", 0L).factory());
  }

  private ScheduledExecutorService createRetryScheduler() {
    return Executors.newSingleThreadScheduledExecutor(
        Thread.ofPlatform().name("BundleForwarderRetry", 0L).factory());
  }

  @Override
  public void stop() {
    // stop forwarders?
    super.stop();
  }
}
