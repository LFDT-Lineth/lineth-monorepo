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
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.*;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

/**
 * These tests address issue <a
 * href="https://github.com/Consensys/linea-monorepo/issues/2355">[ZkTracer] Test transactions where
 * delegations swap the recipient from executable to non-executable and vice versa</a>
 */
public class DelegationsMakingAccountsExecutableAndViceVersaTests extends TracerTestBase {

  final int authorityNonce = 0x66;

  public static final BytecodeCompiler environmentTestByteCode =
      BytecodeCompiler.newProgram(chainConfig)
          .op(SELFBALANCE)
          .op(CODESIZE)
          .op(GAS)
          .op(CALLVALUE)
          .op(ORIGIN)
          // cleaning
          .op(POP)
          .op(POP)
          .op(POP)
          .op(POP)
          .op(POP)
          //
          .op(ADDRESS)
          .op(EXTCODEHASH)
          .op(POP)
          //
          .op(ADDRESS)
          .op(EXTCODESIZE)
          .op(POP)
          // why not SELFDESTRUCT ... ?
          .op(ADDRESS)
          .op(SELFDESTRUCT);

  static final ToyAccount smcWithMeaningfulCode =
      ToyAccount.builder()
          .address(Address.fromHexString("0x900dc0de"))
          .nonce(0x40)
          .balance(Wei.fromEth(1))
          .code(environmentTestByteCode.compile())
          .build();

  static final ToyAccount smcWithGenericCode =
      ToyAccount.builder()
          .address(Address.fromHexString("0x0c0de10b"))
          .nonce(0x40)
          .balance(Wei.fromEth(1))
          .code(Bytes.fromHexString("0x600160005260206000f3"))
          .build();

  static final Address delegatedEoaAddress = Address.fromHexString("0xde1e9a7ed1");
  static final ToyAccount delegatedEoa =
      ToyAccount.builder()
          .address(delegatedEoaAddress)
          .nonce(0x80)
          .balance(Wei.fromEth(5))
          .code(pseudoDelegationCode("ff".repeat(20)))
          .build();

  ///  sender accounts
  ////////////////////

  /**
   * The transaction recipient starts out having <b>nonempty byte code</b>, <b>delegation byte
   * code</b> to be precise. The target of the delegation being a proper smart contract i.e. an
   * account with
   *
   * <ul>
   *   <li>nonempty byte code with either
   *       <ul>
   *         <li>has size <b>≠ 23</b> or
   *         <li>has size <b>= 23</b> but doesn't start with <b>0xef0100</b>
   *       </ul>
   * </ul>
   *
   * A delegation renders the recipient of the transaction non-executable by delegating it to a
   * proper (yet uninteresting) smart contract.
   */

  /**
   * The transaction recipient starts out having <b>empty byte code</b>. A delegation renders the
   * recipient of the transaction executable by delegating it to a proper (yet uninteresting) smart
   * contract.
   */

  /**
   * The transaction recipient starts out having <b>empty byte code</b>. A delegation renders the
   * recipient of the transaction executable by delegating it to delegated EOA. This delegation
   * makes the target code "executable".
   *
   * <p><b>Note.</b> The target code that will be run is itself "delegation code" and thus starts
   * with <b>0xef</b>. Running such "delegation code" immediately raises an
   * <b>invalidOpCodeException</b> which stops execution with an exception.
   */
  @ParameterizedTest
  @MethodSource("delegationModifyingExecutabilityTestScenarios")
  void delegationToEoaMakesRecipientNonExecutableTest(
      AuthorityDelegationStatus initialAuthorityDelegationStatus,
      DelegationSuccess delegationSuccess,
      AuthorityDelegationStatus finalAuthorityDelegationStatus,
      TestInfo testInfo) {

    runTestWithParameters(
        initialAuthorityDelegationStatus,
        delegationSuccess,
        finalAuthorityDelegationStatus,
        testInfo);
  }

  @Test
  void targetedTest1(TestInfo testInfo) {
    runTestWithParameters(
        AuthorityDelegationStatus.AUTHORITY_DOES_NOT_EXIST,
        DelegationSuccess.DELEGATION_SUCCESS,
        AuthorityDelegationStatus.AUTHORITY_DOES_NOT_EXIST,
        testInfo);
  }

  @Test
  void targetedTest2(TestInfo testInfo) {
    runTestWithParameters(
        AuthorityDelegationStatus.AUTHORITY_DELEGATED_TO_EMPTY_CODE_EOA,
        DelegationSuccess.DELEGATION_SUCCESS,
        AuthorityDelegationStatus.AUTHORITY_EXISTS_NOT_DELEGATED,
        testInfo);
  }

  void runTestWithParameters(
      AuthorityDelegationStatus initialAuthorityDelegationStatus,
      DelegationSuccess delegationSuccess,
      AuthorityDelegationStatus finalAuthorityDelegationStatus,
      TestInfo testInfo) {
    final KeyPair toAccountKeyPair = new SECP256K1().generateKeyPair();
    final ToyAccount toAccount = toAccount(toAccountKeyPair, initialAuthorityDelegationStatus);

    final ToyTransaction.ToyTransactionBuilder transaction =
        ToyTransaction.builder()
            .sender(senderAccount())
            .to(toAccount)
            .keyPair(senderKeyPair)
            .gasLimit(1_000_000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .value(Wei.of(1000));

    final Address delegationAddress = delegationAddress(finalAuthorityDelegationStatus);

    transaction.addCodeDelegation(
        chainConfig.id,
        delegationAddress == null ? Address.ZERO : delegationAddress,
        delegationSuccess == DelegationSuccess.DELEGATION_SUCCESS ? toAccount.getNonce() : 0xdadaL,
        toAccountKeyPair);

    final List<ToyAccount> accounts = new ArrayList<>();
    populateAccounts(
        accounts,
        initialAuthorityDelegationStatus != AuthorityDelegationStatus.AUTHORITY_DOES_NOT_EXIST,
        toAccount);

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(accounts)
        .transaction(transaction.build())
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  void populateAccounts(List<ToyAccount> accounts, boolean insertToAccount, ToyAccount toAccount) {
    accounts.clear();
    accounts.add(smcWithMeaningfulCode);
    accounts.add(smcWithGenericCode);
    accounts.add(delegatedEoa);
    accounts.add(senderAccount());

    if (insertToAccount) {
      accounts.add(toAccount);
    }
  }

  public static Stream<Arguments> delegationModifyingExecutabilityTestScenarios() {
    final List<Arguments> argumentsList = new ArrayList<>();

    for (AuthorityDelegationStatus initialAuthorityDelegationStatus :
        AuthorityDelegationStatus.values()) {
      for (DelegationSuccess delegationSuccess : DelegationSuccess.values()) {
        for (AuthorityDelegationStatus finalAuthorityDelegationStatus :
            AuthorityDelegationStatus.values()) {

          // If delegation fails the initial and final delegation statuses ought to be the same
          if (delegationSuccess == DelegationSuccess.DELEGATION_FAILURE
              && initialAuthorityDelegationStatus != finalAuthorityDelegationStatus) {
            continue;
          }

          // delegations can't evict an account from the state
          // (a change in delegation status forces a nonce update)
          if (initialAuthorityDelegationStatus.authorityMustExist()
              && !finalAuthorityDelegationStatus.authorityMustExist()) {
            continue;
          }

          argumentsList.add(
              Arguments.of(
                  initialAuthorityDelegationStatus,
                  delegationSuccess,
                  finalAuthorityDelegationStatus));
        }
      }
    }

    return argumentsList.stream();
  }

  private ToyAccount toAccount(KeyPair keyPair, AuthorityDelegationStatus initialStatus) {

    final Address toAddress = Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));

    if (initialStatus == AuthorityDelegationStatus.AUTHORITY_DOES_NOT_EXIST) {
      return ToyAccount.builder()
          .address(toAddress)
          .nonce(0)
          .balance(Wei.ZERO)
          .code(Bytes.EMPTY)
          .build();
    }

    final Address delegationAddress =
        switch (initialStatus) {
          case AUTHORITY_EXISTS_NOT_DELEGATED -> null;
          case AUTHORITY_DELEGATED_TO_EMPTY_CODE_EOA -> Address.fromHexString("0x0e0a");
          case AUTHORITY_DELEGATED_TO_DELEGATED -> delegatedEoa.getAddress();
          case AUTHORITY_DELEGATED_TO_SMC -> smcWithMeaningfulCode.getAddress();
          case AUTHORITY_DELEGATED_TO_SMC_ALT -> smcWithGenericCode.getAddress();
          case AUTHORITY_DELEGATED_TO_PRC -> Address.BLS12_G1ADD;
          default -> throw new IllegalStateException("Unexpected value: " + initialStatus);
        };

    return ToyAccount.builder()
        .address(toAddress)
        .code(
            delegationAddress == null ? Bytes.EMPTY : delegationCodeFromAddress(delegationAddress))
        .nonce(authorityNonce)
        .balance(Wei.fromEth(13))
        .build();
  }

  private Address delegationAddress(AuthorityDelegationStatus finalStatus) {
    return switch (finalStatus) {
      case AUTHORITY_DOES_NOT_EXIST -> null;
      case AUTHORITY_EXISTS_NOT_DELEGATED -> Address.ZERO;
      case AUTHORITY_DELEGATED_TO_EMPTY_CODE_EOA -> Address.fromHexString("0x0e0a");
      case AUTHORITY_DELEGATED_TO_DELEGATED -> delegatedEoa.getAddress();
      case AUTHORITY_DELEGATED_TO_SMC -> smcWithMeaningfulCode.getAddress();
      case AUTHORITY_DELEGATED_TO_SMC_ALT -> smcWithGenericCode.getAddress();
      case AUTHORITY_DELEGATED_TO_PRC -> Address.ID;
    };
  }
}
