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

package net.consensys.linea.zktracer.delegation.multiDelegation;

import static net.consensys.linea.zktracer.Trace.LINEA_CHAIN_ID;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.math.BigInteger;
import java.util.List;
import lombok.RequiredArgsConstructor;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.container.stacked.ModuleOperationStackedList;
import net.consensys.linea.zktracer.module.hub.section.TupleAnalysis;
import net.consensys.linea.zktracer.module.rlpAuth.RlpAuthOperation;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

// https://github.com/Consensys/linea-monorepo/issues/2455

@ExtendWith(UnitTestWatcher.class)
public class TrivialMultiDelegationTest extends TracerTestBase {

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

  @Test
  void trivialMultiDelegationMonoTransactionTest(TestInfo testInfo) {
    NonceProvider authorityNonceProvider = new NonceProvider(AUTHORITY_NONCE);

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(recipientAccount)
            .keyPair(senderKeyPair)
            .transactionType(TransactionType.DELEGATE_CODE)
            .nonce(SENDER_NONCE)
            .gasLimit(96000L)
            .addCodeDelegation(
                BigInteger.valueOf(LINEA_CHAIN_ID),
                delegationAddress,
                authorityNonceProvider.getCurrentNonce(),
                authorityKeyPair)
            .addCodeDelegation(
                BigInteger.valueOf(LINEA_CHAIN_ID),
                delegationAddress,
                authorityNonceProvider.getCurrentNonce(),
                authorityKeyPair)
            .addCodeDelegation(
                BigInteger.valueOf(LINEA_CHAIN_ID),
                delegationAddress,
                authorityNonceProvider.getCurrentNonce(),
                authorityKeyPair)
            .build();

    ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, authorityAccount, delegationAccount, recipientAccount))
            .transaction(tx)
            .build();
    toyExecutionEnvironmentV2.run();

    ModuleOperationStackedList<RlpAuthOperation> operations =
        toyExecutionEnvironmentV2.getHub().rlpAuth().operations();
    assertEquals(3, operations.size());
    for (RlpAuthOperation operation : operations.getAll()) {
      assertEquals(TupleAnalysis.TUPLE_IS_VALID, operation.authorizationFragment().tupleAnalysis());
    }
  }

  // Utils
  @RequiredArgsConstructor
  private static class NonceProvider {
    private final long nonce;
    private int nonceOffset = -1;

    public long getCurrentNonce() {
      nonceOffset++;
      return nonce + nonceOffset;
    }

    public long getWrongNonce() {
      return nonce + 1234;
    }
  }
}
