/*
 * Copyright ConsenSys Inc.
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

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

/**
 * These tests address issue <a
 * href="https://github.com/Consensys/linea-monorepo/issues/2322">[ZkTracer] Test refunds for
 * EIP-7702</a>
 */
public class AccountDelegationAndRevertTests extends TracerTestBase {

  /**
   * We require tests like so: mono transaction block contains a single type 4 transaction.
   *
   * <p>this tx has 1 delegation with all combinations of the following
   *
   * <ul>
   *   <li>delegations are valid: <b>[yes / no]</b>
   *   <li>(for valid delegations) authority exists: <b>[yes / no]</b>
   *   <li>TX_REQUIRES_REQUIRES_EVM_EXECUTION: <b>[yes / no]</b>
   *   <li>If yes: tx
   *       <ul>
   *         <li>reverts: <b>[yes / no]</b>
   *         <li>tx incurs another refund (say: a single SSTORE that resets storage): <b>[yes /
   *             no]</b>
   *       </ul>
   *   <li>if no: no further requirements
   * </ul>
   */
  @ParameterizedTest
  @MethodSource("delegatesAndRevertsTestsSource")
  void delegatesAndRevertsTest(scenario scenario, TestInfo testInfo) {

    final ToyTransaction.ToyTransactionBuilder tx = tx();
    final ToyAccount smcAccount = smcAccount();

    if (scenario == AccountDelegationAndRevertTests.scenario.DELEGATION_IS_VALID___NO) {
      // delegation is known to fail because of chainID, signature, etc
      tx.addCodeDelegation(
          chainConfig.id.and(
              Bytes.fromHexString("0x17891789178917891789178917891789178917891789178900000000")
                  .toUnsignedBigInteger()),
          Address.ZERO,
          0L,
          BigInteger.valueOf(78),
          BigInteger.valueOf(89),
          (byte) 0);
    } else {
      tx.addCodeDelegation(chainConfig.id, smcAddress, authNonce, Utils.authorityKeyPair);
      if (scenario
          != AccountDelegationAndRevertTests.scenario
              .DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___NO) {
        if (scenario
            != AccountDelegationAndRevertTests.scenario
                .DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___SI___REQUIRES_EVM_EXECUTION___NO) {
          switch (scenario) {
            case DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___SI___REQUIRES_EVM_EXECUTION___SI___TRANSACTION_REVERTS___NO___OTHER_REFUNDS___NO -> {
              smcAccount.setCode(
                  BytecodeCompiler.newProgram(chainConfig)
                      .push(1)
                      .push(2)
                      .push(3)
                      .op(OpCode.ADDMOD)
                      .op(OpCode.POP)
                      .compile());
            }
            case DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___SI___REQUIRES_EVM_EXECUTION___SI___TRANSACTION_REVERTS___NO___OTHER_REFUNDS___SI -> {
              smcAccount.setCode(
                  BytecodeCompiler.newProgram(chainConfig)
                      .push(0xc0ffee)
                      .push(0x5107) // 0x 5107 <> slot
                      .op(OpCode.SSTORE) // write nontrivial value
                      .push(0)
                      .push(0x5107)
                      .op(OpCode.SSTORE) // incur refund (reset to zero)
                      .push(0)
                      .push(0)
                      .op(OpCode.REVERT)
                      .compile());
            }
            case DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___SI___REQUIRES_EVM_EXECUTION___SI___TRANSACTION_REVERTS___SI___OTHER_REFUNDS___NO -> {
              smcAccount.setCode(
                  BytecodeCompiler.newProgram(chainConfig)
                      .push(1)
                      .push(2)
                      .push(3)
                      .op(OpCode.ADDMOD)
                      .op(OpCode.POP)
                      .push(0)
                      .push(0)
                      .op(OpCode.REVERT)
                      .compile());
            }
            case DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___SI___REQUIRES_EVM_EXECUTION___SI___TRANSACTION_REVERTS___SI___OTHER_REFUNDS___SI -> {
              smcAccount.setCode(
                  BytecodeCompiler.newProgram(chainConfig)
                      .push(0xc0ffee)
                      .push(0x5107)
                      .op(OpCode.SSTORE) // write nontrivial value
                      .push(0)
                      .push(0x5107)
                      .op(OpCode.SSTORE) // incur refund (reset to zero)
                      .push(0)
                      .push(0)
                      .op(OpCode.REVERT)
                      .compile());
            }
            default -> throw new IllegalArgumentException("Unknown scenario:" + scenario);
          }
        }
      }
    }

    final List<ToyAccount> accounts = new ArrayList<>();
    accounts.add(senderAccount());
    accounts.add(authorityAccount());
    accounts.add(smcAccount);

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(accounts)
        .transaction(tx.gasLimit(100_000L).build())
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  private static Stream<Arguments> delegatesAndRevertsTestsSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (scenario sc1 : scenario.values()) {
      arguments.add(Arguments.of(sc1));
    }
    return arguments.stream();
    // arguments.clear();
    // arguments.add(
    //     Arguments.of(
    //         scenario
    //
    // .DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___SI___REQUIRES_EVM_EXECUTION___SI___TRANSACTION_REVERTS___NO___OTHER_REFUNDS___SI));
    // return arguments.stream();
  }

  private enum scenario {
    DELEGATION_IS_VALID___NO,
    DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___NO,
    DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___SI___REQUIRES_EVM_EXECUTION___NO,
    DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___SI___REQUIRES_EVM_EXECUTION___SI___TRANSACTION_REVERTS___NO___OTHER_REFUNDS___NO,
    DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___SI___REQUIRES_EVM_EXECUTION___SI___TRANSACTION_REVERTS___NO___OTHER_REFUNDS___SI,
    DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___SI___REQUIRES_EVM_EXECUTION___SI___TRANSACTION_REVERTS___SI___OTHER_REFUNDS___NO,
    DELEGATION_IS_VALID___SI___AUTHORITY_EXISTS___SI___REQUIRES_EVM_EXECUTION___SI___TRANSACTION_REVERTS___SI___OTHER_REFUNDS___SI
  }

  @ParameterizedTest
  @MethodSource("delegationsAndRevertsFullTestSource")
  void delegationsAndRevertsFullTest(
      Utils.ChainIdValidity chainIdValidity,
      Utils.AuthorityExistence authorityExistence,
      Utils.RequiresEvmExecution requiresEvmExecution,
      Utils.TransactionReverts transactionReverts,
      Utils.OtherRefunds ExecutionAccruesOtherRefunds,
      TestInfo testInfo) {

    runTestWithParameters(
        chainIdValidity,
        authorityExistence,
        requiresEvmExecution,
        transactionReverts,
        ExecutionAccruesOtherRefunds,
        testInfo);
  }

  @Test
  void targetedTest(TestInfo testInfo) {
    runTestWithParameters(
        Utils.ChainIdValidity.DELEGATION_CHAIN_ID_IS_NETWORK_CHAIN_ID,
        Utils.AuthorityExistence.AUTHORITY_DOES_NOT_EXIST,
        Utils.RequiresEvmExecution.REQUIRES_EVM_EXECUTION,
        Utils.TransactionReverts.TRANSACTION_DOES_NOT_REVERT,
        Utils.OtherRefunds.OTHER_REFUNDS,
        testInfo);
  }

  void runTestWithParameters(
      Utils.ChainIdValidity chainIdValidity,
      Utils.AuthorityExistence authorityExistence,
      Utils.RequiresEvmExecution requiresEvmExecution,
      Utils.TransactionReverts transactionReverts,
      Utils.OtherRefunds ExecutionAccruesOtherRefunds,
      TestInfo testInfo) {

    final ToyTransaction.ToyTransactionBuilder tx = tx();
    tx.clearCodeDelegations();
    tx.addCodeDelegation(
        chainIdValidity.tupleChainId(),
        delegationAddress,
        authorityExistence.tupleNonce(),
        authorityKeyPair);

    final ToyAccount smcAccount = smcAccount();

    smcAccount.setCode(
        codeThatMayAccrueRefundsAndMayRevert(
                requiresEvmExecution, ExecutionAccruesOtherRefunds, transactionReverts)
            .compile());

    final List<ToyAccount> accounts = new ArrayList<>();
    accounts.add(senderAccount());
    accounts.add(smcAccount);
    if (authorityExistence == Utils.AuthorityExistence.AUTHORITY_EXISTS) {
      accounts.add(authorityAccount());
    }

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(accounts)
        .transaction(tx.gasLimit(100_000L).build())
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  private static Stream<Arguments> delegationsAndRevertsFullTestSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (Utils.ChainIdValidity chainIdValidity : Utils.ChainIdValidity.values()) {
      for (Utils.AuthorityExistence authorityExistence : Utils.AuthorityExistence.values()) {
        for (Utils.RequiresEvmExecution requiresEvmExecution :
            Utils.RequiresEvmExecution.values()) {

          // Note: some combinations yield effective duplicates: if no execution is required the
          // transaction cannot revert nor accrue refunds.
          if (requiresEvmExecution == Utils.RequiresEvmExecution.DOES_NOT_REQUIRE_EVM_EXECUTION) {
            arguments.add(
                Arguments.of(
                    chainIdValidity,
                    authorityExistence,
                    requiresEvmExecution,
                    Utils.TransactionReverts.TRANSACTION_DOES_NOT_REVERT,
                    Utils.OtherRefunds.NO_OTHER_REFUNDS));
            continue;
          }

          for (Utils.OtherRefunds ExecutionAccruesOtherRefunds : Utils.OtherRefunds.values()) {
            for (Utils.TransactionReverts transactionReverts : Utils.TransactionReverts.values()) {
              arguments.add(
                  Arguments.of(
                      chainIdValidity,
                      authorityExistence,
                      requiresEvmExecution,
                      transactionReverts,
                      ExecutionAccruesOtherRefunds));
            }
          }
        }
      }
    }
    return arguments.stream();
  }
}
