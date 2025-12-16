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

import static net.consensys.linea.zktracer.Fork.isPostCancun;
import static net.consensys.linea.zktracer.statemanager.StateManagerUtils.*;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.math.BigInteger;
import java.util.*;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.MultiBlockExecutionEnvironment;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class ConflationAccountTest extends TracerTestBase {
  TestContext tc;

  @Test
  void testConflationMapAccount(TestInfo testInfo) {
    // initialize the test context
    this.tc = new TestContext();
    this.tc.initializeTestContext();
    // prepare the transaction validator

    // Transaction 10 (starts at tx 0) is a create2 after a self-destruct
    // From Cancun and on, if the self-destruct doesn't happen in the same transaction, the contract
    // is not deleted and the following create2 cannot happen.
    // Instead of 2 logs for Creates (1 CallExecuted and 1 ContractCreated), we are left with 1 log
    // only which is CallExecuted
    // (See TestingBase.sol contract)
    final int nbLogsForTransaction10 = isPostCancun(fork) ? 1 : 2;

    final TransactionProcessingResultValidator resultValidator =
        new StateManagerTestValidator(
            tc.frameworkEntryPointAccount,
            // Creates and self-destructs generate 2 logs,
            // Transfers generate 3 logs, the 1s are for reverted operations
            List.of(3, 3, 1, 3, 2, 3, 3, 2, 3, 2, nbLogsForTransaction10, 3, 2, 1));

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
            // Transaction 0 : test account operations for an account prexisting in the state
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
            // Transaction 1
            .addBlock(
                List.of(
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[2],
                        tc.addresses[0],
                        20L,
                        false,
                        BigInteger.ONE)))
            // Transaction 2
            .addBlock(
                List.of(
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.addresses[2],
                        50L,
                        true,
                        BigInteger.ONE))) // this action is reverted
            // Transaction 3
            .addBlock(
                List.of(
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.addresses[2],
                        10L,
                        false,
                        BigInteger.ONE)))
            // Transaction 4 : deploy another account ctxt.addresses[3] and perform account
            // operations on it
            .addBlock(
                List.of(
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[0],
                        TestContext.snippetsCodeForCreate2,
                        false)))
            // Transaction 5
            .addBlock(
                List.of(
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.newAddresses[0],
                        49L,
                        false,
                        BigInteger.ONE)))
            // Transaction 6
            .addBlock(
                List.of(
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[0],
                        tc.addresses[0],
                        27L,
                        false,
                        BigInteger.ONE)))
            // Transaction 7 : deploy another account and self destruct it at the end, redeploy it
            // and change its
            // balance  again
            // Account at newAddresses[1], deployed with salt[1]
            .addBlock(
                List.of(
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[1],
                        TestContext.snippetsCodeForCreate2,
                        false)))
            // Transaction 8
            .addBlock(
                List.of(
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.newAddresses[1],
                        98L,
                        false,
                        BigInteger.ONE)))
            // Transaction 9 : self-destruct the account with newAddresses[1] deployed in
            // transaction 7
            .addBlock(
                List.of(
                    tc.selfDestruct(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[1],
                        tc.addresses[2],
                        false,
                        BigInteger.ONE)))
            // Transaction 10 : attempt to redeploy the account with salt[1] that was just
            // self-destructed if the fork
            // allows it
            .addBlock(
                List.of(
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[1],
                        TestContext.snippetsCodeForCreate2,
                        false)))
            // Transaction 11
            .addBlock(
                List.of(
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        tc.newAddresses[1],
                        123L,
                        false,
                        BigInteger.ONE)))
            // Transaction 12 : deploy a new account and check revert operations on it
            .addBlock(
                List.of(
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[2],
                        TestContext.snippetsCodeForCreate2,
                        false)))
            // Transaction 13
            .addBlock(
                List.of(
                    tc.transferTo(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[2],
                        tc.newAddresses[2],
                        1L,
                        true,
                        BigInteger.ONE)))
            .transactionProcessingResultValidator(resultValidator)
            .build();

    multiBlockEnv.run();

    // Replay the transaction's trace from the hub to compute the first and last values for the
    // account storage
    final List<Map<Address, FragmentFirstAndLast<AccountFragment>>> accountFirstAndLastMapList =
        computeAccountFirstAndLastMapList(multiBlockEnv.getHub());

    // Replay trace from the hub to compute blockMapAccount
    final Map<Address, Map<Integer, FragmentFirstAndLast<AccountFragment>>> blockMapAccount =
        computeBlockMapAccount(multiBlockEnv.getHub(), accountFirstAndLastMapList);

    final Map<Address, FragmentFirstAndLast<AccountFragment>> conflationMapAccount =
        computeConflationMapAccount(
            multiBlockEnv.getHub(), accountFirstAndLastMapList, blockMapAccount);

    // prepare data for asserts
    // expected first values for the keys we are testing
    final Wei[] expectedFirst = {
      TestContext.defaultBalance, TestContext.defaultBalance, Wei.of(0L), Wei.of(0L), Wei.of(0L)
    };
    // expected last values for the keys we are testing
    final Wei[] expectedLast = {
      TestContext.defaultBalance
          .subtract(8L)
          .add(20L)
          .subtract(10L)
          .subtract(49L)
          .add(27L)
          .subtract(98L)
          .subtract(123L),
      TestContext.defaultBalance
          .add(8L)
          .subtract(20L)
          .add(10L)
          .add(98L), // 98L obtained from the self destruct of the account at ctxt.addresses[4]
      Wei.of(0L).add(49L).subtract(27L),
      Wei.of(123L),
      Wei.of(0L)
    };

    // prepare the key pairs
    final Address[] keys = {
      tc.initialAccounts[0].getAddress(),
      tc.initialAccounts[2].getAddress(),
      tc.newAddresses[0],
      tc.newAddresses[1],
      tc.newAddresses[2]
    };

    for (int i = 0; i < keys.length; i++) {
      final FragmentFirstAndLast<AccountFragment> accountData = conflationMapAccount.get(keys[i]);
      // asserts for the first and last storage values in conflation
      assertEquals(expectedFirst[i], accountData.getFirst().oldState().balance());
      assertEquals(expectedLast[i], accountData.getLast().newState().balance());
    }

    System.out.println("Done");
  }
}
