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
package net.consensys.linea.config;

import com.google.common.base.MoreObjects;
import net.consensys.linea.plugins.LineaCliOptions;
import picocli.CommandLine;

public class LineaTracerCliOptions implements LineaCliOptions {
  public static final String CONFIG_KEY = "tracer-config";

  public static final String MODULE_LIMIT_FILE_PATH = "--plugin-linea-module-limit-file-path";
  public static final String DEFAULT_MODULE_LIMIT_FILE_PATH = "moduleLimitFile.toml";

  @CommandLine.Option(
      names = {MODULE_LIMIT_FILE_PATH},
      hidden = true,
      paramLabel = "<STRING>",
      description =
          "Path to the toml file containing the module limits (default: ${DEFAULT-VALUE})")
  private String moduleLimitFilePath = DEFAULT_MODULE_LIMIT_FILE_PATH;

  private LineaTracerCliOptions() {}

  /**
   * Create Linea cli options.
   *
   * @return the Linea cli options
   */
  public static LineaTracerCliOptions create() {
    return new LineaTracerCliOptions();
  }

  /**
   * Linea cli options from config.
   *
   * @param config the config
   * @return the Linea cli options
   */
  public static LineaTracerCliOptions fromConfig(final LineaTracerConfiguration config) {
    final LineaTracerCliOptions options = create();
    options.moduleLimitFilePath = config.moduleLimitsFilePath();
    return options;
  }

  /**
   * To domain object Linea factory configuration.
   *
   * @return the Linea factory configuration
   */
  @Override
  public LineaTracerConfiguration toDomainObject() {
    return LineaTracerConfiguration.builder().moduleLimitsFilePath(moduleLimitFilePath).build();
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(MODULE_LIMIT_FILE_PATH, moduleLimitFilePath)
        .toString();
  }
}
