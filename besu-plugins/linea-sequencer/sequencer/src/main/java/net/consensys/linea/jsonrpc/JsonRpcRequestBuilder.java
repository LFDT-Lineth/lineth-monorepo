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

package net.consensys.linea.jsonrpc;

import java.time.Instant;
import java.util.List;
import java.util.Optional;
import java.util.concurrent.atomic.AtomicLong;

import com.google.gson.JsonArray;
import com.google.gson.JsonObject;
import net.consensys.linea.config.LineaNodeType;
import net.consensys.linea.sequencer.modulelimit.ModuleLimitsValidationResult;
import org.hyperledger.besu.datatypes.Transaction;

/**
 * Helper class to build JSON-RPC requests for rejected transactions.
 *
 * <pre>
 * {@code linea_saveRejectedTransactionV1({
 *         "txRejectionStage": "SEQUENCER/RPC/P2P",
 *         "timestamp": "2024-08-22T09:18:51Z", # ISO8601 UTC+0 when tx was rejected by node, usefull if P2P edge node.
 *         "blockNumber": "base 10 number",
 *         "transactionRLP": "transaction as the user sent in eth_sendRawTransaction",
 *         "reason": "Transaction line count for module ADD=402 is above the limit 70"
 *         "overflows": [{
 *           "module": "ADD",
 *           "count": 402,
 *           "limit": 70
 *         }, {
 *           "module": "MUL",
 *           "count": 587,
 *           "limit": 400
 *         }]
 *     })
 * }
 * </pre>
 */
public class JsonRpcRequestBuilder {
  private static final AtomicLong idCounter = new AtomicLong(1);

  /**
   * Generate linea_saveRejectedTransactionV1 JSON-RPC request from given arguments.
   *
   * @param lineaNodeType Linea node type which is reporting the rejected transaction.
   * @param transaction The rejected transaction. The encoded transaction RLP is used in the
   *     JSON-RPC request.
   * @param timestamp The timestamp when the transaction was rejected.
   * @param blockNumber Optional block number where the transaction was rejected. Used for sequencer
   *     node.
   * @param reasonMessage The reason message for the rejection.
   * @return JSON-RPC request as a string.
   */
  public static String generateSaveRejectedTxJsonRpc(
      final LineaNodeType lineaNodeType,
      final Transaction transaction,
      final Instant timestamp,
      final Optional<Long> blockNumber,
      final String reasonMessage,
      final List<ModuleLimitsValidationResult> overflowValidationResults) {
    final JsonObject params = new JsonObject();
    params.addProperty("txRejectionStage", lineaNodeType.name());
    params.addProperty("timestamp", timestamp.toString());
    blockNumber.ifPresent(number -> params.addProperty("blockNumber", number));
    params.addProperty("transactionRLP", transaction.encoded().toHexString());
    params.addProperty("reasonMessage", reasonMessage);

    // overflows
    final JsonArray overflows = new JsonArray();
    for (ModuleLimitsValidationResult result : overflowValidationResults) {
      JsonObject overflow = new JsonObject();
      overflow.addProperty("module", result.getModuleName());
      overflow.addProperty("count", result.getModuleLineCount());
      overflow.addProperty("limit", result.getModuleLineLimit());
      overflows.add(overflow);
    }
    params.add("overflows", overflows);

    // request
    final JsonObject request = new JsonObject();
    request.addProperty("jsonrpc", "2.0");
    request.addProperty("method", "linea_saveRejectedTransactionV1");
    request.add("params", params);
    request.addProperty("id", idCounter.getAndIncrement());
    return request.toString();
  }
}
