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
package net.consensys.linea.zktracer.instructionprocessing.utilities;

import static org.assertj.core.api.Fail.fail;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.List;
import java.util.Map;

import lombok.NonNull;
import lombok.RequiredArgsConstructor;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.processing.TransactionProcessingResult;
import org.hyperledger.besu.evm.log.LogTopic;

@RequiredArgsConstructor
public class SmartContractTestValidator implements TransactionProcessingResultValidator {
  @NonNull List<Integer> txStatuses;
  @NonNull Map<String, List<Integer>> logsTopicMap;
  @NonNull Map<String, List<Bytes>> logsDataMap;
  int txCounter = 0;

  @Override
  public void accept(Transaction transaction, TransactionProcessingResult result) {
    TransactionProcessingResultValidator.EMPTY_VALIDATOR.accept(transaction, result);
    System.out.println("Transaction number: " + txCounter);

    // Check that the transaction has the awaited status
    TransactionProcessingResult.Status resultStatus = result.getStatus();
    TransactionProcessingResult.Status toValidateStatus =
        txStatuses.get(txCounter) == 1
            ? TransactionProcessingResult.Status.SUCCESSFUL
            : TransactionProcessingResult.Status.FAILED;
    assertEquals(toValidateStatus, resultStatus);

    // Check that logs from topics listed are present in the transaction
    // Check that data from logs topics listed are correct
    int totalLogsMapPerTx = 0;
    for (var logsTopicMapEntry : logsTopicMap.entrySet()) {
      int logsMaplogCount = logsTopicMapEntry.getValue().get(txCounter);
      String logsMapTopic = logsTopicMapEntry.getKey();
      totalLogsMapPerTx = totalLogsMapPerTx + logsMaplogCount;

      if (logsMaplogCount > 0) {
        for (int i = 0; i < result.getLogs().size(); i++) {
          List<LogTopic> txLogsTopics = result.getLogs().get(i).getTopics();
          for (int j = 0; j < txLogsTopics.size(); j++) {
            String txLogsTopic = txLogsTopics.get(j).toString();
            if (logsMapTopic.toString().equals(txLogsTopic)) {
              logsMaplogCount--;
              if ((logsDataMap.get(txLogsTopic) != null)
                  && (logsDataMap.get(txLogsTopic).get(txCounter) != Bytes.EMPTY)) {
                // Check Data
                assertEquals(
                    logsDataMap.get(txLogsTopic).get(txCounter).toString(),
                    result.getLogs().get(i).getData().toString());
              }
            }
          }
        }
      }

      // Check log topics
      if (logsMaplogCount != 0) {
        fail("Log count mismatch for topic: " + logsMapTopic + " and Tx counter: " + txCounter);
      }
    }

    // Global check on number of logs per transaction
    // (In case some logs were not listed above)
    assertEquals(totalLogsMapPerTx, result.getLogs().size());

    txCounter++;
  }
}
