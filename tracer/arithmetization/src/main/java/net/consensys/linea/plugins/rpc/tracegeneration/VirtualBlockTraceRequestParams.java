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

/**
 * Holds parameters for generating virtual block conflated traces. Used for invalidity proof
 * generation for BadPrecompile and TooManyLogs scenarios.
 */
public record VirtualBlockTraceRequestParams(long blockNumber, String[] txsRlpEncoded) {

  public void validate() {
    if (blockNumber < 1) {
      throw new IllegalArgumentException(
          "INVALID_BLOCK_NUMBER: blockNumber: %d must be at least 1 (need parent block to exist)"
              .formatted(blockNumber));
    }

    if (txsRlpEncoded == null || txsRlpEncoded.length == 0) {
      throw new IllegalArgumentException(
          "INVALID_TRANSACTIONS: txsRlpEncoded must contain at least one transaction");
    }
  }
}
