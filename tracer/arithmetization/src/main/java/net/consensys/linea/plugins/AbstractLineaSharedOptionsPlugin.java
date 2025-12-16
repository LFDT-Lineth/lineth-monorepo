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

import java.util.Map;
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedCliOptions;
import net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration;
import net.consensys.linea.plugins.config.LineaTracerSharedCliOptions;
import net.consensys.linea.plugins.config.LineaTracerSharedConfiguration;

/** In this class we put CLI options that are shared with other plugins not defined here */
@Slf4j
public abstract class AbstractLineaSharedOptionsPlugin extends AbstractLineaOptionsPlugin {

  @Override
  public Map<String, LineaOptionsPluginConfiguration> getLineaPluginConfigMap() {
    final LineaL1L2BridgeSharedCliOptions l1L2BridgeCliOptions =
        LineaL1L2BridgeSharedCliOptions.create();
    final LineaTracerSharedCliOptions tracerSharedCliOptions = LineaTracerSharedCliOptions.create();

    return Map.of(
        LineaL1L2BridgeSharedCliOptions.CONFIG_KEY, l1L2BridgeCliOptions.asPluginConfig(),
        LineaTracerSharedCliOptions.CONFIG_KEY, tracerSharedCliOptions.asPluginConfig());
  }

  public LineaL1L2BridgeSharedConfiguration l1L2BridgeSharedConfiguration() {
    final LineaL1L2BridgeSharedConfiguration l2L1 =
        (LineaL1L2BridgeSharedConfiguration)
            getConfigurationByKey(LineaL1L2BridgeSharedCliOptions.CONFIG_KEY).optionsConfig();

    if (l2L1.equals(LineaL1L2BridgeSharedConfiguration.EMPTY)) {
      throw new IllegalStateException("L1L2 bridge configuration not provided.");
    }
    return l2L1;
  }

  public LineaTracerSharedConfiguration tracerSharedConfiguration() {
    return (LineaTracerSharedConfiguration)
        getConfigurationByKey(LineaTracerSharedCliOptions.CONFIG_KEY).optionsConfig();
  }

  @Override
  public void start() {
    super.start();
  }
}
