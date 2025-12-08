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

package net.consensys.linea.plugins.readiness;

import com.google.common.base.MoreObjects;
import net.consensys.linea.plugins.LineaCliOptions;
import picocli.CommandLine;

public class TracerReadinessCliOptions implements LineaCliOptions {
  public static final String CONFIG_KEY = "tracer-readiness";

  static final String TRACER_READINESS_SERVER_HOST = "--plugin-linea-tracer-readiness-server-host";
  static final String TRACER_READINESS_SERVER_PORT = "--plugin-linea-tracer-readiness-server-port";
  static final String TRACER_READINESS_MAX_BLOCKS_BEHIND =
      "--plugin-linea-tracer-readiness-max-blocks-behind";

  @CommandLine.Option(
      required = true,
      names = {TRACER_READINESS_SERVER_PORT},
      hidden = true,
      paramLabel = "<SERVER_PORT>",
      description = "HTTP server port for tracer readiness plugin")
  private int serverPort = 8548;

  @CommandLine.Option(
      required = true,
      names = {TRACER_READINESS_SERVER_HOST},
      hidden = true,
      paramLabel = "<SERVER_HOST>",
      description = "HTTP server host for tracer readiness plugin")
  private String serverHost = "0.0.0.0";

  @CommandLine.Option(
      required = true,
      names = {TRACER_READINESS_MAX_BLOCKS_BEHIND},
      hidden = true,
      paramLabel = "<BLOCK_COUNT>",
      description =
          "Maximum number of block behind the head of the chain to treat negligible in state sync")
  private int maxBlocksBehind;

  private TracerReadinessCliOptions() {}

  /**
   * Create Linea cli options.
   *
   * @return the Linea cli options
   */
  public static TracerReadinessCliOptions create() {
    return new TracerReadinessCliOptions();
  }

  /**
   * Linea cli options from config.
   *
   * @param config the config
   * @return the Linea cli options
   */
  static TracerReadinessCliOptions fromConfig(final TracerReadinessConfiguration config) {
    final TracerReadinessCliOptions options = create();
    options.serverHost = config.serverHost();
    options.serverPort = config.serverPort();
    options.maxBlocksBehind = config.maxBlocksBehind();
    return options;
  }

  /**
   * To domain object Linea factory configuration.
   *
   * @return the Linea factory configuration
   */
  @Override
  public TracerReadinessConfiguration toDomainObject() {
    return TracerReadinessConfiguration.builder()
        .serverHost(serverHost)
        .serverPort(serverPort)
        .maxBlocksBehind(maxBlocksBehind)
        .build();
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(TRACER_READINESS_SERVER_HOST, serverHost)
        .add(TRACER_READINESS_SERVER_PORT, serverPort)
        .add(TRACER_READINESS_MAX_BLOCKS_BEHIND, maxBlocksBehind)
        .toString();
  }
}
