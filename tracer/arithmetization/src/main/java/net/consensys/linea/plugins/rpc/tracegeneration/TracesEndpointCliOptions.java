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

import com.google.common.base.MoreObjects;
import net.consensys.linea.plugins.LineaCliOptions;
import picocli.CommandLine;

public class TracesEndpointCliOptions implements LineaCliOptions {

  static final String CONFIG_KEY = "traces-endpoint-config";

  static final String CONFLATED_TRACE_GENERATION_TRACES_OUTPUT_PATH =
      "--plugin-linea-conflated-trace-generation-traces-output-path";

  static final String CACHING = "--plugin-linea-rpc-caching";

  static final String CONFLATED_TRACE_GENERATION_TRACE_COMPRESSION =
      "--plugin-linea-conflated-trace-generation-trace-compression";

  static final String CONFLATED_TRACE_GENERATION_TRACE_FILE_VERSION =
      "--plugin-linea-conflated-trace-generation-trace-file-version";

  @CommandLine.Option(
      required = true,
      names = {CONFLATED_TRACE_GENERATION_TRACES_OUTPUT_PATH},
      hidden = true,
      paramLabel = "<PATH>",
      description = "Path to where traces will be written")
  private String tracesOutputPath = null;

  @CommandLine.Option(
      names = {CACHING},
      hidden = true,
      paramLabel = "<CACHING>",
      description = "Reuse existing trace files when available")
  private boolean caching = true;

  @CommandLine.Option(
      names = {CONFLATED_TRACE_GENERATION_TRACE_COMPRESSION},
      hidden = true,
      paramLabel = "<BOOL>",
      description = "Specify whether or not to employ trace compression")
  private boolean traceCompression = true;

  @CommandLine.Option(
      names = {CONFLATED_TRACE_GENERATION_TRACE_FILE_VERSION},
      hidden = true,
      paramLabel = "<INT>",
      description = "Specify major version of trace file format to use (e.g. v1 or v2)")
  private int traceFileVersion = 2;

  private TracesEndpointCliOptions() {}

  /**
   * Create Linea cli options.
   *
   * @return the Linea cli options
   */
  public static TracesEndpointCliOptions create() {
    return new TracesEndpointCliOptions();
  }

  /**
   * Linea cli options from config.
   *
   * @param config the config
   * @return the Linea cli options
   */
  static TracesEndpointCliOptions fromConfig(final TracesEndpointConfiguration config) {
    final TracesEndpointCliOptions options = create();
    options.tracesOutputPath = config.tracesOutputPath();
    options.traceCompression = config.traceCompression();
    options.traceFileVersion = config.traceFileVersion();
    options.caching = config.caching();
    return options;
  }

  /**
   * To domain object Linea factory configuration.
   *
   * @return the Linea factory configuration
   */
  @Override
  public TracesEndpointConfiguration toDomainObject() {
    return TracesEndpointConfiguration.builder()
        .tracesOutputPath(tracesOutputPath)
        .traceCompression(traceCompression)
        .traceFileVersion(traceFileVersion)
        .caching(caching)
        .build();
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(CONFLATED_TRACE_GENERATION_TRACES_OUTPUT_PATH, tracesOutputPath)
        .add(CONFLATED_TRACE_GENERATION_TRACE_COMPRESSION, traceCompression)
        .add(CONFLATED_TRACE_GENERATION_TRACE_FILE_VERSION, traceFileVersion)
        .add(CACHING, caching)
        .toString();
  }
}
