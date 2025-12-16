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
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

// Recap : CustomCreate2 is a contract with multiple methods to deploy ContractC using CREATE2
// opcode
// See more details in AllScenariiInitCodeTests

/*

SCENARIO 3 - ATTEMPT CREATE2 WITHIN A CREATE2

(1) Deploy ContractC and the deployment attempts redeployment.
The ContractC deployment is done with value 2 (CREATE2_WITH_IMMEDIATE_REDEPLOYMENT) - this value pilots the initcode so immediate redeployment is attempted.
While deploying ContractC adds STOP opcode after immediate redeployment attempt has failed.
ContractC is deployed with empty bytecode.
(2) Call ContractC to modify storage
(3) And Revert

       CALL                    CALL            fails
     -------->  (1) CREATE2  -------> CREATE2 -------> add STOP, deploy ContractC with empty bytecode
                     CALLC
                (2) ---------> modify storage
                (3) REVERT

When Nested :

       CALL           CALL              CALL
     -------->  (0) -------> Scenario2 -------> Scenario1
                (1) (2) and (3) stay the same


Note : storeInitCodeC and storeSalt transactions are preparation transactions
Note 2 : CALLC stands for CALL (contract) C

 */

public class Scenario3UnitTests extends TracerTestBase {

  /*
  SCENARIO 3 - NO REVERT AT THE END
  We test Scenario 3 with no revert at the end to check
  - ContractC is effectively deployed in attempt (1)
  - Immediate redeployment is attempted and fails
  - ContractC code is empty, so no storage modification is done
  TXSTATUS : Successful
  LOGS: 1 ContractCreated, 1 ImmediateRedeploymentFail, 0 CallCreate2WithInitCodeC_withValue
  because the create2 in the redeployment fails, 0 StoreInMap as the code is empty
  Note: transaction is sent with value 2 (CREATE2_WITH_IMMEDIATE_REDEPLOYMENT) to do a create2 within a create2
   */
  @Test
  void deployScenario3NoRevert(TestInfo testInfo) {
    Map<String, List<Integer>> logsTopicMap = new HashMap<>();

    // Tx status
    List<Integer> txStatuses = List.of(1, 1, 1);

    // Tx logs
    logsTopicMap.put(contractCreatedEvent, List.of(0, 0, 1));
    // Callback to attempt a create2 within a create2, triggered by the msg.value == 2
    logsTopicMap.put(immediateRedeploymentFailEvent, List.of(0, 0, 1));

    // Instantiate validator
    TransactionProcessingResultValidator txValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, new HashMap<>());

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(storeInitCodeC, storeSalt, create2CallC_noRevert),
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
  SCENARIO 3 -
  (1) Attempts a create 2 within a create2, goes back to initCodeC and stops
  Code deployed is empty.
  (2) We call contract C to modify storage.
  (3) We revert the whole transaction.
  TXSTATUS : Failed
  LOGS: 0 ContractCreated, O ImmediateRedeploymentFail, 0 CallCreate2WithInitCodeC_withValue
  0 StoreInMap as the whole transaction is reverted
  Note: transaction is sent with value 2 (CREATE2_WITH_IMMEDIATE_REDEPLOYMENT) to pilot initcode and do a create2 within a create2
  Note 2 : used in ScenariiTriggeredFromRoot
   */
  @Test
  void deployScenario3(TestInfo testInfo) {
    Map<String, List<Integer>> logsTopicMap = new HashMap<>();
    Map<String, List<Bytes>> logsDataMap = new HashMap<>();
    List<Integer> txStatuses = List.of(1, 1, 0);

    logsTopicMap.put(contractCreatedEvent, List.of(0, 0, 0));
    // Callback to attempt a create2 within a create2, triggered by the msg.value == 2
    logsTopicMap.put(immediateRedeploymentFailEvent, List.of(0, 0, 0));

    // Instantiate validator
    TransactionProcessingResultValidator txValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, logsDataMap);

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(storeInitCodeC, storeSalt, create2CallC_withRevert),
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
  SCENARIO 3 - WITH NESTED CALLS TO SCENARIO 1 AND 2
  Call to Scenario 2 that calls Scenario 1 and then
  Attempts a create2 within a create2, goes back to initCodeC and stops
  Code deployed is empty. We call contract C to modify storage. We revert the whole transaction
  TXSTATUS : Successful
  LOGS: 1 CallMyselfFail, 0 ContractCreated, O ImmediateRedeploymentFail, 0
  CallCreate2WithInitCodeC_withValue, 0 StoreInMap
  Note: transaction is sent with value 2 (CREATE2_WITH_IMMEDIATE_REDEPLOYMENT) to pilot initcode and do a create2 within a create2
  Note 2 : used in ScenariiNestedCalls
   */
  @Test
  void deployScenario3Nested(TestInfo testInfo) {
    Map<String, List<Integer>> logsTopicMap = new HashMap<>();
    Map<String, List<Bytes>> logsDataMap = new HashMap<>();
    List<Integer> txStatuses = List.of(1, 1, 1);

    logsTopicMap.put(callMyselfFailEvent, List.of(0, 0, 1));

    // Instantiate validator
    TransactionProcessingResultValidator create2OneTxValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, logsDataMap);

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(storeInitCodeC, storeSalt, callMyselfWithCreate2CallC_withRevertAndNested),
            List.of(NONE, NONE, CREATE2_WITH_IMMEDIATE_REDEPLOYMENT));

    final ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount, customCreate2Account))
            .transactions(transactions)
            .transactionProcessingResultValidator(create2OneTxValidator)
            .build();
    toyExecutionEnvironmentV2.run();

    // 1 create2 reverted (scenario 1) and 1 create2 reverted (scenario 3)
    assertDeploymentNumberContractC(toyExecutionEnvironmentV2, 2);
  }
}
