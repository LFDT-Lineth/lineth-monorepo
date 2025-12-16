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

package net.consensys.linea.plugins.rpc.tracegeneration;

import com.google.auto.service.AutoService;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.HashMap;
import java.util.Map;
import java.util.Optional;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.plugins.AbstractLineaPrivateOptionsPlugin;
import net.consensys.linea.plugins.BesuServiceProvider;
import net.consensys.linea.plugins.LineaOptionsPluginConfiguration;
import net.consensys.linea.plugins.exception.TraceOutputException;
import net.consensys.linea.plugins.rpc.RequestLimiter;
import net.consensys.linea.plugins.rpc.RequestLimiterDispatcher;
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.RpcEndpointService;

/**
 * Registers RPC endpoints .This class provides an RPC endpoint named
 * 'generateConflatedTracesToFileV0' under the 'rollup' namespace. It uses {@link
 * GenerateConflatedTracesV2} to generate conflated file traces. This class provides an RPC endpoint
 * named 'generateConflatedTracesToFileV0' under the 'rollup' namespace.
 */
@Slf4j
@AutoService(BesuPlugin.class)
public class TracesEndpointServicePlugin extends AbstractLineaPrivateOptionsPlugin {
  private ServiceManager besuContext;
  private RpcEndpointService rpcEndpointService;

  @Override
  public Map<String, LineaOptionsPluginConfiguration> getLineaPluginConfigMap() {
    final var configMap = new HashMap<>(super.getLineaPluginConfigMap());
    final TracesEndpointCliOptions tracesEndpointCliOptions = TracesEndpointCliOptions.create();
    configMap.put(TracesEndpointCliOptions.CONFIG_KEY, tracesEndpointCliOptions.asPluginConfig());
    return configMap;
  }

  /**
   * Register the RPC service.
   *
   * @param context the BesuContext to be used.
   */
  @Override
  public void register(final ServiceManager context) {
    super.register(context);
    besuContext = context;
    rpcEndpointService = BesuServiceProvider.getRpcEndpointService(context);
  }

  @Override
  public void beforeExternalServices() {
    super.beforeExternalServices();

    final TracesEndpointConfiguration endpointConfiguration =
        (TracesEndpointConfiguration)
            getConfigurationByKey(TracesEndpointCliOptions.CONFIG_KEY).optionsConfig();

    final Optional<Path> tracesOutputPath =
        initTracesOutputPath(endpointConfiguration.tracesOutputPath());
    if (tracesOutputPath.isEmpty()) {
      throw new TraceOutputException(
          "Traces output path is null, please specify a valid path with %s CLI option or in a toml config file"
              .formatted(TracesEndpointCliOptions.CONFLATED_TRACE_GENERATION_TRACES_OUTPUT_PATH));
    }

    RequestLimiterDispatcher.setLimiterIfMissing(
        RequestLimiterDispatcher.SINGLE_INSTANCE_REQUEST_LIMITER_KEY,
        rpcConfiguration().concurrentRequestsLimit());
    final RequestLimiter reqLimiter =
        RequestLimiterDispatcher.getLimiter(
            RequestLimiterDispatcher.SINGLE_INSTANCE_REQUEST_LIMITER_KEY);

    final GenerateConflatedTracesV2 method =
        new GenerateConflatedTracesV2(
            besuContext, reqLimiter, endpointConfiguration, l1L2BridgeSharedConfiguration());

    createAndRegister(method, rpcEndpointService);
  }

  private Optional<Path> initTracesOutputPath(final String tracesOutputPathOption) {
    final Optional<Path> tracesOutputPath = Optional.of(Paths.get(tracesOutputPathOption));

    try {
      Files.createDirectories(tracesOutputPath.get());
    } catch (IOException e) {
      throw new TraceOutputException(e.getMessage());
    }

    return tracesOutputPath;
  }

  /**
   * Create and register the RPC service.
   *
   * @param method the RollupGenerateConflatedTracesToFileV0 method to be used.
   * @param rpcEndpointService the RpcEndpointService to be registered.
   */
  private void createAndRegister(
      final GenerateConflatedTracesV2 method, final RpcEndpointService rpcEndpointService) {
    rpcEndpointService.registerRPCEndpoint(
        method.getNamespace(), method.getName(), method::execute);
  }

  /** Start the RPC service. This method loads the OpCodes. */
  @Override
  public void start() {}
}
