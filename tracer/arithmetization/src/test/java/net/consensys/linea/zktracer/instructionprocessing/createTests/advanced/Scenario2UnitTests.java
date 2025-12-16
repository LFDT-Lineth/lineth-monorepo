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
import lombok.extern.slf4j.Slf4j;
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

SCENARIO 2 - STATIC CALL A CREATE2

Attempt to static call a create2 deployment of ContractC

     STATICCALL
   -------------->  - CREATE2

 */

@Slf4j
public class Scenario2UnitTests extends TracerTestBase {

  /*
  SCENARIO 2 -
  Attempt to static call a create2 deployment
  TXSTATUS : Successful
  LOGS: 1 StaticCallMyselfFail
  Note : transactions are sent with value 2 as will be done in the final AllScenarii test
  Note 2 : used in ScenariiTriggeredFromRoot
   */
  @Test
  void deployScenario2(TestInfo testInfo) {
    Map<String, List<Integer>> logsTopicMap = new HashMap<>();

    // Tx status
    List<Integer> txStatuses = List.of(1);

    // Tx logs to check in validator
    logsTopicMap.put(staticCallMyselfFailEvent, List.of(1));

    // Instantiate validator
    TransactionProcessingResultValidator txValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, new HashMap<>());

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(create2WithStaticCall),
            List.of(CREATE2_WITH_IMMEDIATE_REDEPLOYMENT));

    final ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount, customCreate2Account))
            .transactions(transactions)
            .transactionProcessingResultValidator(txValidator)
            .build();
    toyExecutionEnvironmentV2.run();

    assertDeploymentNumberContractC(toyExecutionEnvironmentV2, 0);
  }

  /*
  SCENARIO 2 - WITH NESTED CALL TO SCENARIO 1
  Through a call, after create2 four times call, attempt to static call a create2 deployment
  TXSTATUS : Successful
  LOGS: 1 CallMyselfFail, 1 StaticCallMyselfFail
  Note 2 : used in ScenariiNestedCalls
  */
  @Test
  void deployScenario2Nested(TestInfo testInfo) {
    Map<String, List<Integer>> logsTopicMap = new HashMap<>();
    // Tx status
    List<Integer> txStatuses = List.of(1, 1, 1);

    // Tx logs to check in validator
    // 1 from scenario 1 nested
    logsTopicMap.put(callMyselfFailEvent, List.of(0, 0, 1));
    // 1 from scenario 2's static call
    logsTopicMap.put(staticCallMyselfFailEvent, List.of(0, 0, 1));

    log.info("callMyselfFailEvent: {}", callMyselfFailEvent);
    log.info("staticCallMyselfFailEvent: {}", staticCallMyselfFailEvent);
    // Instantiate validator
    TransactionProcessingResultValidator txValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, new HashMap<>());

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(storeInitCodeC, storeSalt, callMyselfWithCreate2WithStaticCall_nested),
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
