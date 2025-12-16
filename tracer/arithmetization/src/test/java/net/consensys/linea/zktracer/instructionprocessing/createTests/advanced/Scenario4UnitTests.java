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

import static net.consensys.linea.zktracer.instructionprocessing.createTests.advanced.AdvancedCreate2ScenarioValue.CREATE2_WITH_IMMEDIATE_REDEPLOYMENT;
import static net.consensys.linea.zktracer.instructionprocessing.createTests.advanced.AdvancedCreate2ScenarioValue.NONE;
import static net.consensys.linea.zktracer.instructionprocessing.createTests.advanced.ScenarioUtils.*;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.userAccount;

import java.util.HashMap;
import java.util.List;
import java.util.Map;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import net.consensys.linea.zktracer.instructionprocessing.utilities.SmartContractTestValidator;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

// Recap : CustomCreate2 is a contract with multiple methods to deploy ContractC using CREATE2
// opcode
// See more details in AllScenariiInitCodeTests

/*

SCENARIO 4 - ATTEMPT CREATE2 AFTER A CREATE2

Attempts a create2 after a successful create2.
Done by calling the ContractC that has just been deployed to do a callback to CustomCreate2 and attempt redeployment.

       CALL
     -------->  (1) CREATE2 -----> ContractC deployed
                     CALLC                CALL
                (2) ---------> callback ---------> CREATE2 -----> Collision, deployment fails

When Nested :

       CALL           CALL               CALL               CALL
      -------->  (0) -------> Scenario3 -------> Scenario2 -------> Scenario1
                 (1) and (2) stay the same

Note : storeInitCodeC and storeSalt transactions are preparation transactions
Note 2 : CALLC stands for CALL (contract) C

*/

public class Scenario4UnitTests extends TracerTestBase {

  /*
  SCENARIO 4 -
  TXSTATUS : Successful
  LOGS: 1 ContractCreated, 1 CallCreate2WithInitCodeC_noValue
  Note: transaction is sent with value 2
  Note 2 : used in ScenariiTriggeredFromRoot
   */
  @Test
  void deployScenario4(TestInfo testInfo) {
    Map<String, List<Integer>> logsTopicMap = new HashMap<>();
    List<Integer> txStatuses = List.of(1, 1, 1);

    logsTopicMap.put(contractCreatedEvent, List.of(0, 0, 1));
    // Call to attempt a redeployment after the successful one
    logsTopicMap.put(callCreate2WithInitCodeC_noValue_Event, List.of(0, 0, 1));

    // Instantiate validator
    TransactionProcessingResultValidator create2OneTxValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, new HashMap<>());

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(storeInitCodeC, storeSalt, create2WithCallCtoCallback_noValue),
            List.of(NONE, NONE, CREATE2_WITH_IMMEDIATE_REDEPLOYMENT));

    final ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount, customCreate2Account))
            .transactions(transactions)
            .transactionProcessingResultValidator(create2OneTxValidator)
            .build();
    toyExecutionEnvironmentV2.run();

    assertDeploymentNumberContractC(toyExecutionEnvironmentV2, 1);
  }

  /*
  SCENARIO 4 - WITH NESTED CALLS TO SCENARIO 1 AND 2 AND 3
  TXSTATUS : Successful
  LOGS: 1 CallMyselfFail, 1 ContractCreated, 1 CallCreate2WithInitCodeC_noValue
  Note: transaction is sent with value 2
  Note 2 : used in ScenariiNestedCalls
   */
  @Test
  void deployScenario4Nested(TestInfo testInfo) {
    Map<String, List<Integer>> logsTopicMap = new HashMap<>();
    List<Integer> txStatuses = List.of(1, 1, 1);

    // 1 from the nested call
    logsTopicMap.put(callMyselfFailEvent, List.of(0, 0, 1));
    // 1 from the scenario 4
    logsTopicMap.put(contractCreatedEvent, List.of(0, 0, 1));
    // Callback from ContractC to attempt a redeployment after the successful one
    logsTopicMap.put(callCreate2WithInitCodeC_noValue_Event, List.of(0, 0, 1));

    // Instantiate validator
    TransactionProcessingResultValidator txValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, new HashMap<>());

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(
                storeInitCodeC,
                storeSalt,
                callMyselfWithCreate2WithCallCtoCallback_noValueAndNested),
            List.of(NONE, NONE, CREATE2_WITH_IMMEDIATE_REDEPLOYMENT));

    final ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount, customCreate2Account))
            .transactions(transactions)
            .transactionProcessingResultValidator(txValidator)
            .build();
    toyExecutionEnvironmentV2.run();

    // 1 create2 reverted (scenario 1), 1 create2 reverted (scenario 3) and 1 create2 successful
    // (scenario 4)
    assertDeploymentNumberContractC(toyExecutionEnvironmentV2, 3);
  }
}
