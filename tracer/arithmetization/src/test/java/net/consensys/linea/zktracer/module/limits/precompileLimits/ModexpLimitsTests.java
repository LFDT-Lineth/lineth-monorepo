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

package net.consensys.linea.zktracer.module.limits.precompileLimits;

import static net.consensys.linea.testing.BytecodeRunner.MAX_GAS_LIMIT;
import static net.consensys.linea.zktracer.Fork.forkPredatesOsaka;
import static net.consensys.linea.zktracer.module.ModuleName.PRECOMPILE_MODEXP_EFFECTIVE_CALLS;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.*;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.module.ModuleName;
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

public class ModexpLimitsTests extends TracerTestBase {

  @ParameterizedTest
  @MethodSource("modexpInput")
  void modexpCall(int bbs, int ebs, int mbs, TestInfo testInfo) {
    // sender account
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(senderKeyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(123)).nonce(12).address(senderAddress).build();

    final int gasArgument = 10_000_000;

    // receiver account: calls MODEXP
    final ToyAccount callPRC =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .address(Address.wrap(Bytes.repeat((byte) 1, Address.SIZE)))
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    // populate memory with BBS
                    .push(bbs) // value
                    .push(BBS_MIN_OFFSET) //  offset
                    .op(OpCode.MSTORE)
                    // populate memory with EBS
                    .push(ebs) // value
                    .push(EBS_MIN_OFFSET) //  offset
                    .op(OpCode.MSTORE)
                    // populate memory with MBS
                    .push(mbs) // value
                    .push(MBS_MIN_OFFSET) //  offset
                    .op(OpCode.MSTORE)
                    // call modexp
                    .push(2) // return size
                    .push(0) // return offset
                    .push(2000) // cds
                    .push(0) // offset
                    .push(0) // value
                    .push(Address.MODEXP) // address
                    .push(gasArgument) // gas
                    .op(OpCode.CALL)
                    .compile())
            .build();

    // Note: at the point in time when the call takes place memory is
    // <bbs> <ebs> <mbs>
    // and nothing beyond

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(callPRC)
            .keyPair(senderKeyPair)
            .gasLimit(MAX_GAS_LIMIT)
            .value(Wei.of(10000000))
            .build();

    final ToyExecutionEnvironmentV2 toyWorld =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, callPRC))
            .transaction(tx)
            .zkTracerValidator(zkTracer -> {})
            .build();

    toyWorld.runForCounting();

    final Map<String, Integer> lineCountMap = toyWorld.getZkCounter().getModulesLineCount();

    // check MODEXP limits:
    final int legalModexpComponentByteSize = 1024;
    final int numberOfEffectiveModexpCallsForInvalidInputs =
        forkPredatesOsaka(fork) ? Integer.MAX_VALUE : 0;
    final int actualCount = lineCountMap.get(PRECOMPILE_MODEXP_EFFECTIVE_CALLS.toString());
    final boolean validByteSizes =
        (bbs <= legalModexpComponentByteSize
            && ebs <= legalModexpComponentByteSize
            && mbs <= legalModexpComponentByteSize);
    long roughOsakaModexpCost;
    {
      final long maxMbsBbs = Math.max(mbs, bbs);
      final long maxOver8 = Math.ceilDiv(maxMbsBbs, 8);
      final long multiplier = forkPredatesOsaka(fork) ? 8 : 16;
      final long leadLogCost = Math.max(1, multiplier * (ebs - 32));
      roughOsakaModexpCost = 2 * maxOver8 * maxOver8 * leadLogCost;
    }
    final boolean sufficientGasForOsaka = gasArgument >= roughOsakaModexpCost;
    final boolean successExpected =
        (forkPredatesOsaka(fork)) ? validByteSizes : validByteSizes && sufficientGasForOsaka;
    final int expectedCount = successExpected ? 1 : numberOfEffectiveModexpCallsForInvalidInputs;
    assertEquals(
        expectedCount,
        actualCount,
        PRECOMPILE_MODEXP_EFFECTIVE_CALLS + " discrepancy, actual = " + actualCount);

    final int maxByteWidth = Math.max(Math.max(bbs, ebs), mbs);
    assertEquals(
        (maxByteWidth > 32 && maxByteWidth <= legalModexpComponentByteSize) ? 1 : 0,
        lineCountMap.get(ModuleName.PRECOMPILE_LARGE_MODEXP_EFFECTIVE_CALLS.toString()),
        ModuleName.PRECOMPILE_LARGE_MODEXP_EFFECTIVE_CALLS + " discrepancy");
  }

  private static Stream<Arguments> modexpInput() {
    final List<Arguments> arguments = new ArrayList<>();
    for (int bbs : BYTE_SIZE_TO_TEST) {
      for (int ebs : BYTE_SIZE_TO_TEST) {
        for (int mbs : BYTE_SIZE_TO_TEST) {
          arguments.add(Arguments.of(bbs, ebs, mbs));
        }
      }
    }

    return arguments.stream();
  }

  private static List<Integer> BYTE_SIZE_TO_TEST = List.of(0, 18, 32, 216, 318, 512, 513);
}
