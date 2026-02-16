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

package net.consensys.linea.plugins.rpc.linecounts;

import java.security.InvalidParameterException;
import net.consensys.linea.zktracer.ZkTracer;

/** Holds needed parameters for sending an execution trace generation request. */
public record LineCountsRequestParams(long blockNumber, String expectedTracesEngineVersion) {
  public void validate() {
    if (!expectedTracesEngineVersion.equals(getTracerRuntime())) {
      throw new InvalidParameterException(
          String.format(
              "INVALID_TRACES_VERSION: Runtime version is %s, requesting version %s",
              getTracerRuntime(), expectedTracesEngineVersion));
    }

    if (blockNumber < 0) {
      throw new InvalidParameterException(
          "INVALID_BLOCK_NUMBER: blockNumber: %d cannot be a negative number"
              .formatted(blockNumber));
    }
  }

  private static String getTracerRuntime() {
    String tracerVersion = ZkTracer.class.getPackage().getSpecificationVersion();
    // remove the "7-character hex string" suffix (if any) from the tracer version
    return tracerVersion != null
        ? ZkTracer.VERSION_HEX_SUFFIX_PATTERN.matcher(tracerVersion).replaceAll("")
        : null;
  }
}
