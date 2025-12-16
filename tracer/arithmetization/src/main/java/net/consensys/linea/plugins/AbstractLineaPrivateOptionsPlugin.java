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

package net.consensys.linea.plugins;

import java.util.HashMap;
import java.util.Map;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.plugins.rpc.RpcCliOptions;
import net.consensys.linea.plugins.rpc.RpcConfiguration;

/**
 * In this class we put CLI options that are shared with other plugins that are only part of the
 * tracer
 */
@Slf4j
public abstract class AbstractLineaPrivateOptionsPlugin extends AbstractLineaSharedOptionsPlugin {

  @Override
  public Map<String, LineaOptionsPluginConfiguration> getLineaPluginConfigMap() {
    final var configMap = new HashMap<>(super.getLineaPluginConfigMap());

    final RpcCliOptions rpcCliOptions = RpcCliOptions.create();
    configMap.put(RpcCliOptions.CONFIG_KEY, rpcCliOptions.asPluginConfig());

    return configMap;
  }

  protected RpcConfiguration rpcConfiguration() {
    return (RpcConfiguration) getConfigurationByKey(RpcCliOptions.CONFIG_KEY).optionsConfig();
  }

  @Override
  public void start() {
    super.start();
  }
}
