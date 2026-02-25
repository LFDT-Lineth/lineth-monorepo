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

package net.consensys.linea.zktracer.module.rlpauth;

import static net.consensys.linea.zktracer.Trace.LINEA_CHAIN_ID;
import static net.consensys.linea.zktracer.module.ecdata.EcDataOperation.SECP256K1N;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.math.BigInteger;
import java.util.List;
import java.util.Optional;
import java.util.stream.Stream;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.Utils;
import net.consensys.linea.zktracer.delegation.Utils.*;
import net.consensys.linea.zktracer.module.hub.section.TupleAnalysis;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.*;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.*;

@ExtendWith(UnitTestWatcher.class)
public class RlpAuthTest extends TracerTestBase {

  final long SENDER_NONCE = 42;
  final long AUTHORITY_NONCE = 1337L;
  final long DELEGATION_NONCE = 69;
  final long RECIPIENT_NONCE = 0xc0ffeeL;

  final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
  final Address senderAddress =
      Address.extract(Hash.hash(senderKeyPair.getPublicKey().getEncodedBytes()));
  final ToyAccount senderAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .nonce(SENDER_NONCE)
          .address(senderAddress)
          .build();

  final KeyPair authorityKeyPair = new SECP256K1().generateKeyPair();
  final Address authorityAddress =
      Address.extract(Hash.hash(authorityKeyPair.getPublicKey().getEncodedBytes()));
  final ToyAccount authorityAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(2))
          .nonce(AUTHORITY_NONCE)
          .address(authorityAddress)
          .build();

  static final KeyPair delegationKeyPair = new SECP256K1().generateKeyPair();
  static final Address delegationAddress =
      Address.extract(Hash.hash(delegationKeyPair.getPublicKey().getEncodedBytes()));
  final ToyAccount delegationAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(3))
          .nonce(DELEGATION_NONCE)
          .address(delegationAddress)
          .build();

  final KeyPair recipientSmcKeyPair = new SECP256K1().generateKeyPair();
  final Address recipientSmcAddress =
      Address.extract(Hash.hash(recipientSmcKeyPair.getPublicKey().getEncodedBytes()));
  final ToyAccount recipientAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(4))
          .nonce(RECIPIENT_NONCE)
          .address(recipientSmcAddress)
          .code(Bytes.fromHexString("0x5b")) // nontrivial code that does nothing
          .build();

  org.hyperledger.besu.datatypes.CodeDelegation validDelegationTuple =
      CodeDelegation.builder()
          .chainId(BigInteger.valueOf(LINEA_CHAIN_ID))
          .address(delegationAddress)
          .nonce(AUTHORITY_NONCE)
          .signAndBuild(authorityKeyPair);

  @ParameterizedTest
  @ValueSource(longs = {AUTHORITY_NONCE, AUTHORITY_NONCE + 1})
  void tupleNonceVsStateNonceTest(long nonceParam, TestInfo testInfo) {

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(recipientAccount)
            .keyPair(senderKeyPair)
            .transactionType(TransactionType.DELEGATE_CODE)
            .nonce(SENDER_NONCE)
            .addCodeDelegation(
                BigInteger.valueOf(LINEA_CHAIN_ID), delegationAddress, nonceParam, authorityKeyPair)
            .build();

    ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, authorityAccount, delegationAccount, recipientAccount))
            .transaction(tx)
            .build();
    toyExecutionEnvironmentV2.run();

    TupleAnalysis tupleAnalysis =
        toyExecutionEnvironmentV2
            .getHub()
            .rlpAuth()
            .operations()
            .getFirst()
            .authorizationFragment()
            .tupleAnalysis();
    if (nonceParam == AUTHORITY_NONCE) {
      assertEquals(TupleAnalysis.TUPLE_IS_VALID, tupleAnalysis);
    } else {
      assertEquals(TupleAnalysis.TUPLE_FAILS_DUE_TO_NONCE_MISMATCH, tupleAnalysis);
    }
  }

  @ParameterizedTest
  @ValueSource(ints = {0, LINEA_CHAIN_ID, LINEA_CHAIN_ID + 1})
  void chainIdVsNetworkChainIdTest(int chainId, TestInfo testInfo) {

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(recipientAccount)
            .keyPair(senderKeyPair)
            .transactionType(TransactionType.DELEGATE_CODE)
            .nonce(SENDER_NONCE)
            .addCodeDelegation(
                BigInteger.valueOf(chainId), delegationAddress, AUTHORITY_NONCE, authorityKeyPair)
            .build();

    ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, authorityAccount, delegationAccount, recipientAccount))
            .transaction(tx)
            .build();
    toyExecutionEnvironmentV2.run();

    TupleAnalysis tupleAnalysis =
        toyExecutionEnvironmentV2
            .getHub()
            .rlpAuth()
            .operations()
            .getFirst()
            .authorizationFragment()
            .tupleAnalysis();
    if (chainId == 0 || chainId == LINEA_CHAIN_ID) {
      assertEquals(TupleAnalysis.TUPLE_IS_VALID, tupleAnalysis);
    } else {
      assertEquals(TupleAnalysis.TUPLE_FAILS_CHAIN_ID_CHECK, tupleAnalysis);
    }
  }

  static Stream<Arguments> sVsHalfCurveOrderTestArguments() {
    final List<Arguments> argumentsList = new java.util.ArrayList<>();
    for (STampering tampering : STampering.values()) {
      for (SFlipping flip : SFlipping.values()) {
        argumentsList.add(Arguments.of(tampering, flip));
      }
    }

    return argumentsList.stream();
  }

  @ParameterizedTest
  @MethodSource("sVsHalfCurveOrderTestArguments")
  void sVsHalfCurveOrderTest(STampering tamperWithS, SFlipping flipS, TestInfo testInfo) {
    // s <= secp256k1n/2 is a requirement

    // We flip s to flippedS = n - s for a valid signature of a delegation tuple,
    // which is also a valid signature but with flippedS > n/2 due to malleability,
    // and we check that the tuple validity is TUPLE_ANALYSIS_FAILS_S_RANGE_CHECK

    // s <= n/2
    BigInteger s = validDelegationTuple.s(); // valid
    BigInteger sMinusOne = s.subtract(BigInteger.ONE); // invalid
    assertTrue(
        sMinusOne.signum()
            > 0); // extremely unlikely, but we check to ensure s-1 is still within the valid range

    // s > n/2
    BigInteger flippedS = SECP256K1N.toBigInteger().subtract(validDelegationTuple.s()); // valid
    BigInteger flippedSPlusOne = flippedS.add(BigInteger.ONE); // invalid
    assertTrue(
        flippedSPlusOne.compareTo(SECP256K1N.toBigInteger())
            < 0); // extremely unlikely, but we check to ensure s+1 is still within the valid range

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(recipientAccount)
            .keyPair(senderKeyPair)
            .transactionType(TransactionType.DELEGATE_CODE)
            .nonce(SENDER_NONCE)
            .addCodeDelegation(
                BigInteger.valueOf(LINEA_CHAIN_ID),
                delegationAddress,
                AUTHORITY_NONCE,
                validDelegationTuple.r(),
                tamperWithS == STampering.NOT_TAMPERED_WITH
                    ? (flipS == SFlipping.FLIPPED ? flippedS : s)
                    : (flipS == SFlipping.FLIPPED ? flippedSPlusOne : sMinusOne),
                validDelegationTuple.v())
            .build();

    ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, authorityAccount, delegationAccount, recipientAccount))
            .transaction(tx)
            .build();
    toyExecutionEnvironmentV2.run();

    TupleAnalysis tupleAnalysis =
        toyExecutionEnvironmentV2
            .getHub()
            .rlpAuth()
            .operations()
            .getFirst()
            .authorizationFragment()
            .tupleAnalysis();

    if (flipS == SFlipping.FLIPPED) {
      assertEquals(TupleAnalysis.TUPLE_FAILS_S_RANGE_CHECK, tupleAnalysis);
      return;
    }

    // invalid signatures
    if (tamperWithS == STampering.TAMPERED_WITH) {
      final Optional<Address> authority =
          toyExecutionEnvironmentV2
              .getHub()
              .rlpAuth()
              .operations()
              .getFirst()
              .authorizationFragment()
              .delegation()
              .authorizer();
      if (authority.isPresent()) {
        assertEquals(TupleAnalysis.TUPLE_FAILS_DUE_TO_NONCE_MISMATCH, tupleAnalysis);
      } else {
        assertEquals(TupleAnalysis.TUPLE_FAILS_TO_RECOVER_AUTHORITY_ADDRESS, tupleAnalysis);
      }
    } else {
      assertEquals(TupleAnalysis.TUPLE_IS_VALID, tupleAnalysis);
    }
  }

  @Getter
  @RequiredArgsConstructor
  enum CodeScenario {
    EMPTY_CODE(""), // valid
    NON_EMPTY_CODE("0x5b"), // nontrivial code that does nothing, non-valid
    DELEGATED_TO_EXISTENT_EOA_CODE(Utils.addDelegationPrefixToAddress(delegationAddress)), // valid
    DELEGATED_TO_NON_EXISTENT_EOA_CODE(
        Utils.addDelegationPrefixToAddress(
            Address.fromHexString("0x1234567890123456789012345678901234567890"))), // valid
    DELEGATED_TO_PRECOMPILE_CODE(Utils.addDelegationPrefixToAddress(Address.ECREC)); // valid

    private final String code;
  }

  @ParameterizedTest
  @EnumSource(value = CodeScenario.class) // to filter , names = {"EMPTY_CODE","NON_EMPTY_CODE"})
  void codeOfAuthorityScenariosTest(CodeScenario codeScenario, TestInfo testInfo) {
    // this authority account is used instead of the one defined at the class level for this test
    // as we want to vary its code according to the scenario
    final ToyAccount authorityAccountWithNonEmptyCode =
        ToyAccount.builder()
            .balance(Wei.fromEth(2))
            .nonce(AUTHORITY_NONCE)
            .address(authorityAddress)
            .code(Bytes.fromHexString(codeScenario.getCode()))
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(recipientAccount)
            .keyPair(senderKeyPair)
            .transactionType(TransactionType.DELEGATE_CODE)
            .nonce(SENDER_NONCE)
            .addCodeDelegation(
                BigInteger.valueOf(LINEA_CHAIN_ID),
                delegationAddress,
                AUTHORITY_NONCE,
                authorityKeyPair)
            .build();

    ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(
                List.of(
                    senderAccount,
                    authorityAccountWithNonEmptyCode,
                    delegationAccount,
                    recipientAccount))
            .transaction(tx)
            .build();
    toyExecutionEnvironmentV2.run();

    TupleAnalysis tupleAnalysis =
        toyExecutionEnvironmentV2
            .getHub()
            .rlpAuth()
            .operations()
            .getFirst()
            .authorizationFragment()
            .tupleAnalysis();
    if (codeScenario == CodeScenario.NON_EMPTY_CODE) {
      assertEquals(
          TupleAnalysis.TUPLE_FAILS_DUE_TO_AUTHORITY_NEITHER_HAVING_EMPTY_CODE_NOR_BEING_DELEGATED,
          tupleAnalysis);
    } else {
      assertEquals(TupleAnalysis.TUPLE_IS_VALID, tupleAnalysis);
    }
  }
}
