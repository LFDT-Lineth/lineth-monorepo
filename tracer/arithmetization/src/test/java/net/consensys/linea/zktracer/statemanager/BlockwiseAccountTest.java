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
import static net.consensys.linea.zktracer.statemanager.StateManagerUtils.computeBlockMapAccount;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.math.BigInteger;
import java.util.List;
import java.util.Map;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.MultiBlockExecutionEnvironment;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class BlockwiseAccountTest extends TracerTestBase {
  TestContext tc;

  @Test
  void testBlockwiseMapAccount(TestInfo testInfo) {
    // initialize the test context
    this.tc = new TestContext();
    this.tc.initializeTestContext();
    // prepare the transaction validator
    TransactionProcessingResultValidator resultValidator =
        new StateManagerTestValidator(
            tc.frameworkEntryPointAccount,
            // Creates, writes, reads and self-destructs generate 2 logs,
            // Reverted operations only have 1 log
            List.of(3, 3, 3, 3, 3, 3, 3, 3, 3, 1));
    // fetch the Hub metadata for the state manager maps

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
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.addresses[2],
                        1L,
                        false,
                        BigInteger.ONE),
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[2],
                        tc.addresses[0],
                        2L,
                        false,
                        BigInteger.ONE),
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.addresses[2],
                        5L,
                        false,
                        BigInteger.ONE)))
            .addBlock(
                List.of(
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.addresses[2],
                        10L,
                        false,
                        BigInteger.ONE),
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[2],
                        tc.addresses[0],
                        20L,
                        false,
                        BigInteger.ONE),
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.addresses[2],
                        50L,
                        false,
                        BigInteger.ONE)))
            .addBlock(
                List.of(
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.addresses[2],
                        100L,
                        false,
                        BigInteger.ONE),
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[2],
                        tc.addresses[0],
                        200L,
                        false,
                        BigInteger.ONE),
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.addresses[2],
                        500L,
                        false,
                        BigInteger.ONE),
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.addresses[2],
                        1234L,
                        true,
                        BigInteger.ONE)))
            .transactionProcessingResultValidator(resultValidator)
            .build();

    multiBlockEnv.run();

    // Replay the transaction's trace from the hub to compute the first and last values for the
    // account storage
    List<Map<Address, FragmentFirstAndLast<AccountFragment>>> accountFirstAndLastMapList =
        computeAccountFirstAndLastMapList(multiBlockEnv.getHub());

    // Replay trace from the hub to compute blockMapAccount
    Map<Address, Map<Integer, FragmentFirstAndLast<AccountFragment>>> blockMapAccount =
        computeBlockMapAccount(multiBlockEnv.getHub(), accountFirstAndLastMapList);

    // prepare data for asserts
    // expected first values for the keys we are testing
    int noBlocks = 3;
    Wei[][] expectedFirst = {
      {TestContext.defaultBalance, TestContext.defaultBalance},
      {
        TestContext.defaultBalance.subtract(1L).add(2L).subtract(5L),
        TestContext.defaultBalance.add(1L).subtract(2L).add(5L),
      },
      {
        TestContext.defaultBalance
            .subtract(1L)
            .add(2L)
            .subtract(5L)
            .subtract(10L)
            .add(20L)
            .subtract(50L),
        TestContext.defaultBalance.add(1L).subtract(2L).add(5L).add(10L).subtract(20L).add(50L),
      },
    };
    // expected last values for the keys we are testing
    Wei[][] expectedLast = {
      {
        TestContext.defaultBalance.subtract(1L).add(2L).subtract(5L),
        TestContext.defaultBalance.add(1L).subtract(2L).add(5L),
      },
      {
        TestContext.defaultBalance
            .subtract(1L)
            .add(2L)
            .subtract(5L)
            .subtract(10L)
            .add(20L)
            .subtract(50L),
        TestContext.defaultBalance.add(1L).subtract(2L).add(5L).add(10L).subtract(20L).add(50L),
      },
      {
        TestContext.defaultBalance
            .subtract(1L)
            .add(2L)
            .subtract(5L)
            .subtract(10L)
            .add(20L)
            .subtract(50L)
            .subtract(100L)
            .add(200L)
            .subtract(500L),
        TestContext.defaultBalance
            .add(1L)
            .subtract(2L)
            .add(5L)
            .add(10L)
            .subtract(20L)
            .add(50L)
            .add(100L)
            .subtract(200L)
            .add(500L)
      },
    };
    // prepare the key pairs
    Address[] keys = {
      tc.initialAccounts[0].getAddress(), tc.initialAccounts[2].getAddress(),
    };

    // blocks are numbered starting from 1
    for (int block = 1; block <= noBlocks; block++) {
      for (int i = 0; i < keys.length; i++) {
        FragmentFirstAndLast<AccountFragment> accountData = blockMapAccount.get(keys[i]).get(block);
        // asserts for the first and last storage values in conflation
        // -1 due to block numbering
        assertEquals(expectedFirst[block - 1][i], accountData.getFirst().oldState().balance());
        assertEquals(expectedLast[block - 1][i], accountData.getLast().newState().balance());
      }
    }

    System.out.println("Done");
  }
}
