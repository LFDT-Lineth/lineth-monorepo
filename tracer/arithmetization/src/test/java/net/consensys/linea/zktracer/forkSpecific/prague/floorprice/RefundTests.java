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

package net.consensys.linea.zktracer.forkSpecific.prague.floorprice;

import static net.consensys.linea.testing.BytecodeRunner.MAX_GAS_LIMIT;
import static net.consensys.linea.zktracer.Fork.isPostPrague;

import com.google.common.base.Preconditions;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import net.consensys.linea.zktracer.module.txndata.transactions.UserTransaction;
import net.consensys.linea.zktracer.module.txndata.transactions.UserTransaction.DominantCost;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.AddressUtils;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class RefundTests extends TracerTestBase {

  final Bytes runtimeCode =
      BytecodeCompiler.newProgram(chainConfig)
          .push(0) // value
          .push(1) // key
          .op(OpCode.SSTORE) // reset storage slot 1 to 0
          .op(OpCode.JUMPDEST, 10)
          .compile(); // We assume this is at most 32 bytes

  final Bytes initCode =
      BytecodeCompiler.newProgram(chainConfig)
          .push(0xff) // value
          .push(1) // key
          .op(OpCode.SSTORE) // the contract is initialised with a storage slot 1 set to 0xff
          .push(runtimeCode) // value
          .push(0) // offset
          .op(OpCode.MSTORE)
          .push(runtimeCode.size()) // size
          .push(32 - runtimeCode.size()) // offset
          .op(OpCode.RETURN)
          .compile();

  @ParameterizedTest
  @MethodSource("refundTestSource")
  void refundTest(Bytes callData, DominantCost dominantCostPrediction, TestInfo testInfo) {
    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));

    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(1)).nonce(0).address(senderAddress).build();

    // Deploy contract with slot 1 set to 1
    final Transaction deploymentTransaction =
        ToyTransaction.builder()
            .payload(initCode)
            .gasLimit(MAX_GAS_LIMIT)
            .sender(senderAccount)
            .value(Wei.of(272)) // 256 + 16, easier for debugging
            .keyPair(keyPair)
            .gasPrice(Wei.of(8))
            .build();

    // Compute the address of the deployed contract
    final Address deploymentAddress = AddressUtils.effectiveToAddress(deploymentTransaction);

    // Call the deployment contract which resets slot 1 to 0
    final Transaction callTransaction =
        ToyTransaction.builder()
            .payload(callData)
            .gasLimit(MAX_GAS_LIMIT)
            .sender(senderAccount)
            .toAddress(deploymentAddress)
            .value(Wei.ZERO)
            .keyPair(keyPair)
            .gasPrice(Wei.of(8))
            .nonce(senderAccount.getNonce() + 1)
            .build();

    ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount))
            .transactions(List.of(deploymentTransaction, callTransaction))
            .transactionProcessingResultValidator(
                TransactionProcessingResultValidator.EMPTY_VALIDATOR)
            .build();
    toyExecutionEnvironmentV2.run();

    // Test blocks contain 5 transactions: 2 system transactions, 2 user transaction (the deployment
    // transaction and the one we
    // created) and 1 noop transaction.
    if (isPostPrague(fork)) {
      UserTransaction userTransaction =
          (UserTransaction) toyExecutionEnvironmentV2.getHub().txnData().operations().get(3);
      Preconditions.checkArgument(userTransaction.getDominantCost() == dominantCostPrediction);
    }
  }

  static Stream<Arguments> refundTestSource() {
    /*
    SSTORE cost: 2_100 + 2_900 = 5_000
    execution cost: 21_000 + 3 + 3 + 5_000 + 10*1 + 16*cds = 26_016 + 16*cds
    refund: 4800
    refund bound: execution cost / 5 ≥ 5_203 + 5*cds > 4_800
    we get full refund

    execution cost after refunds: 26_016 + 16*cds - 4_800 = 21_216 + 16*cds

    floor price: 21_000 + 40*cds

    threshold: 21_000 + 40*cds > 21_216 + 16*cds <=> 24*cds > 216 <=> cds > 9
     */
    List<Arguments> arguments = new ArrayList<>();
    arguments.add(
        Arguments.of(Bytes.fromHexString("11".repeat(9)), DominantCost.EXECUTION_COST_DOMINATES));
    arguments.add(
        Arguments.of(Bytes.fromHexString("11".repeat(10)), DominantCost.FLOOR_COST_DOMINATES));
    return arguments.stream();
  }
}
