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
package net.consensys.linea.zktracer.statemanager;

import static org.assertj.core.api.Fail.fail;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.List;
import lombok.NonNull;
import lombok.RequiredArgsConstructor;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import net.consensys.linea.testing.Web3jUtils;
import net.consensys.linea.testing.generated.FrameworkEntrypoint;
import net.consensys.linea.testing.generated.StateManagerEvents;
import net.consensys.linea.testing.generated.TestSnippet_Events;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.hyperledger.besu.ethereum.processing.TransactionProcessingResult;
import org.hyperledger.besu.evm.log.Log;
import org.web3j.abi.EventEncoder;

@RequiredArgsConstructor
public class StateManagerTestValidator implements TransactionProcessingResultValidator {
  @NonNull ToyAccount frameworkEntryPointAccount;
  @NonNull List<Integer> expectedNoLogs;
  int txCounter = 0;

  @Override
  public void accept(Transaction transaction, TransactionProcessingResult result) {
    TransactionProcessingResultValidator.DEFAULT_VALIDATOR.accept(transaction, result);
    // One event from the snippet
    // One event from the framework entrypoint about contract call
    assertEquals(
        expectedNoLogs.get(txCounter),
        result.getLogs().size(),
        "Unexpected number of logs for transaction" + txCounter);
    for (int i = 0; i < result.getLogs().size(); i++) {
      Log currentLog = result.getLogs().get(i);
      String callEventSignature = EventEncoder.encode(FrameworkEntrypoint.CALLEXECUTED_EVENT);
      String writeEventSignature = EventEncoder.encode(StateManagerEvents.WRITE_EVENT);
      String readEventSignature = EventEncoder.encode(StateManagerEvents.READ_EVENT);
      String destroyedEventSignature =
          EventEncoder.encode(FrameworkEntrypoint.CONTRACTDESTROYED_EVENT);
      String createdEventSignature = EventEncoder.encode(FrameworkEntrypoint.CONTRACTCREATED_EVENT);
      String sentETHEventSignature = EventEncoder.encode(StateManagerEvents.PAYETH_EVENT);
      String recETHEventSignature = EventEncoder.encode(StateManagerEvents.RECETH_EVENT);
      String logTopic = currentLog.getTopics().getFirst().toHexString();
      if (EventEncoder.encode(TestSnippet_Events.DATANOINDEXES_EVENT).equals(logTopic)) {
        TestSnippet_Events.DataNoIndexesEventResponse response =
            TestSnippet_Events.getDataNoIndexesEventFromLog(Web3jUtils.fromBesuLog(currentLog));
        // assertEquals(response.singleInt, BigInteger.valueOf(123456));
      } else if (EventEncoder.encode(FrameworkEntrypoint.CALLEXECUTED_EVENT).equals(logTopic)) {
        FrameworkEntrypoint.CallExecutedEventResponse response =
            FrameworkEntrypoint.getCallExecutedEventFromLog(Web3jUtils.fromBesuLog(currentLog));
        if (i > 0
            && !result
                .getLogs()
                .get(i - 1)
                .getTopics()
                .getFirst()
                .toHexString()
                .equals(callEventSignature)) {
          // when the number of logs is 1, in our tests we will have an operation
          // with the revert flag set to true.
          assertTrue(response.isSuccess);
        }
        if (logTopic.equals(createdEventSignature)) {
          assertEquals(response.destination, frameworkEntryPointAccount.getAddress().toHexString());
        } else {
          // assertEquals(response.destination,
          // this.testContext.initialAccounts[0].getAddress().toHexString());
        }
      } else {
        if (!(logTopic.equals(callEventSignature)
            || logTopic.equals(writeEventSignature)
            || logTopic.equals(readEventSignature)
            || logTopic.equals(destroyedEventSignature)
            || logTopic.equals(createdEventSignature)
            || logTopic.equals(sentETHEventSignature)
            || logTopic.equals(recETHEventSignature))) {
          fail();
        }
      }
    }
    txCounter++;
  }
}
