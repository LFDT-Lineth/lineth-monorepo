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
import lombok.extern.slf4j.Slf4j;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.*;
import net.consensys.linea.zktracer.instructionprocessing.utilities.SmartContractTestValidator;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

/*
This suite aims at testing advanced CREATE2 scenarii using smart contracts

** CustomCreate2 **
CustomCreate2 is the smart contract used to pilot the deployment of a complex initCodeC
CustomCreate2 stores an initCodeC and a salt used for subsequent deployments
CustomCreate2 has different create2 methods to have deployment scenarii within the same
transaction
CustomCreate2 can CALL/STATICCALL itself
CustomCreate2 can CALL/STATICCALL ContractC

** initCodeC **
initCodeC can be piloted by CustomCreate2 via the value passed in the transaction
value 1 Wei enables a storage modification in ContractC
value 2 Wei calls CustomCreate2 back to trigger an immediate redeployment of ContractC
value 3 Wei triggers a self-destruct of ContractC
value 4 Wei triggers a revert on demand

** ContractC **
ContractC can modify storage, revert on demand, self-destruct on demand, and call back
CustomCreate2 to trigger a redeployment of ContractC

In this test, we test Scenario 1 to 5 in two ways
- we trigger Calls from a root context
- we trigger nested Calls going from Scenario 5 to 1, with 1 being executed first
The 5 scenarii are unit tested in separate tests for clarity and ease of debugging

When Triggered from root context, in one transaction

      - (1) Scenario 1
              CALL
             -------->  - create2 four times
      - (2) Scenario 2
             STATICCALL
           -------------->  - CREATE2
      - (3) Scenario 3
              CALL
             -------->  - create2 within create2
      - (4) Scenario 4
              CALL
             -------->  - create2 after create2
      - (5) Scenario 5
              CALLC
             --------> - Modify storage
              CALLC
             --------> - Self-destruct

When Nested

       CALL            CALL               CALL               CALL               CALL
     --------> - (0) -------> Scenario4 -------> Scenario3 -------> Scenario2 -------> Scenario1
       CALLC
     -------->  - (1) Modify storage
       CALLC
     -------->  - (2) Self-destruct

Note : CALLC stands for CALL (contract) C

 */

@Slf4j
@ExtendWith(UnitTestWatcher.class)
public class AllScenariiInitCodeTests extends TracerTestBase {

  /*
  ** TRANSACTION 1 **
  Play Scenario from 1 to 5 with Calls from root context.
  The scenarii should end with a successful self-destruct of ContractC

  ** TRANSACTION 2 **
  As a check, we send a second transaction to attempt a redeployment of ContractC at the same address
   */
  @Test
  void deployContractCWithCreate2ScenariiTriggeredFromRoot(TestInfo testInfo) {
    // Payload preparation
    Bytes advancedCreateScenariiTriggeredFromRoot =
        CustomCreate2Payload.advancedCreateScenariiTriggeredFromRoot(initCodeC, salt);

    Map<String, List<Integer>> logsTopicMap = new HashMap<>();
    Map<String, List<Bytes>> logsDataMap = new HashMap<>();

    // Tx status
    List<Integer> txStatuses = List.of(1, 1);

    // Tx logs to check in validator

    // 1 in scenario 1
    // 1 in scenario 3
    logsTopicMap.put(callMyselfFailEvent, List.of(2, 0));
    // 1 in scenario 2
    logsTopicMap.put(staticCallMyselfFailEvent, List.of(1, 0));
    // 1 in scenario 4
    logsTopicMap.put(contractCreatedEvent, List.of(1, 1));
    logsTopicMap.put(callCreate2WithInitCodeC_noValue_Event, List.of(1, 0));
    // 1 in scenario 5
    logsTopicMap.put(storeInMapEvent, List.of(1, 0));
    logsTopicMap.put(selfDestructEvent, List.of(1, 0));
    // We test a create2 in a separate transaction from the scenarii, to see if selfdestruct from
    // previous
    // transaction has worked
    logsTopicMap.put(callCreate2WithInitCodeC_withValue_Event, List.of(0, 1));

    logsDataMap.put(
        contractCreatedEvent,
        List.of(expectedContractCAddressLogData, expectedContractCAddressLogData));
    // The storeInMap call uses key 1
    logsDataMap.put(
        storeInMapEvent,
        List.of(
            Bytes.fromHexString(
                "0x0000000000000000000000000000000000000000000000000000000000000001"),
            Bytes.EMPTY));

    // Instantiate validator
    TransactionProcessingResultValidator txValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, logsDataMap);

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(advancedCreateScenariiTriggeredFromRoot, create2WithInitCodeC_withValue),
            List.of(CREATE2_WITH_IMMEDIATE_REDEPLOYMENT, NONE));

    final ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount, customCreate2Account))
            .transactions(transactions)
            .transactionProcessingResultValidator(txValidator)
            .build();
    toyExecutionEnvironmentV2.run();

    // Check deployment number for ContractC

    // At start, deploymentNumber = 0

    // TRANSACTION 1

    // Scenario 1 - create2 four times : deploymentNumber ++
    // - Only one create2 is successful so increments the deployment number by 1
    // - deploymentNumber = 1
    // - ends with a revert
    // Scenario 2 - staticCallMyselfFail : deploymentNumber not changed
    // Scenario 3 - create2 within create2 : deploymentNumber ++
    // - Deploys contract C with empty bytecode
    // - deploymentNumber = 2
    // - ends with a revert
    // Scenario 4 - create2 after create2 : deploymentNumber ++
    // - First create2 is successful so increments the deployment number
    // - deploymentNumber = 3
    // Scenario 5 - modify storage and self-destruct : deploymentNumber ++
    // - Self-destruct successful increments deployment number
    // - deploymentNumber = 4

    // TRANSACTION 2 - attempt redeployment : deploymentNumber ++
    // - Attempts a redeployment of ContractC at the same address as in transaction 1
    // - deploymentNumber = 5

    assertDeploymentNumberContractC(toyExecutionEnvironmentV2, 5);
  }

  /*

  ** Transaction 1 **
  Trigger Scenario 5 to 1 in nested Calls
  The scenarii should end with a successful self-destruct of ContractC

  ** Transaction 2 **
  As a check, we send a second transaction to attempt a redeployment of ContractC at the same address
  - pre-Cancun : this should be successful
  - post-Cancun : this should fail as the self-destruct did not happen in the same transaction as
  the deployment

  */
  @Test
  void deployContractCWithCreate2ScenariiNestedCalls(TestInfo testInfo) {
    // Payload preparation
    Bytes advancedCreateScenariiNestedCalls =
        CustomCreate2Payload.advancedCreateScenariiNestedCalls(initCodeC, salt);

    Map<String, List<Integer>> logsTopicMap = new HashMap<>();
    Map<String, List<Bytes>> logsDataMap = new HashMap<>();

    List<Integer> txStatuses = List.of(1, 1);

    // 1 in call to scenario 3
    logsTopicMap.put(callMyselfFailEvent, List.of(1, 0));
    // 1 in scenario 4
    logsTopicMap.put(contractCreatedEvent, List.of(1, 1));
    logsTopicMap.put(callCreate2WithInitCodeC_noValue_Event, List.of(1, 0));
    // 1 in scenario 5
    logsTopicMap.put(storeInMapEvent, List.of(1, 0));
    logsTopicMap.put(selfDestructEvent, List.of(1, 0));
    // In the last create2 from the last transaction, to see if selfdestruct from previous
    // transaction has worked
    logsTopicMap.put(callCreate2WithInitCodeC_withValue_Event, List.of(0, 1));

    logsDataMap.put(
        contractCreatedEvent,
        List.of(expectedContractCAddressLogData, expectedContractCAddressLogData));
    // The storeInMap call uses key 1
    logsDataMap.put(
        storeInMapEvent,
        List.of(
            Bytes.fromHexString(
                "0x0000000000000000000000000000000000000000000000000000000000000001"),
            Bytes.EMPTY));

    // Instantiate validator
    TransactionProcessingResultValidator txValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, logsDataMap);

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(advancedCreateScenariiNestedCalls, create2WithInitCodeC_withValue),
            List.of(CREATE2_WITH_IMMEDIATE_REDEPLOYMENT, NONE));

    final ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount, customCreate2Account))
            .transactions(transactions)
            .transactionProcessingResultValidator(txValidator)
            .build();
    toyExecutionEnvironmentV2.run();

    // See explanation in test deployContractCWithCreate2ScenariiTriggeredFromRoot above
    assertDeploymentNumberContractC(toyExecutionEnvironmentV2, 5);
  }
}
