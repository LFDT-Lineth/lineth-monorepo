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

import static net.consensys.linea.zktracer.statemanager.StateManagerUtils.computeAccountFirstAndLastMapList;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.math.BigInteger;
import java.util.List;
import java.util.Map;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.MultiBlockExecutionEnvironment;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import net.consensys.linea.testing.generated.FrameworkEntrypoint;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class TransactionAccountTest extends TracerTestBase {
  TestContext tc;

  @Test
  void testTransactionMapAccount(TestInfo testInfo) {
    // initialize the test context
    this.tc = new TestContext();
    this.tc.initializeTestContext();
    // prepare the transaction validator
    TransactionProcessingResultValidator resultValidator =
        new StateManagerTestValidator(
            tc.frameworkEntryPointAccount,
            // Creates, writes, reads and self-destructs generate 2 logs,
            // transfers generate 3 logs
            // Reverted operations only have 1 log
            List.of(10, 10));
    // fetch the Hub metadata for the state manager maps
    /*
            StateManagerMetadata stateManagerMetadata = Hub.stateManagerMetadata();
    */

    // prepare a multi-block execution of transactions
    final MultiBlockExecutionEnvironment multiBlockEnv =
        MultiBlockExecutionEnvironment.builder(chainConfig, testInfo)
            // initialize accounts
            .accounts(
                List.of(
                    tc.initialAccounts[0],
                    tc.externallyOwnedAccounts[0],
                    tc.initialAccounts[2],
                    tc.frameworkEntryPointAccount))
            // Block 1
            .addBlock(
                List.of(
                    tc.newTxFromCalls(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        new FrameworkEntrypoint.ContractCall[] {
                          tc.transferToCall(
                              tc.addresses[0], tc.addresses[2], 8L, false, BigInteger.ONE),
                          tc.transferToCall(
                              tc.addresses[2], tc.addresses[0], 9L, false, BigInteger.ONE),
                          tc.transferToCall(
                              tc.addresses[0], tc.addresses[2], 15L, false, BigInteger.ONE),
                          tc.transferToCall(
                              tc.addresses[2],
                              tc.addresses[0],
                              1234L,
                              true,
                              BigInteger.ONE), // revert this one
                        }),
                    tc.newTxFromCalls(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        new FrameworkEntrypoint.ContractCall[] {
                          tc.transferToCall(
                              tc.addresses[0], tc.addresses[2], 200L, false, BigInteger.ONE),
                          tc.transferToCall(
                              tc.addresses[2], tc.addresses[0], 500L, false, BigInteger.ONE),
                          tc.transferToCall(
                              tc.addresses[2],
                              tc.addresses[0],
                              1234L,
                              true,
                              BigInteger.ONE), // revert this one
                          tc.transferToCall(
                              tc.addresses[0], tc.addresses[2], 900L, false, BigInteger.ONE),
                        })))
            .transactionProcessingResultValidator(resultValidator)
            .build();

    multiBlockEnv.run();

    // Total number of transactions
    int txCount = multiBlockEnv.getHub().state().getUserTransactionNumber();

    // Replay the transaction's trace from the hub to compute the first and last values for the
    // account fragment
    List<Map<Address, FragmentFirstAndLast<AccountFragment>>> accountFirstAndLastMapList =
        computeAccountFirstAndLastMapList(multiBlockEnv.getHub());

    // prepare data for asserts
    // expected first values for the keys we are testing
    Wei[][] expectedFirst = {
      {
        TestContext.defaultBalance, TestContext.defaultBalance,
      },
      {
        TestContext.defaultBalance.subtract(8L).add(9L).subtract(15L),
        TestContext.defaultBalance.add(8L).subtract(9L).add(15L),
      }
    };

    // expected last values for the keys we are testing
    Wei[][] expectedLast = {
      {
        TestContext.defaultBalance.subtract(8L).add(9L).subtract(15L),
        TestContext.defaultBalance.add(8L).subtract(9L).add(15L),
      },
      {
        TestContext.defaultBalance
            .subtract(8L)
            .add(9L)
            .subtract(15L)
            .subtract(200L)
            .add(500L)
            .subtract(900L),
        TestContext.defaultBalance.add(8L).subtract(9L).add(15L).add(200L).subtract(500L).add(900L),
      }
    };

    // prepare the key pairs
    Address[] keys = {
      tc.initialAccounts[0].getAddress(), tc.initialAccounts[2].getAddress(),
    };

    // blocks are numbered starting from 1
    for (int txCounter = 0; txCounter < txCount; txCounter++) {
      Map<Address, FragmentFirstAndLast<AccountFragment>> accountMap =
          accountFirstAndLastMapList.get(txCounter);
      for (int i = 0; i < keys.length; i++) {
        FragmentFirstAndLast<AccountFragment> accountData = accountMap.get(keys[i]);
        // asserts for the first and last storage values in conflation
        // -1 due to block numbering
        assertEquals(expectedFirst[txCounter][i], accountData.getFirst().oldState().balance());
        assertEquals(expectedLast[txCounter][i], accountData.getLast().newState().balance());
      }
    }

    System.out.println("Done");
  }
}
