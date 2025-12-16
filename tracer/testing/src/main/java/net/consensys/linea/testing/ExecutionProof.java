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

package net.consensys.linea.testing;

import com.fasterxml.jackson.databind.node.ArrayNode;
import java.util.List;

public final class ExecutionProof {

  public record BatchExecutionProofRequestDto(
      String zkParentStateRootHash,
      String keccakParentStateRootHash,
      String conflatedExecutionTracesFile,
      String tracesEngineVersion,
      String type2StateManagerVersion,
      ArrayNode zkStateMerkleProof,
      List<RlpBridgeLogsData> blocksData) {}

  public record RlpBridgeLogsData(String rlp, List<BridgeLogsData> bridgeLogs) {}

  public record BridgeLogsData(
      Boolean removed,
      String logIndex,
      String transactionIndex,
      String transactionHash,
      String blockHash,
      String blockNumber,
      String address,
      String data,
      List<String> topics) {}

  public static String getExecutionProofRequestFilename(
      long startBlockNumber,
      long endBlockNumber,
      String tracesVersion,
      String stateManagerVersion) {
    return String.format(
        "%d-%d-etv%s-stv%s-getZkProof.json",
        startBlockNumber, endBlockNumber, tracesVersion, stateManagerVersion);
  }
}
