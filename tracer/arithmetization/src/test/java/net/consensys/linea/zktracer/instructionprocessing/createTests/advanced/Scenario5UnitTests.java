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

import static net.consensys.linea.zktracer.Fork.isPostCancun;
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

SCENARIO 5 - MODIFY STORAGE, SELFDESTRUCT

After a successful create2 (from Scenario 4), we modify the contract storage and self-destruct

       CALLC
     -------->  - (1) Modify storage
       CALLC
     -------->  - (2) Self-destruct

Note : storeInitCodeC and storeSalt and create2WithCallCtoCallback_noValue transactions are preparation transactions. create2WithCallCtoCallback_noValue is there to have the state post scenario 4
Note 2 : CALLC stands for CALL (contract) C

 */

public class Scenario5UnitTests extends TracerTestBase {

  /*
  SCENARIO 5 - CREATE2, MODIFY STORAGE, SELFDESTRUCT
  After a successful create2 (from Scenario 4), we modify the contract storage and self-destruct
  TXSTATUS : Successful
  LOGS: 1 ContractCreated, 1 CallCreate2WithInitCodeC_noValue, 1 StoreInMap, 1 SelfDestruct
  Note: transaction is sent with value 2
  Note 2 : used in ScenariiTriggeredFromRoot
   */
  @Test
  void deployScenario5(TestInfo testInfo) {
    Map<String, List<Integer>> logsTopicMap = new HashMap<>();
    Map<String, List<Bytes>> logsDataMap = new HashMap<>();
    List<Integer> txStatuses = List.of(1, 1, 1, 1);

    // Logs from deployment done in scenario 4
    logsTopicMap.put(contractCreatedEvent, List.of(0, 0, 1, 0));
    logsTopicMap.put(callCreate2WithInitCodeC_noValue_Event, List.of(0, 0, 1, 0));
    // Logs from scenario 5
    logsTopicMap.put(storeInMapEvent, List.of(0, 0, 0, 1));
    logsTopicMap.put(selfDestructEvent, List.of(0, 0, 0, 1));

    // Instantiate validator
    TransactionProcessingResultValidator txValidator =
        new SmartContractTestValidator(txStatuses, logsTopicMap, logsDataMap);

    List<Transaction> transactions =
        getTransactions(
            customCreate2Account,
            userAccount,
            List.of(
                storeInitCodeC,
                storeSalt,
                create2WithCallCtoCallback_noValue, /* Same deployment as scenario 4 */
                callCToModifyStorageAndSelfdestruct),
            List.of(
                NONE,
                NONE,
                CREATE2_WITH_IMMEDIATE_REDEPLOYMENT,
                CREATE2_WITH_IMMEDIATE_REDEPLOYMENT));

    final ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount, customCreate2Account))
            .transactions(transactions)
            .transactionProcessingResultValidator(txValidator)
            .build();
    toyExecutionEnvironmentV2.run();

    // Pre-Cancun : 1 create2 + 1 selfdestruct
    // Post-Cancun : 1 create2 only as selfdestruct is not in the same transaction
    int expectedDeploymentNumber = isPostCancun(fork) ? 1 : 2;
    assertDeploymentNumberContractC(toyExecutionEnvironmentV2, expectedDeploymentNumber);
  }
}
