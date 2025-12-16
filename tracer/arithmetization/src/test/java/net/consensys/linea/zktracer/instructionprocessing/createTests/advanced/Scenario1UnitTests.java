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
package net.consensys.linea.zktracer.instructionprocessing.createTests.advanced;

import static net.consensys.linea.zktracer.instructionprocessing.createTests.advanced.AdvancedCreate2ScenarioValue.*;
import static net.consensys.linea.zktracer.instructionprocessing.createTests.advanced.ScenarioUtils.*;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.userAccount;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import net.consensys.linea.zktracer.instructionprocessing.utilities.SmartContractTestValidator;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

// Recap : CustomCreate2 is a contract with multiple methods to deploy ContractC using CREATE2
// opcode
// See more details in AllScenariiInitCodeTests

/*

SCENARIO 1 - CREATE2 FOUR TIMES

Four ContractC deployment attempts :
  (1) with max value - aborted because of balance too low
  (2) acceptable value - ContractC deployed
  We test that ContractC is deployed with non-empty code by modifying its storage
  (3) max value - aborted
  (4) acceptable value - ContractC deployment fails as it's a collision with attempt (2)
  Ends with a revert

       CALL
     -------->  - (1) CREATE2
                - (2) CREATE2 -----> ContractC deployed
                - (3) CREATE2
                - (4) CREATE2 -----> Collision, deployment fails
                - REVERT

Note : storeInitCodeC and storeSalt transactions are preparation transactions

 */

public class Scenario1UnitTests extends TracerTestBase {

  /*
   SCENARIO 1 - NO REVERT AT THE END
   The Scenario 1 ends with a revert. We test with no revert at the end as an intermediary step to check
   - ContractC is effectively deployed in attempt (2)
   - ContractC code is non-empty and we can modify its storage
   TXSTATUS : Successful
   LOGS: 1 ContractCreated, 1 StoreInMap
   Note : 4 CREATE2 opcode called
   Note 2 : transactions are sent with value 2 as will be done in the final AllScenarii test (see Scenarii 3 for reason to use value 2)
  */
  @Test
  void deployScenario1NoRevert(TestInfo testInfo) {
    Map<String, List<Integer>> logsTopicMap = new HashMap<>();
    Map<String, List<Bytes>> logsDataMap = new HashMap<>();

    // Tx status
    List<Integer> txStatuses = List.of(1, 1, 1);

    // Tx logs to check in validator
    logsTopicMap.put(contractCreatedEvent, List.of(0, 0, 1));
    logsTopicMap.put(storeInMapEvent, List.of(0, 0, 1));
    logsDataMap.put(
        contractCreatedEvent, List.of(Bytes.EMPTY, Bytes.EMPTY, expectedContractCAddressLogData));
    // The storeInMap call uses msg.value as the key, here it's 2
    logsDataMap.put(
        storeInMapEvent,
        List.of(
            Bytes.EMPTY,
            Bytes.EMPTY,
            Bytes.fromHexString(
                "0x0000000000000000000000000000000000000000000000000000000000000002")));

    // Instantiate validator
    TransactionProcessingResultValidator txValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, logsDataMap);

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(storeInitCodeC, storeSalt, create2FourTimes_noRevert),
            List.of(NONE, NONE, CREATE2_WITH_IMMEDIATE_REDEPLOYMENT));

    final ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount, customCreate2Account))
            .transactions(transactions)
            .transactionProcessingResultValidator(txValidator)
            .build();
    toyExecutionEnvironmentV2.run();

    assertDeploymentNumberContractC(toyExecutionEnvironmentV2, 1);
  }

  /*
  SCENARIO 1 - WITH REVERT AT THE END
  TXSTATUS : Failed
  LOGS: 0 ContractCreated, 0 StoreInMap as the whole transaction is reverted
  Note : 4 CREATE2 opcode called
  Note 2 : transactions are sent with value 2 as will be done in the final AllScenarii test
   */
  @Test
  void deployScenario1WithRevert(TestInfo testInfo) {
    Map<String, List<Integer>> logsTopicMap = new HashMap<>();

    // Tx status
    List<Integer> txStatuses = List.of(1, 1, 0);

    // Tx logs to check in validator
    logsTopicMap.put(contractCreatedEvent, List.of(0, 0, 0));
    logsTopicMap.put(storeInMapEvent, List.of(0, 0, 0));

    // Instantiate validator
    TransactionProcessingResultValidator txValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, new HashMap<>());

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(storeInitCodeC, storeSalt, create2FourTimes_withRevert),
            List.of(NONE, NONE, CREATE2_WITH_IMMEDIATE_REDEPLOYMENT));

    final ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount, customCreate2Account))
            .transactions(transactions)
            .transactionProcessingResultValidator(txValidator)
            .build();
    toyExecutionEnvironmentV2.run();

    assertDeploymentNumberContractC(toyExecutionEnvironmentV2, 1);
  }

  /*
  SCENARIO 1 - Through a Call
  TXSTATUS : Successful
  LOGS: 1 CallMyselfFail, 0 ContractCreated, 0 StoreInMap
  Note : 4 CREATE2 opcode called
  Note 2 : transactions are sent with value 2 as will be done in the final AllScenarii test
  Note 3 : used in ScenariiTriggeredFromRoot and in ScenariiNestedCalls
   */
  @Test
  void deployScenario1(TestInfo testInfo) {
    Map<String, List<Integer>> logsTopicMap = new HashMap<>();

    // Tx status
    List<Integer> txStatuses = List.of(1, 1, 1);

    // Tx logs to check in validator
    logsTopicMap.put(callMyselfFailEvent, List.of(0, 0, 1));
    logsTopicMap.put(contractCreatedEvent, List.of(0, 0, 0));
    logsTopicMap.put(storeInMapEvent, List.of(0, 0, 0));

    // Instantiate validator
    TransactionProcessingResultValidator txValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, new HashMap<>());

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(storeInitCodeC, storeSalt, callMyselfWithCreate2FourTimes_withRevert),
            List.of(NONE, NONE, CREATE2_WITH_IMMEDIATE_REDEPLOYMENT));

    final ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount, customCreate2Account))
            .transactions(transactions)
            .transactionProcessingResultValidator(txValidator)
            .build();
    toyExecutionEnvironmentV2.run();

    assertDeploymentNumberContractC(toyExecutionEnvironmentV2, 1);
  }
}
