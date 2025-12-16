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

package net.consensys.linea.zktracer.instructionprocessing.selfdestructTests;

import static net.consensys.linea.testing.ToyExecutionEnvironmentV2.DEFAULT_COINBASE_ADDRESS;
import static net.consensys.linea.zktracer.instructionprocessing.selfdestructTests.Heir.HEIR_IS_EOA;
import static net.consensys.linea.zktracer.instructionprocessing.selfdestructTests.Heir.basicSelfDestructor;
import static net.consensys.linea.zktracer.types.AddressUtils.getCreateRawAddress;

import java.util.ArrayList;
import java.util.List;
import java.util.Optional;
import java.util.stream.Stream;
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
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

public class SelfdestructCoinbaseTests extends TracerTestBase {

  /**
   * This test aims to test the SELFDESTRUCT of the COINBASE address in various scenarii: - root
   * context is deployment - coinbase / recipient address collision - coinbase is deployed prior to
   * the transaction - the transaction is reverted
   */
  @ParameterizedTest
  @MethodSource("selfDestructCoinbaseInputs")
  void selfdestructCoinbaseTests(
      boolean rootIsDeployment,
      boolean recipientCoinbaseCollision,
      boolean coinBaseDeployed,
      boolean revertingTransaction,
      TestInfo testInfo) {
    final ToyAccount CHECKING_COINBASE =
        ToyAccount.builder()
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    .op(OpCode.COINBASE)
                    .op(OpCode.BALANCE)
                    .op(OpCode.POP)
                    .op(OpCode.COINBASE)
                    .op(OpCode.EXTCODESIZE)
                    .push(0)
                    .push(0)
                    .op(OpCode.COINBASE)
                    .op(OpCode.EXTCODECOPY)
                    .op(OpCode.COINBASE)
                    .op(OpCode.EXTCODEHASH)
                    .compile())
            .build();

    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(senderKeyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(380)).nonce(1).address(senderAddress).build();

    final Address depAddress =
        Address.extract(getCreateRawAddress(senderAddress, senderAccount.getNonce()));

    final ToyAccount selfDestructorCoinbaseAccount =
        basicSelfDestructor(
            HEIR_IS_EOA,
            Optional.of(
                (coinBaseDeployed && rootIsDeployment) ? DEFAULT_COINBASE_ADDRESS : depAddress),
            chainConfig);
    if (revertingTransaction) {
      setRevert(selfDestructorCoinbaseAccount);
    }

    final ToyAccount callingCoinbaseAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(1)
            .address(Address.fromHexString("0x1122334455667788990011223344556677889900"))
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    .push(0)
                    .push(0)
                    .push(0)
                    .push(0)
                    .push(0)
                    .op(OpCode.COINBASE)
                    .push(100000)
                    .op(OpCode.CALL)
                    .compile())
            .build();

    final Transaction selfdestruction =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(
                rootIsDeployment
                    ? null
                    : recipientCoinbaseCollision
                        ? selfDestructorCoinbaseAccount
                        : callingCoinbaseAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .payload(selfDestructorCoinbaseAccount.getCode())
            .build();

    final Transaction checkingCoinbase =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(senderKeyPair)
            .to(CHECKING_COINBASE)
            .gasLimit(10000000L)
            .nonce(senderAccount.getNonce() + 1)
            .build();

    final List<ToyAccount> deployedAccounts = new ArrayList<>();
    deployedAccounts.add(senderAccount);
    if (coinBaseDeployed) {
      deployedAccounts.add(selfDestructorCoinbaseAccount);
    }

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(deployedAccounts)
        .transactions(List.of(selfdestruction, checkingCoinbase))
        .zkTracerValidator(zkTracer -> {})
        .coinbase(depAddress)
        .build()
        .run();
  }

  private static Stream<Arguments> selfDestructCoinbaseInputs() {
    final List<Arguments> arguments = new ArrayList<>();

    for (int rootIsDeployment = 0; rootIsDeployment <= 1; rootIsDeployment++) {
      for (int recipientCoinbaseCollision = 0;
          recipientCoinbaseCollision <= 1;
          recipientCoinbaseCollision++) {
        for (int coinBaseDeployed = 0; coinBaseDeployed <= 1; coinBaseDeployed++) {
          for (int revertingTransaction = 0; revertingTransaction <= 1; revertingTransaction++) {
            arguments.add(
                Arguments.of(
                    rootIsDeployment == 0,
                    recipientCoinbaseCollision == 0,
                    coinBaseDeployed == 0,
                    revertingTransaction == 0));
          }
        }
      }
    }
    return arguments.stream();
  }

  private void setRevert(ToyAccount account) {
    account.setCode(
        Bytes.concatenate(
            account.getCode(), Bytes.fromHexString("0x60006000fd"))); // PUSH1 0 PUSH1 0 REVERT
  }
}
