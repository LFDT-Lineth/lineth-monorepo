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
package net.consensys.linea.zktracer.precompiles.modexp;

import static net.consensys.linea.zktracer.types.Conversions.bytesToBoolean;
import static org.junit.jupiter.api.Assertions.assertEquals;

import com.google.common.base.Preconditions;
import java.math.BigInteger;
import java.time.LocalDate;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.Random;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.commons.math3.util.Pair;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

public class ModexpEIP7883Tests extends TracerTestBase {

  // See https://github.com/Consensys/linea-tracer/issues/2496
  static final List<Pair<Integer, Integer>> bbsMbsPairOfValues =
      List.of(Pair.create(0, 0), Pair.create(0, 3), Pair.create(21, 23), Pair.create(56, 55));

  static final List<Integer> ebsValues = List.of(0, 1, 16, 27, 32, 39, 173);

  // Pre-computed leading words for the exponent for each ebs value
  static final Map<Integer, List<String>> ebsToExponentLeadingWords =
      ebsValues.stream()
          .collect(
              Collectors.toMap(
                  ebsItem -> ebsItem,
                  ebsItem -> {
                    List<String> leadingWords = new ArrayList<>();
                    final int minEbs32 = Math.min(ebsItem, 32);
                    BigInteger leadingWord =
                        minEbs32 > 0 ? new BigInteger("ff".repeat(minEbs32), 16) : BigInteger.ZERO;
                    for (int z = 0; z <= 8 * minEbs32; z++) {
                      final String leadingWordAsHex =
                          leadingWord.signum() != 0 ? leadingWord.toString(16) : "";
                      final String leftPaddedLeadingWordAsHex =
                          "0".repeat(Math.max(0, 2 * minEbs32 - leadingWordAsHex.length()))
                              + leadingWordAsHex;
                      Preconditions.checkArgument(
                          leftPaddedLeadingWordAsHex.length() == 2 * minEbs32);
                      leadingWords.add(leftPaddedLeadingWordAsHex);
                      leadingWord = leadingWord.shiftRight(1);
                    }
                    return leadingWords;
                  }));

  // Support method to compute cds given bbs, ebs, mbs
  static List<Integer> cdsValues(Integer bbs, Integer ebs, Integer mbs) {
    List<Integer> cdsValues = new ArrayList<>();
    for (Integer extra : ebs != 0 ? List.of(ebs / 2, ebs, ebs + mbs) : List.of(0, mbs)) {
      cdsValues.add(96 + bbs + extra);
    }
    return cdsValues;
  }

  @ParameterizedTest
  @MethodSource("modexpEIP7883TestSource")
  void modexpEIP7883Test(
      int bbs, int ebs, int mbs, int cds, String exponentLeadingWord, TestInfo testInfo) {
    modexpEIP7883TestBody(bbs, ebs, mbs, cds, exponentLeadingWord, testInfo);
  }

  @Tag("nightly")
  @ParameterizedTest
  @MethodSource("modexpEIP7883TestSourceNightly")
  void modexpEIP7883TestNightly(
      int bbs, int ebs, int mbs, int cds, String exponentLeadingWord, TestInfo testInfo) {
    modexpEIP7883TestBody(bbs, ebs, mbs, cds, exponentLeadingWord, testInfo);
  }

  private void modexpEIP7883TestBody(
      int bbs, int ebs, int mbs, int cds, String exponentLeadingWord, TestInfo testInfo) {
    final String bbsHex = String.format("%064x", bbs);
    final String ebsHex = String.format("%064x", ebs);
    final String mbsHex = String.format("%064x", mbs);

    final Bytes input =
        Bytes.fromHexString(
            bbsHex
                + ebsHex
                + mbsHex
                + "00".repeat(bbs)
                + exponentLeadingWord); // Then the right cds implicitly adds the right padding

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    final Address callDataAsBytecodeAddress = Address.fromHexString("0xC0DE");
    final ToyAccount codeOwnerAccount =
        ToyAccount.builder()
            .balance(Wei.of(0))
            .nonce(1)
            .address(callDataAsBytecodeAddress)
            .code(input)
            .build();

    // First place the parameters in memory
    // Copy to targetOffset the code of codeOwnerAccount
    program
        .push(callDataAsBytecodeAddress)
        .op(OpCode.EXTCODESIZE) // size
        .push(0) // offset
        .push(0) // targetOffset
        .push(callDataAsBytecodeAddress) // address
        .op(OpCode.EXTCODECOPY);

    // Do the call
    program
        .push(mbs) // retSize
        .push(cds) // retOffset
        .push(cds) // argSize
        .push(0) // argOffset
        .push(Address.MODEXP) // address
        .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
        .op(OpCode.STATICCALL)
        .op(OpCode.RETURNDATASIZE)
        .op(OpCode.JUMPDEST, 32); // TODO: temporary workaround for go-corset issue

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(List.of(codeOwnerAccount), chainConfig, testInfo);

    final Bytes returnDataSize = bytecodeRunner.getHub().currentFrame().frame().getStackItem(0);
    final Bytes callSuccess = bytecodeRunner.getHub().currentFrame().frame().getStackItem(1);
    if (bytesToBoolean(callSuccess)) {
      assertEquals(mbs, returnDataSize.toInt());
    }
  }

  static Stream<Arguments> modexpEIP7883TestSource() {
    List<Arguments> arguments = new ArrayList<>(modexpEIP7883TestSourceNightly().toList());
    Collections.shuffle(arguments, new Random(LocalDate.now().toEpochDay()));
    return arguments.stream().limit(arguments.size() / 40); // Execute 2.5 % of the tests
  }

  static Stream<Arguments> modexpEIP7883TestSourceNightly() {
    List<Arguments> arguments = new ArrayList<>();
    for (Pair<Integer, Integer> bbsMbs : bbsMbsPairOfValues) {
      Integer bbs = bbsMbs.getFirst();
      Integer mbs = bbsMbs.getSecond();
      for (Integer ebs : ebsValues) {
        List<Integer> cdsValues = cdsValues(bbs, ebs, mbs);
        List<String> exponentLeadingWordsForEbs = ebsToExponentLeadingWords.get(ebs);
        for (Integer cds : cdsValues) {
          for (String exponentLeadingWordForEbs : exponentLeadingWordsForEbs) {
            arguments.add(Arguments.of(bbs, ebs, mbs, cds, exponentLeadingWordForEbs));
          }
        }
      }
    }
    return arguments.stream();
  }
}
