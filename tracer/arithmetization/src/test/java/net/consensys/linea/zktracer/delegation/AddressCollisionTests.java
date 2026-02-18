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

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class AddressCollisionTests extends TracerTestBase {

  @Disabled
  @ParameterizedTest
  @MethodSource("addressCollisionTestSource")
  void simpleDelegationTest(
      boolean requiresEvm, int sender, int recipient, int coinbase, TestInfo testInfo) {

    final KeyPair add1KeyPair = new SECP256K1().generateKeyPair();
    final Address add1Address =
        Address.extract(Hash.hash(add1KeyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount add1Account =
        ToyAccount.builder().balance(Wei.fromEth(1789)).nonce(0).address(add1Address).build();

    final KeyPair add2KeyPair = new SECP256K1().generateKeyPair();
    final Address add2Address =
        Address.extract(Hash.hash(add2KeyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount add2Account =
        ToyAccount.builder().balance(Wei.fromEth(1789)).nonce(0).address(add2Address).build();

    final KeyPair add3KeyPair = new SECP256K1().generateKeyPair();
    final Address add3Address =
        Address.extract(Hash.hash(add3KeyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount add3Account =
        ToyAccount.builder().balance(Wei.fromEth(1789)).nonce(0).address(add3Address).build();

    // useless stupid code or empty. In the case sender == delegated, we can't set the code of the
    // sender prior to the transaction, but will instead add a self delegation to trigger the evm
    // with an invalid opcode
    final boolean senderIsDelegated = sender == 4;
    final Bytes code =
        requiresEvm && !senderIsDelegated
            ? BytecodeCompiler.newProgram(chainConfig)
                .op(OpCode.ADDRESS)
                .push(12)
                .op(OpCode.SLT)
                .compile()
            : Bytes.EMPTY;

    final KeyPair delegateeKeyPair = new SECP256K1().generateKeyPair();
    final Address delegateeAddress =
        Address.extract(Hash.hash(delegateeKeyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount delegateeAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1789))
            .nonce(0)
            .address(delegateeAddress)
            .code(code)
            .build();

    final ToyAccount senderAccount =
        switch (sender) {
          case 1 -> add1Account;
          case 2 -> add2Account;
          case 3 -> add3Account;
          case 4 -> delegateeAccount;
          default -> throw new IllegalArgumentException("invalid sender");
        };

    final KeyPair senderKeyPair =
        switch (sender) {
          case 1 -> add1KeyPair;
          case 2 -> add2KeyPair;
          case 3 -> add3KeyPair;
          case 4 -> delegateeKeyPair;
          default -> throw new IllegalArgumentException("invalid sender");
        };

    final ToyAccount recipientAccount =
        switch (recipient) {
          case 2 -> add2Account;
          case 3 -> add3Account;
          case 4 -> delegateeAccount;
          default -> throw new IllegalArgumentException("invalid recipient");
        };

    final KeyPair recipientKeyPair =
        switch (recipient) {
          case 2 -> add2KeyPair;
          case 3 -> add3KeyPair;
          case 4 -> delegateeKeyPair;
          default -> throw new IllegalArgumentException("invalid sender");
        };

    final ToyAccount coinbaseAccount =
        switch (coinbase) {
          case 3 -> add3Account;
          case 4 -> delegateeAccount;
          default -> throw new IllegalArgumentException("invalid Coinbase");
        };

    final ToyTransaction.ToyTransactionBuilder tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(recipientAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .addCodeDelegation(
                chainConfig.id, delegateeAddress, recipient == sender ? 1 : 0, recipientKeyPair);

    if (senderIsDelegated && requiresEvm) {
      tx.addCodeDelegation(
          chainConfig.id, delegateeAddress, recipient == sender ? 2 : 1, delegateeKeyPair);
    }

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(add1Account, add2Account, add3Account, delegateeAccount))
        .transaction(tx.build())
        .zkTracerValidator(zkTracer -> {})
        .coinbase(coinbaseAccount.getAddress())
        .build()
        .run();
  }

  private static Stream<Arguments> addressCollisionTestSource() {
    final List<Arguments> arguments = new ArrayList<>();
    for (int requireEvm = 0; requireEvm <= 1; requireEvm++) {
      for (int sender = 1; sender <= 4; sender++) {
        for (int recipient = 2; recipient <= 4; recipient++) {
          for (int coinbase = 3; coinbase <= 4; coinbase++) {
            arguments.add(Arguments.of(requireEvm == 1, sender, recipient, coinbase));
          }
        }
      }
    }
    return arguments.stream();
  }
}
