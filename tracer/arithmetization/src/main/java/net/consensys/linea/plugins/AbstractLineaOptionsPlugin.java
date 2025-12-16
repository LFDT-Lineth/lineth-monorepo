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
import org.hyperledger.besu.plugin.BesuPlugin;
import org.hyperledger.besu.plugin.ServiceManager;
import org.hyperledger.besu.plugin.services.PicoCLIOptions;

/**
 * In this class we put CLI options that are private to plugins in this repo.
 *
 * <p>For the moment is just a placeholder since there are no private options
 */
@Slf4j
public abstract class AbstractLineaOptionsPlugin implements BesuPlugin {
  private static final String CLI_OPTIONS_PREFIX = "linea";
  private static final Map<String, LineaOptionsPluginConfiguration> LINEA_PLUGIN_CONFIG_MAP =
      new HashMap<>();

  protected abstract Map<String, LineaOptionsPluginConfiguration> getLineaPluginConfigMap();

  protected LineaOptionsPluginConfiguration getConfigurationByKey(final String key) {
    return LINEA_PLUGIN_CONFIG_MAP.get(key);
  }

  @Override
  public synchronized void register(final ServiceManager context) {
    final PicoCLIOptions cmdlineOptions = BesuServiceProvider.getPicoCLIOptionsService(context);

    getLineaPluginConfigMap()
        .forEach(
            (key, pluginConfiguration) -> {
              if (!LINEA_PLUGIN_CONFIG_MAP.containsKey(key)) {
                LINEA_PLUGIN_CONFIG_MAP.put(key, pluginConfiguration);
                cmdlineOptions.addPicoCLIOptions(
                    CLI_OPTIONS_PREFIX, pluginConfiguration.cliOptions());
              }
            });
  }

  @Override
  public void beforeExternalServices() {
    // TODO: find a way to do this only once and check fro that.
    LINEA_PLUGIN_CONFIG_MAP.forEach((opts, config) -> config.initOptionsConfig());

    LINEA_PLUGIN_CONFIG_MAP.forEach(
        (opts, config) -> {
          log.debug(
              "Configured plugin {} with configuration: {}", getName(), config.optionsConfig());
        });
  }

  @Override
  public void start() {}

  @Override
  public void stop() {
    LINEA_PLUGIN_CONFIG_MAP.clear();
  }
}
