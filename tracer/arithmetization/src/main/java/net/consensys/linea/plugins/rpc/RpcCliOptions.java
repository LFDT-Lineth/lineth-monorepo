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

import com.google.common.base.MoreObjects;
import net.consensys.linea.plugins.LineaCliOptions;
import picocli.CommandLine;

public class RpcCliOptions implements LineaCliOptions {

  public static final String CONFIG_KEY = "rpc-config";

  static final String RPC_CONCURRENT_REQUESTS_LIMIT =
      "--plugin-linea-rpc-concurrent-requests-limit";

  @CommandLine.Option(
      required = true,
      names = {RPC_CONCURRENT_REQUESTS_LIMIT},
      hidden = true,
      paramLabel = "<REQUEST_COUNT_LIMIT>",
      description = "Number of allowed concurrent requests")
  private int concurrentRequestsLimit = 1;

  private RpcCliOptions() {}

  /**
   * Create Linea cli options.
   *
   * @return the Linea cli options
   */
  public static RpcCliOptions create() {
    return new RpcCliOptions();
  }

  /**
   * Linea cli options from config.
   *
   * @param config the config
   * @return the Linea cli options
   */
  static RpcCliOptions fromConfig(final RpcConfiguration config) {
    final RpcCliOptions options = create();
    options.concurrentRequestsLimit = config.concurrentRequestsLimit();
    return options;
  }

  /**
   * To domain object Linea factory configuration.
   *
   * @return the Linea factory configuration
   */
  @Override
  public RpcConfiguration toDomainObject() {
    return RpcConfiguration.builder().concurrentRequestsLimit(concurrentRequestsLimit).build();
  }

  @Override
  public String toString() {
    return MoreObjects.toStringHelper(this)
        .add(RPC_CONCURRENT_REQUESTS_LIMIT, concurrentRequestsLimit)
        .toString();
  }
}
