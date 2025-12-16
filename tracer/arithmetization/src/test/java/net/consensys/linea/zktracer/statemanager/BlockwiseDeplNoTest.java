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
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.math.BigInteger;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.MultiBlockExecutionEnvironment;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class BlockwiseDeplNoTest extends TracerTestBase {
  TestContext tc;

  @Test
  void testBlockwiseDeplNo(TestInfo testInfo) {
    // initialize the test context
    this.tc = new TestContext();
    this.tc.initializeTestContext();
    // prepare the transaction validator

    // Transaction 2 and Transaction 9 (starts at tx 0) are a create2 after a self-destruct
    // From Cancun and on, if the self-destruct doesn't happen in the same transaction, the contract
    // is not deleted and the following create2 cannot happen.
    // Instead of 2 logs for Creates (1 CallExecuted and 1 ContractCreated), we are left with 1 log
    // only which is CallExecuted (See TestingBase.sol contract)
    final int nbLogsForTransaction2 = isPostCancun(fork) ? 1 : 2;
    final int nbLogsForTransaction9 = isPostCancun(fork) ? 1 : 2;

    TransactionProcessingResultValidator resultValidator =
        new StateManagerTestValidator(
            tc.frameworkEntryPointAccount,
            // Creates, writes, reads and self-destructs generate 2 logs,
            // Reverted operations only have 1 log
            List.of(2, 2, nbLogsForTransaction2, 2, 2, 2, 2, 2, 1, nbLogsForTransaction9, 1));

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
                    // transaction 0 : deploy at newAddresses[0] with create2 with salts[0]
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[0],
                        TestContext.snippetsCodeForCreate2,
                        false),
                    // transaction 1 : self-destruct the contract at newAddresses[0]
                    tc.selfDestruct(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[0],
                        tc.frameworkEntryPointAddress,
                        false,
                        BigInteger.ONE),
                    // transaction 2 : attempt to deploy again at newAddresses[0] with create2 with
                    // salts[0] if fork allows
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[0],
                        TestContext.snippetsCodeForCreate2,
                        false),
                    // transaction 3
                    tc.selfDestruct(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[0],
                        tc.frameworkEntryPointAddress,
                        false,
                        BigInteger.ONE)))
            // Block 2
            .addBlock(
                List.of(
                    // transaction 4
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[1],
                        TestContext.snippetsCodeForCreate2,
                        false),
                    // transaction 5
                    tc.selfDestruct(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[1],
                        tc.frameworkEntryPointAddress,
                        false,
                        BigInteger.ONE)))
            // Block 3
            .addBlock(
                List.of( // test some reverted calls
                    // transaction 6 : deploy at newAddresses[2] with create2 with salts[2]
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[2],
                        TestContext.snippetsCodeForCreate2,
                        false),
                    // transaction 7 : self-destruct the contract at newAddresses[2]
                    tc.selfDestruct(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[2],
                        tc.frameworkEntryPointAddress,
                        false,
                        BigInteger.ONE),
                    // transaction 8
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[2],
                        TestContext.snippetsCodeForCreate2,
                        true),
                    // transaction 9 : attempt to deploy again at newAddresses[2] with create2 with
                    // salts[2]
                    tc.deployWithCreate2_withRevertTrigger(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.frameworkEntryPointAddress,
                        tc.salts[2],
                        TestContext.snippetsCodeForCreate2,
                        false),
                    // transaction 10
                    tc.selfDestruct(
                        tc.externallyOwnedAccounts[0],
                        tc.keyPairs[0],
                        tc.newAddresses[2],
                        tc.frameworkEntryPointAddress,
                        true,
                        BigInteger.ONE)
                    // since the last self-destruct gets reverted, the last call will not increase
                    // the deplNo
                    ))
            .transactionProcessingResultValidator(resultValidator)
            .build();

    multiBlockEnv.run();

    // As the state manager feature is not available anymore, we are not able to keep track of the
    // deployment number per address
    // However, in the trace we have all the sections and fragments to replay and record the values
    // that the state manager used to keep track of while tracing was ongoing

    // Map to keep track of the deployment number per address
    Map<Address, Map<Integer, Integer>> minDeplNoBlock = new HashMap<>();
    Map<Address, Map<Integer, Integer>> maxDeplNoBlock = new HashMap<>();

    // We count the number of transactions in the hub, here we have 11
    int txCount = multiBlockEnv.getHub().state().getUserTransactionNumber();
    for (int txNb = 0; txNb < txCount; txNb++) {
      // Relative block number is constant per transaction
      // Tx number starts from 1
      int relBlokNo =
          multiBlockEnv
              .getHub()
              .txStack()
              .getByAbsoluteTransactionNumber(txNb + 1)
              .getRelativeBlockNumber();
      // We retrieve the trace section list for each transaction
      List<TraceSection> traceSectionList =
          multiBlockEnv.getHub().state().getUserTransaction(txNb + 1).traceSections().trace();
      // For each trace section
      for (TraceSection traceSection : traceSectionList) {
        // We iterate over the fragments
        for (TraceFragment traceFragment : traceSection.fragments()) {
          // We cast them to AccountFragment
          // If an exception occurs, it means the Fragment is not an AccountFragment so we
          // disregard it and continue
          try {
            AccountFragment accountFragment = (AccountFragment) traceFragment;
            Address address = accountFragment.oldState().address();
            int deplNo = accountFragment.newState().deploymentNumber();
            updateDeplNoBlockMaps(address, relBlokNo, deplNo, minDeplNoBlock, maxDeplNoBlock);
          } catch (Exception e) {
            // ignore
          }
        }
      }
    }

    // prepare data for asserts
    // expected first values for the keys we are testing
    int noBlocks = 3;
    Integer[][] expectedMin = {
      {1, null, null},
      {null, 1, null},
      {null, null, 1},
    };
    // expected last values for the keys we are testing
    // For Block1, the sequence is create2, self-destruct, create2, self-destruct
    // From Cancun and on, only the first create2 increments the deployment number, so max deplNo is
    // 1 i/o 4
    int maxDeplNoBlock1 = isPostCancun(fork) ? 1 : 4;
    // For Block2, the sequence is create2, self-destruct
    // From Cancun and on, only the first create2 increments the deployment number, so max deplNo is
    // 1 i/o 2
    int maxDeplNoBlock2 = isPostCancun(fork) ? 1 : 2;
    // For Block2, the sequence is create2, self-destruct, create2 with revert trigger, create2,
    // self-destruct
    // From Cancun and on, only the first create2 increments the deployment number, so max deplNo is
    // 1 i/o 4
    int maxDeplNoBlock3 = isPostCancun(fork) ? 1 : 4;
    Integer[][] expectedMax = {
      {
        maxDeplNoBlock1, null, null,
      },
      {
        null, maxDeplNoBlock2, null,
      },
      {null, null, maxDeplNoBlock3},
    };
    // prepare the key pairs
    Address[] keys = {
      tc.newAddresses[0], tc.newAddresses[1], tc.newAddresses[2],
    };

    // blocks are numbered starting from 1
    for (int block = 1; block <= noBlocks; block++) {
      for (int i = 0; i < keys.length; i++) {
        Integer minNo = minDeplNoBlock.get(keys[i]).get(block);
        Integer maxNo = maxDeplNoBlock.get(keys[i]).get(block);
        // asserts for the first and last storage values in conflation
        // -1 due to block numbering
        assertEquals(expectedMin[block - 1][i], minNo);
        assertEquals(expectedMax[block - 1][i], maxNo);
      }
    }

    System.out.println("Done");
  }

  public void updateDeplNoBlockMaps(
      Address address,
      int blockNumber,
      int currentDeplNo,
      Map<Address, Map<Integer, Integer>> minDeplNoBlock,
      Map<Address, Map<Integer, Integer>> maxDeplNoBlock) {
    if (minDeplNoBlock.containsKey(address)
        && minDeplNoBlock.get(address).containsKey(blockNumber)) {
      // the maps already contain deployment info for this address, and this is not the first one in
      // the block
      // since it is not the first, we do not update the minDeplNoBlock
      // but we update the maxDeplNoBlock
      maxDeplNoBlock.put(address, new HashMap<>());
      maxDeplNoBlock.get(address).put(blockNumber, currentDeplNo);
    } else {
      // this is the first time we have a deployment at this address in the block
      minDeplNoBlock.put(address, new HashMap<>());
      minDeplNoBlock.get(address).put(blockNumber, currentDeplNo);
      maxDeplNoBlock.put(address, new HashMap<>());
      maxDeplNoBlock.get(address).put(blockNumber, currentDeplNo);
    }
  }
}
