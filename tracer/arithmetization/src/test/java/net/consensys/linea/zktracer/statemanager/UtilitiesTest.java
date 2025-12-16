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

import java.math.BigInteger;
import java.util.List;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.MultiBlockExecutionEnvironment;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class UtilitiesTest extends TracerTestBase {
  TestContext tc;

  @Test
  void testBuildingBlockOperations(TestInfo testInfo) {
    // initialize the test context
    this.tc = new TestContext();
    this.tc.initializeTestContext();
    TransactionProcessingResultValidator resultValidator =
        new StateManagerTestValidator(
            tc.frameworkEntryPointAccount,
            // Creates and self-destructs generate 2 logs,
            // Transfers generate 3 logs, the 1s are for reverted operations
            List.of(2, 2, 3, 2, 2));

    MultiBlockExecutionEnvironment.builder(chainConfig, testInfo)
        .accounts(
            List.of(
                tc.initialAccounts[0],
                tc.externallyOwnedAccounts[0],
                tc.initialAccounts[2],
                tc.frameworkEntryPointAccount))
        .addBlock(
            List.of(
                tc.writeToStorage(
                    tc.externallyOwnedAccounts[0],
                    tc.keyPairs[0],
                    tc.addresses[0],
                    123L,
                    1L,
                    false,
                    BigInteger.ZERO)))
        .addBlock(
            List.of(
                tc.readFromStorage(
                    tc.externallyOwnedAccounts[0],
                    tc.keyPairs[0],
                    tc.addresses[0],
                    123L,
                    false,
                    BigInteger.ZERO)))
        .addBlock(
            List.of(
                tc.transferTo(
                    tc.externallyOwnedAccounts[0],
                    tc.keyPairs[0],
                    tc.addresses[0],
                    tc.addresses[2],
                    8L,
                    false,
                    BigInteger.ONE)))
        // test operations above, before self-destructing a snippet in the next line
        .addBlock(
            List.of(
                tc.selfDestruct(
                    tc.externallyOwnedAccounts[0],
                    tc.keyPairs[0],
                    tc.addresses[0],
                    tc.frameworkEntryPointAddress,
                    false,
                    BigInteger
                        .ONE))) // use BigInteger.ONE, otherwise the framework entry point gets
        // destroyed
        .addBlock(
            List.of(
                tc.deployWithCreate2_withRevertTrigger(
                    tc.externallyOwnedAccounts[0],
                    tc.keyPairs[0],
                    tc.frameworkEntryPointAddress,
                    "0x0000000000000000000000000000000000000000000000000000000000000002",
                    TestContext.snippetsCodeForCreate2,
                    false)))
        .transactionProcessingResultValidator(resultValidator)
        .build()
        .run();
  }

  // Create 2 has a weird behavior and does not seem to work with the
  // bytecode output by the function
  // SmartContractUtils.getYulContractByteCode("StateManagerSnippets.yul")
  @Test
  void testCreate2Snippets(TestInfo testInfo) {
    // initialize the test context
    this.tc = new TestContext();
    this.tc.initializeTestContext();
    TransactionProcessingResultValidator resultValidator =
        new StateManagerTestValidator(tc.frameworkEntryPointAccount, List.of(2));
    MultiBlockExecutionEnvironment.builder(chainConfig, testInfo)
        .accounts(
            List.of(
                tc.initialAccounts[0],
                tc.externallyOwnedAccounts[0],
                tc.frameworkEntryPointAccount))
        .addBlock(
            List.of(
                tc.deployWithCreate2_withRevertTrigger(
                    tc.externallyOwnedAccounts[0],
                    tc.keyPairs[0],
                    tc.frameworkEntryPointAddress,
                    "0x0000000000000000000000000000000000000000000000000000000000004312",
                    TestContext.snippetsCodeForCreate2,
                    false)))
        .transactionProcessingResultValidator(resultValidator)
        .build()
        .run();
  }
}
