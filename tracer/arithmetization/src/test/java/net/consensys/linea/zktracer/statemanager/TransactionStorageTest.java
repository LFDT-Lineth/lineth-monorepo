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

import static net.consensys.linea.zktracer.statemanager.StateManagerUtils.computeStorageFirstAndLastMapList;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.math.BigInteger;
import java.util.List;
import java.util.Map;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.MultiBlockExecutionEnvironment;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import net.consensys.linea.testing.generated.FrameworkEntrypoint;
import net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragment;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class TransactionStorageTest extends TracerTestBase {
  TestContext tc;

  @Test
  void testTransactionMapStorage(TestInfo testInfo) {
    // initialize the test context
    this.tc = new TestContext();
    this.tc.initializeTestContext();
    // prepare the transaction validator
    TransactionProcessingResultValidator resultValidator =
        new StateManagerTestValidator(
            tc.frameworkEntryPointAccount,
            // Creates, writes, reads and self-destructs generate 2 logs,
            // Reverted operations only have 1 log
            List.of(7, 7));
    // fetch the Hub metadata for the state manager maps
    // StateManagerMetadata stateManagerMetadata = Hub.stateManagerMetadata();

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
                          tc.writeToStorageCall(tc.addresses[0], 3L, 1L, false, BigInteger.ONE),
                          tc.writeToStorageCall(tc.addresses[0], 3L, 2L, false, BigInteger.ONE),
                          tc.writeToStorageCall(tc.addresses[0], 3L, 3L, false, BigInteger.ONE),
                          tc.writeToStorageCall(tc.addresses[0], 3L, 1234L, true, BigInteger.ONE),
                        }),
                    tc.newTxFromCalls(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        new FrameworkEntrypoint.ContractCall[] {
                          tc.writeToStorageCall(tc.addresses[0], 3L, 1234L, true, BigInteger.ONE),
                          tc.writeToStorageCall(tc.addresses[0], 3L, 4L, false, BigInteger.ONE),
                          tc.writeToStorageCall(tc.addresses[0], 3L, 5L, false, BigInteger.ONE),
                          tc.writeToStorageCall(tc.addresses[0], 3L, 6L, false, BigInteger.ONE),
                        })))
            .transactionProcessingResultValidator(resultValidator)
            .build();
    multiBlockEnv.run();

    // Total number of transactions
    final int txCount = multiBlockEnv.getHub().state().getUserTransactionNumber();

    // Replay the transaction's trace from the hub to compute the first and last values for the
    // storage fragment
    final List<Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>>>
        storageFirstAndLastMapList = computeStorageFirstAndLastMapList(multiBlockEnv.getHub());

    // prepare data for asserts
    // expected first values for the keys we are testing
    final EWord[][] expectedFirst = {
      {
        EWord.of(0L),
      },
      {
        EWord.of(3L),
      },
    };
    // expected last values for the keys we are testing
    final EWord[][] expectedLast = {
      {
        EWord.of(3L),
      },
      {
        EWord.of(6L),
      },
    };
    // prepare the key pairs
    final List<Map<Address, Bytes32>> addrStorageKeyMapList =
        List.of(Map.of(tc.initialAccounts[0].getAddress(), EWord.of(3L)));

    for (int txCounter = 0; txCounter < txCount; txCounter++) {
      final Map<Map<Address, Bytes32>, FragmentFirstAndLast<StorageFragment>> storageMap =
          storageFirstAndLastMapList.get(txCounter);
      for (int i = 0; i < addrStorageKeyMapList.size(); i++) {
        final FragmentFirstAndLast<StorageFragment> storageData =
            storageMap.get(addrStorageKeyMapList.get(i));
        // asserts for the first and last storage values in conflation
        // -1 due to block numbering
        assertEquals(expectedFirst[txCounter][i], storageData.getFirst().getValueCurrent());
        assertEquals(expectedLast[txCounter][i], storageData.getLast().getValueNext());
      }
    }

    System.out.println("Done");
  }
}
