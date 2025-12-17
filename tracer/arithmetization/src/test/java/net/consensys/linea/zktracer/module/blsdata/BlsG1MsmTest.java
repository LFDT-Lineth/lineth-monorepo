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

package net.consensys.linea.zktracer.module.blsdata;

import static net.consensys.linea.zktracer.Fork.isPostPrague;
import static net.consensys.linea.zktracer.module.blsdata.BlsTestUtils.SMALL_POINTS;
import static net.consensys.linea.zktracer.module.blsdata.BlsTestUtils.VALID_G1_POINT;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.stream.IntStream;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.module.tables.BlsRt;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class BlsG1MsmTest extends TracerTestBase {

  /**
   * The following test tests the BLS_DATA module's ability to recognize malformed points and, for
   * well-formed points, to offload curve membership and subgroup membership tests to gnark, in a
   * way which is in accordance with the EVM and the return bit of the underlying <b>CALL</b>.
   * Specifically, all the possible values of the reference table and beyond, used for computing the
   * cost, are tested.
   */
  @ParameterizedTest
  @MethodSource({"blsG1MsmSource", "blsG1MsmFullTableSource"})
  void testBlsG1MsmTest(int n, List<String> smallPoints, TestInfo testInfo) {
    final Bytes input =
        IntStream.range(0, smallPoints.size())
            .mapToObj(
                i ->
                    Bytes.concatenate(
                        Bytes.fromHexString(smallPoints.get(i)), Bytes32.leftPad(Bytes.of(i + 1))))
            .reduce(Bytes.EMPTY, Bytes::concatenate);

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    final Address codeOwnerAddress = Address.fromHexString("0xC0DE");
    final ToyAccount codeOwnerAccount =
        ToyAccount.builder()
            .balance(Wei.of(0))
            .nonce(1)
            .address(codeOwnerAddress)
            .code(input)
            .build();

    // First place the parameters in memory
    // Copy to targetOffset the code of codeOwnerAccount
    program
        .push(codeOwnerAddress)
        .op(OpCode.EXTCODESIZE) // size
        .push(0) // offset
        .push(0) // targetOffset
        .push(codeOwnerAddress) // address
        .op(OpCode.EXTCODECOPY);

    // Do the call
    program
        .push(128) // retSize
        .push(input.size()) // retOffset
        .push(input.size()) // argSize
        .push(0) // argOffset
        .push(Address.BLS12_G1MULTIEXP) // address
        .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
        .op(OpCode.STATICCALL);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(List.of(codeOwnerAccount), chainConfig, testInfo);

    if (isPostPrague(fork)) {
      final boolean failureIsExpected =
          smallPoints.stream().anyMatch(p -> !p.equals(VALID_G1_POINT));
      final BlsData blsdata = (BlsData) bytecodeRunner.getHub().blsData();
      assertEquals(failureIsExpected, blsdata.blsDataOperation().malformedDataExternal());
      assertEquals(failureIsExpected, !blsdata.blsDataOperation().successBit());
    }
  }

  private static Stream<Arguments> blsG1MsmSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (String s1 : SMALL_POINTS) {
      arguments.add(Arguments.of(1, List.of(s1)));
      for (String s2 : SMALL_POINTS) {
        arguments.add(Arguments.of(2, List.of(s1, s2)));
        for (String s3 : SMALL_POINTS) {
          arguments.add(Arguments.of(3, List.of(s1, s2, s3)));
        }
      }
    }
    return arguments.stream();
  }

  private static Stream<Arguments> blsG1MsmFullTableSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (int n = 0; n < BlsRt.G1_MSM_DISCOUNTS.size() + 10; n++) {
      arguments.add(Arguments.of(n + 1, Collections.nCopies(n + 1, VALID_G1_POINT)));
    }
    return arguments.stream();
  }
}
