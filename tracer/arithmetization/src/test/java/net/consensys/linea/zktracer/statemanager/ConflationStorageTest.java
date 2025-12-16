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
import static org.junit.jupiter.api.Assertions.*;

import java.math.BigInteger;
import java.util.*;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.*;
import net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragment;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class ConflationStorageTest extends TracerTestBase {
  TestContext tc;

  @Test
  void testConflationMapStorage(TestInfo testInfo) {
    // initialize the test context
    this.tc = new TestContext();
    this.tc.initializeTestContext();
    // prepare the transaction validator

    // Transaction 14 (starts at tx 0) is a create2 after a self-destruct
    // From Cancun and on, if the self-destruct doesn't happen in the same transaction, the contract
    // is not deleted and the following create2 cannot happen.
    // Instead of 2 logs for Creates (1 CallExecuted and 1 ContractCreated), we are left with 1 log
    // only which is CallExecuted
    // (See TestingBase.sol contract)
    final int nbLogsForTransaction14 = isPostCancun(fork) ? 1 : 2;

    TransactionProcessingResultValidator resultValidator =
        new StateManagerTestValidator(
            tc.frameworkEntryPointAccount,
            // Creates, writes, reads and self-destructs generate 2 logs,
            // Reverted operations only have 1 log
            List.of(
                2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, nbLogsForTransaction14, 2, 2, 2, 1, 1));

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
            // Transaction 0 : test storage operations for an account prexisting in the state
            .addBlock(
                List.of(
                    tc.writeToStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        123L,
                        8L,
                        false,
                        BigInteger.ONE)))
            // Transaction 1
            .addBlock(
                List.of(
                    tc.readFromStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        123L,
                        false,
                        BigInteger.ONE)))
            // Transaction 2
            .addBlock(
                List.of(
                    tc.writeToStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        123L,
                        10L,
                        false,
                        BigInteger.ONE)))
            // Transaction 3
            .addBlock(
                List.of(
                    tc.readFromStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        123L,
                        false,
                        BigInteger.ONE)))
            // Transaction 4
            .addBlock(
                List.of(
                    tc.writeToStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.addresses[0],
                        123L,
                        15L,
                        false,
                        BigInteger.ONE)))
            // Transaction 5 : deploy another account and perform storage operations on it
            .addBlock(
                List.of(
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[0],
                        TestContext.snippetsCodeForCreate2,
                        false)))
            // Transaction 6
            .addBlock(
                List.of(
                    tc.writeToStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[0],
                        345L,
                        20L,
                        false,
                        BigInteger.ONE)))
            // Transaction 7
            .addBlock(
                List.of(
                    tc.readFromStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[0],
                        345L,
                        false,
                        BigInteger.ONE)))
            // Transaction 8
            .addBlock(
                List.of(
                    tc.writeToStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[0],
                        345L,
                        40L,
                        false,
                        BigInteger.ONE)))
            // Transaction 9 : deploy another account and self destruct it at the end, redeploy it
            // and change the
            // storage again
            // the salt will be the same twice in a row, which will be on purpose
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
            // Transaction 10
            .addBlock(
                List.of(
                    tc.writeToStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[1],
                        400L,
                        12L,
                        false,
                        BigInteger.ONE)))
            // Transaction 11
            .addBlock(
                List.of(
                    tc.readFromStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[1],
                        400L,
                        false,
                        BigInteger.ONE)))
            // Transaction 12
            .addBlock(
                List.of(
                    tc.writeToStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[1],
                        400L,
                        13L,
                        false,
                        BigInteger.ONE)))
            // Transaction 13 : self-destruct the account with address newAddresses[1], deployed in
            // transaction 9
            .addBlock(
                List.of(
                    tc.selfDestruct(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[1],
                        tc.frameworkEntryPointAddress,
                        false,
                        BigInteger.ONE)))
            // Transaction 14 : attempt to redeploy the account at newAddresses[1] with salt[1] if
            // fork allows
            .addBlock(
                List.of(
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[1],
                        TestContext.snippetsCodeForCreate2,
                        false)))
            // Transaction 15
            .addBlock(
                List.of(
                    tc.writeToStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[1],
                        400L,
                        99L,
                        false,
                        BigInteger.ONE)))
            // Transaction 16 : deploy a new account and check revert operations on it
            .addBlock(
                List.of(
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[2],
                        TestContext.snippetsCodeForCreate2,
                        false)))
            // Transaction 17
            .addBlock(
                List.of(
                    tc.writeToStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[2],
                        500L,
                        23L,
                        false,
                        BigInteger.ONE)))
            // Transaction 18
            .addBlock(
                List.of(
                    tc.writeToStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[2],
                        500L,
                        53L,
                        true,
                        BigInteger.ONE))) // revert flag on
            // Transaction 19
            .addBlock(
                List.of(
                    tc.writeToStorage(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[2],
                        500L,
                        63L,
                        true,
                        BigInteger.ONE))) // revert flag on
            .transactionProcessingResultValidator(resultValidator)
            .build();

    multiBlockEnv.run();

    // Replay the transaction's trace from the hub to compute the first and last values for the
    // storage fragment
    List<Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>>>
        storageFirstAndLastMapList = computeStorageFirstAndLastMapList(multiBlockEnv.getHub());

    // Replay trace from the hub to compute blockMapStorage
    Map<Map<Address, Bytes32>, Map<Integer, FragmentFirstAndLast<StorageFragment>>>
        blockMapStorage =
            computeBlockMapStorage(multiBlockEnv.getHub(), storageFirstAndLastMapList);

    Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>> conflationMapStorage =
        computeConflationMapStorage(
            multiBlockEnv.getHub(), storageFirstAndLastMapList, blockMapStorage);

    // prepare data for asserts
    // expected first values for the keys we are testing
    EWord[] expectedFirst = {EWord.of(0L), EWord.of(0), EWord.of(0), EWord.of(0)};
    // expected last values for the keys we are testing
    EWord[] expectedLast = {EWord.of(15L), EWord.of(40L), EWord.of(99L), EWord.of(23L)};
    // prepare the key pairs
    List<Map<Address, EWord>> keys =
        List.of(
            Map.of(tc.initialAccounts[0].getAddress(), EWord.of(123L)),
            Map.of(tc.newAddresses[0], EWord.of(345L)),
            Map.of(tc.newAddresses[1], EWord.of(400L)),
            Map.of(tc.newAddresses[2], EWord.of(500L)));

    for (int i = 0; i < keys.size(); i++) {
      FragmentFirstAndLast<StorageFragment> storageData = conflationMapStorage.get(keys.get(i));
      // asserts for the first and last storage values in conflation
      assertEquals(expectedFirst[i], storageData.getFirst().getValueCurrent());
      assertEquals(expectedLast[i], storageData.getLast().getValueNext());
    }
    System.out.println("Done");
  }
}
