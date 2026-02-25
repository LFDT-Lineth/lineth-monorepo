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
package net.consensys.linea.zktracer.delegation;

import static net.consensys.linea.zktracer.delegation.Utils.*;
import static net.consensys.linea.zktracer.delegation.Utils.AuthorityExistence.*;
import static net.consensys.linea.zktracer.delegation.Utils.AuthorityInAccessList.*;
import static net.consensys.linea.zktracer.delegation.Utils.RequiresEvmExecution;
import static net.consensys.linea.zktracer.delegation.Utils.RequiresEvmExecution.REQUIRES_EVM_EXECUTION;
import static net.consensys.linea.zktracer.delegation.Utils.TouchAuthority;
import static net.consensys.linea.zktracer.delegation.Utils.TouchAuthority.EXECUTION_DOES_NOT_TOUCH_AUTHORITY;
import static net.consensys.linea.zktracer.delegation.Utils.TouchMethod;
import static net.consensys.linea.zktracer.delegation.Utils.TouchMethod.BALANCE;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import org.hyperledger.besu.datatypes.AccessListEntry;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

/**
 * These tests address issue <a
 * href="https://github.com/Consensys/linea-monorepo/issues/2437">[ZkTracer] Test delegation lists +
 * access lists</a>
 */
public class DelegationAndAccessListTests extends TracerTestBase {

  @ParameterizedTest
  @MethodSource("delegationAndAccessListScenarios")
  void delegationsAndAccessListTests(
      ChainIdValidity chainIdValidity,
      AuthorityInAccessList authorityInAccessList,
      RequiresEvmExecution requiresEvmExecution,
      AuthorityExistence authorityExistence,
      TouchAuthority touchAuthority,
      TouchMethod touchMethod,
      TestInfo testInfo) {

    final ToyTransaction.ToyTransactionBuilder delegationTxBuilder =
        ToyTransaction.builder()
            .sender(senderAccount())
            .to(smcAccount())
            .keyPair(senderKeyPair)
            .gasLimit(300_000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .value(Wei.of(1000));

    final ToyAccount smcAccount =
        ToyAccount.builder().balance(Wei.fromEth(22)).nonce(3).address(smcAddress).build();

    runTestWithParameters(
        chainIdValidity,
        authorityInAccessList,
        requiresEvmExecution,
        authorityExistence,
        touchAuthority,
        touchMethod,
        delegationTxBuilder,
        smcAccount,
        testInfo);
  }

  @Test
  void targetedTest(TestInfo testInfo) {

    final ToyAccount smcAccount1 = smcAccount();
    final ToyTransaction.ToyTransactionBuilder tx1 = tx();
    runTestWithParameters(
        ChainIdValidity.DELEGATION_CHAIN_ID_IS_ZERO,
        AUTHORITY_NOT_IN_ACCESS_LIST,
        REQUIRES_EVM_EXECUTION,
        AUTHORITY_DOES_NOT_EXIST,
        EXECUTION_DOES_NOT_TOUCH_AUTHORITY,
        BALANCE,
        tx1,
        smcAccount1,
        testInfo);

    final ToyAccount smcAccount2 = smcAccount();
    final ToyTransaction.ToyTransactionBuilder tx2 = tx();
    runTestWithParameters(
        ChainIdValidity.DELEGATION_CHAIN_ID_IS_NETWORK_CHAIN_ID,
        AUTHORITY_NOT_IN_ACCESS_LIST,
        REQUIRES_EVM_EXECUTION,
        AUTHORITY_DOES_NOT_EXIST,
        EXECUTION_DOES_NOT_TOUCH_AUTHORITY,
        BALANCE,
        tx2,
        smcAccount2,
        testInfo);
  }

  void runTestWithParameters(
      ChainIdValidity chainIdValidity,
      AuthorityInAccessList authorityInAccessList,
      RequiresEvmExecution requiresEvmExecution,
      AuthorityExistence authorityExistence,
      TouchAuthority touchAuthority,
      TouchMethod touchMethod,
      ToyTransaction.ToyTransactionBuilder delegationTxBuilder,
      ToyAccount smcAccount,
      TestInfo testInfo) {

    if (authorityInAccessList == AuthorityInAccessList.AUTHORITY_IN_ACCESS_LIST) {
      List<AccessListEntry> accessList = new ArrayList<>();
      accessList.add(AccessListEntry.createAccessListEntry(authorityAddress, new ArrayList<>()));
      delegationTxBuilder.accessList(accessList);
    }

    delegationTxBuilder.clearCodeDelegations();
    delegationTxBuilder.addCodeDelegation(
        chainIdValidity.tupleChainId(),
        delegationAddress,
        authorityExistence.tupleNonce(),
        authorityKeyPair);

    smcAccount.setCode(
        codeThatMayTouchAuthority(requiresEvmExecution, touchAuthority, touchMethod).compile());

    final List<ToyAccount> accounts = new ArrayList<>();
    accounts.add(senderAccount());
    accounts.add(smcAccount);
    if (authorityExistence == Utils.AuthorityExistence.AUTHORITY_EXISTS) {
      accounts.add(authorityAccount());
    }

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(accounts)
        .transaction(delegationTxBuilder.build())
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  private static Stream<Arguments> delegationAndAccessListScenarios() {
    List<Arguments> argumentsList = new ArrayList<>();

    for (ChainIdValidity chainIdValidity : ChainIdValidity.values()) {
      for (AuthorityInAccessList authorityInAccessList : AuthorityInAccessList.values()) {
        for (RequiresEvmExecution transactionRequiresEvmExecution : RequiresEvmExecution.values()) {
          for (AuthorityExistence authorityExistence : AuthorityExistence.values()) {
            for (TouchAuthority touchAuthority : TouchAuthority.values()) {
              for (TouchMethod touchMethod : TouchMethod.values()) {
                argumentsList.add(
                    Arguments.of(
                        chainIdValidity,
                        authorityInAccessList,
                        transactionRequiresEvmExecution,
                        authorityExistence,
                        touchAuthority,
                        touchMethod));
              }
            }
          }
        }
      }
    }
    return argumentsList.stream();
  }
}
