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

package net.consensys.linea.zktracer.module.oob;

import static net.consensys.linea.zktracer.module.hub.signals.TracedException.JUMP_FAULT;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Random;
import java.util.stream.Collectors;
import java.util.stream.IntStream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class OobJumpAndJumpiTest extends TracerTestBase {

  public static final BigInteger TWO_POW_128_MINUS_ONE =
      BigInteger.ONE.shiftLeft(128).subtract(BigInteger.ONE);

  @Test
  void testJumpSequenceSuccessTrivial(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendJump(EWord.of(35), program);
    appendStop(program);

    appendJumpDest(program); // PC = 35
    appendJump(EWord.of(71), program);
    appendStop(program);

    appendJumpDest(program); // PC = 71
    appendJump(EWord.of(107), program);
    appendStop(program);

    appendJumpDest(program); // PC = 107
    appendStop(program);

    // codesize = 109

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  @Test
  void testJumpSequenceSuccessBackAndForth(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendJump(EWord.of(71), program);
    appendStop(program);

    appendJumpDest(program); // PC = 35
    appendJump(EWord.of(107), program);
    appendStop(program);

    appendJumpDest(program); // PC = 71
    appendJump(EWord.of(35), program);
    appendStop(program);

    appendJumpDest(program); // PC = 107
    appendStop(program);

    // codesize = 109

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  @Test
  void testJumpSequenceFailingNoJumpdestTrivial(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendJump(EWord.of(35), program);
    appendStop(program);

    appendJumpDest(program); // PC = 35
    appendJump(EWord.of(71), program);
    appendStop(program);

    appendJumpDest(program); // PC = 71
    appendJump(EWord.of(106), program); // It fails because 106 is not a JUMPDEST
    appendStop(program);

    appendJumpDest(program); // PC = 107

    // codesize = 108

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.jumpFault(hub.pch().exceptions()));
    assertEquals(
        JUMP_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testJumpSequenceFailingOobTrivial(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendJump(EWord.of(35), program);
    appendStop(program);

    appendJumpDest(program); // PC = 35
    appendJump(EWord.of(71), program);
    appendStop(program);

    appendJumpDest(program); // PC = 71
    appendJump(EWord.of(108), program); // It fails because pc_new = 108 >= codesize = 108
    appendStop(program);

    appendJumpDest(program); // PC = 107

    // codesize = 108

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.jumpFault(hub.pch().exceptions()));
    assertEquals(
        JUMP_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testJumpSequenceSuccessRandom(TestInfo testInfo) {
    final int N_JUMPS = 200;
    final int MAX_JUMPDESTINATION = 256;
    final int SPREADING_FACTOR = 256;

    // Generate N_JUMPS random jump destinations
    List<Integer> jumpDestinations =
        generateJumpDestinations(N_JUMPS, MAX_JUMPDESTINATION, SPREADING_FACTOR);

    // Compute the PC of each jump destination
    Map<Integer, EWord> jumpDestinationsToPC = computeJumpDestinationsToPCForJUMP(jumpDestinations);

    // Init a byteCode with all STOPs (0x00)
    int byteCodeNumberOfElements = jumpDestinations.get(jumpDestinations.size() - 1) + 1;
    List<Bytes> byteCode = initByteCode(byteCodeNumberOfElements);

    // First jump
    byteCode.set(0, Bytes.of(OpCode.PUSH32.byteValue()));
    byteCode.set(1, jumpDestinationsToPC.get(jumpDestinations.get(0)));
    byteCode.set(2, Bytes.of(OpCode.JUMP.byteValue()));

    // Jumps in the middle
    for (int i = 0; i < jumpDestinations.size() - 1; i++) {
      int jumpSource = jumpDestinations.get(i);
      byteCode.set(jumpSource, Bytes.of(OpCode.JUMPDEST.byteValue()));
      byteCode.set(jumpSource + 1, Bytes.of(OpCode.PUSH32.byteValue()));
      byteCode.set(
          jumpSource + 2,
          jumpDestinationsToPC.get(
              jumpDestinations.get(i + 1))); // Jump to the next jump destination
      byteCode.set(jumpSource + 3, Bytes.of(OpCode.JUMP.byteValue()));
    }

    // Last jump destination
    byteCode.set(
        jumpDestinations.get(jumpDestinations.size() - 1), Bytes.of(OpCode.JUMPDEST.byteValue()));

    // Run the generated bytecode
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(Bytes.concatenate(byteCode));
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();
    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  @Test
  void testJumpSequenceSuccessRandomBackAndForth(TestInfo testInfo) {
    final int N_JUMPS = 200;
    final int MAX_JUMPDESTINATION = 256;
    final int SPREADING_FACTOR = 256;

    // Generate N_JUMPS random jump destinations
    List<Integer> jumpDestinations =
        generateJumpDestinations(N_JUMPS, MAX_JUMPDESTINATION, SPREADING_FACTOR);

    // Compute the PC of each jump destination
    Map<Integer, EWord> jumpDestinationsToPC = computeJumpDestinationsToPCForJUMP(jumpDestinations);

    // Init a byteCode with all STOPs (0x00)
    int byteCodeNumberOfElements = jumpDestinations.get(jumpDestinations.size() - 1) + 1;
    List<Bytes> byteCode = initByteCode(byteCodeNumberOfElements);

    // Define a permutation of the order of the jump destinations
    List<Integer> permutation = generatePermutation(jumpDestinations.size());

    // First jump
    byteCode.set(0, Bytes.of(OpCode.PUSH32.byteValue()));
    byteCode.set(1, jumpDestinationsToPC.get(jumpDestinations.get(permutation.get(0))));
    byteCode.set(2, Bytes.of(OpCode.JUMP.byteValue()));

    // Jumps in the middle
    for (int i = 0; i < jumpDestinations.size() - 1; i++) {
      int jumpSource = jumpDestinations.get(permutation.get(i));
      byteCode.set(jumpSource, Bytes.of(OpCode.JUMPDEST.byteValue()));
      byteCode.set(jumpSource + 1, Bytes.of(OpCode.PUSH32.byteValue()));
      byteCode.set(
          jumpSource + 2,
          jumpDestinationsToPC.get(
              jumpDestinations.get(
                  permutation.get(i + 1)))); // Jump to the next jump destination wrt permutation
      byteCode.set(jumpSource + 3, Bytes.of(OpCode.JUMP.byteValue()));
    }

    // Last jump destination
    byteCode.set(
        jumpDestinations.get(jumpDestinations.size() - 1), Bytes.of(OpCode.JUMPDEST.byteValue()));

    // Run the generated bytecode
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(Bytes.concatenate(byteCode));
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();
    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  @Test
  void testJumpiSequenceSuccessTrivial(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendJumpi(EWord.of(68), EWord.of(1), program);
    appendStop(program);

    appendJumpDest(program); // PC = 68
    appendJumpi(EWord.of(137), EWord.of(1), program);
    appendStop(program);

    appendJumpDest(program); // PC = 137
    appendJumpi(EWord.of(206), EWord.of(1), program);
    appendStop(program);

    appendJumpDest(program); // PC = 206
    appendStop(program);

    // codesize = 208

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  @Test
  void testJumpiSequenceSuccessBackAndForth(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendJumpi(EWord.of(137), EWord.of(1), program);
    appendStop(program);

    appendJumpDest(program); // PC = 68
    appendJumpi(EWord.of(206), EWord.of(1), program);
    appendStop(program);

    appendJumpDest(program); // PC = 137
    appendJumpi(EWord.of(68), EWord.of(1), program);
    appendStop(program);

    appendJumpDest(program); // PC = 206
    appendStop(program);

    // codesize = 208

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  @Test
  void testJumpiSequenceFailingNoJumpdestTrivial(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendJumpi(EWord.of(68), EWord.of(1), program);
    appendStop(program);

    appendJumpDest(program); // PC = 68
    appendJumpi(EWord.of(137), EWord.of(1), program);
    appendStop(program);

    appendJumpDest(program); // PC = 137
    appendJumpi(EWord.of(205), EWord.of(1), program); // It fails because 205 is not a JUMPDEST
    appendStop(program);

    appendJumpDest(program); // PC = 206

    // codesize = 207

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.jumpFault(hub.pch().exceptions()));
    assertEquals(
        JUMP_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testJumpiSequenceFailingOobTrivial(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendJumpi(EWord.of(68), EWord.of(1), program);
    appendStop(program);

    appendJumpDest(program); // PC = 68
    appendJumpi(EWord.of(137), EWord.of(1), program);
    appendStop(program);

    appendJumpDest(program); // PC = 137
    appendJumpi(
        EWord.of(207), EWord.of(1), program); // It fails because pc_new = 207 >= codesize = 207
    appendStop(program);

    appendJumpDest(program); // PC = 206

    // codesize = 207

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.jumpFault(hub.pch().exceptions()));
    assertEquals(
        JUMP_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testNoJumpi(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    appendJumpi(EWord.of(68), EWord.of(0), program); // jumpCondition is 0, that means no JUMPI
    appendStop(program);

    appendJumpDest(program); // PC = 68

    // codesize = 69

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  @Test
  void testJumpiHiNonZero(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    EWord jumpCondition = EWord.of(TWO_POW_128_MINUS_ONE, BigInteger.ZERO);
    appendJumpi(EWord.of(68), jumpCondition, program);
    appendStop(program);

    appendJumpDest(program); // PC = 68

    // codesize = 69

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  @Test
  void testJumpiLoNonZero(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    EWord jumpCondition = EWord.of(BigInteger.valueOf(0), TWO_POW_128_MINUS_ONE);
    appendJumpi(EWord.of(68), jumpCondition, program);
    appendStop(program);

    appendJumpDest(program); // PC = 68

    // codesize = 69

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  @Test
  void testJumpiHiLoNonZero(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    EWord jumpCondition = EWord.of(TWO_POW_128_MINUS_ONE, TWO_POW_128_MINUS_ONE);
    appendJumpi(EWord.of(68), jumpCondition, program);
    appendStop(program);

    appendJumpDest(program); // PC = 68

    // codesize = 69

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  @Test
  void testJumpiSequenceSuccessRandom(TestInfo testInfo) {
    final int N_JUMPIS = 200;
    final int MAX_JUMPDESTINATION = 256;
    final int SPREADING_FACTOR = 256;

    // Generate N_JUMPIS random jump destinations
    List<Integer> jumpDestinations =
        generateJumpDestinations(N_JUMPIS, MAX_JUMPDESTINATION, SPREADING_FACTOR);

    // Compute the PC of each jump destination
    Map<Integer, EWord> jumpDestinationsToPC =
        computeJumpDestinationsToPCForJUMPI(jumpDestinations);

    // Init a byteCode with all STOPs (0x00)
    int byteCodeNumberOfElements = jumpDestinations.get(jumpDestinations.size() - 1) + 1;
    List<Bytes> byteCode = initByteCode(byteCodeNumberOfElements);

    // First jumpi
    byteCode.set(0, Bytes.of(OpCode.PUSH32.byteValue()));
    byteCode.set(1, EWord.of(1));
    byteCode.set(2, Bytes.of(OpCode.PUSH32.byteValue()));
    byteCode.set(3, jumpDestinationsToPC.get(jumpDestinations.get(0)));
    byteCode.set(4, Bytes.of(OpCode.JUMPI.byteValue()));

    // Jumpis in the middle
    for (int i = 0; i < jumpDestinations.size() - 1; i++) {
      int jumpiSource = jumpDestinations.get(i);
      byteCode.set(jumpiSource, Bytes.of(OpCode.JUMPDEST.byteValue()));
      byteCode.set(jumpiSource + 1, Bytes.of(OpCode.PUSH32.byteValue()));
      byteCode.set(jumpiSource + 2, EWord.of(1));
      byteCode.set(jumpiSource + 3, Bytes.of(OpCode.PUSH32.byteValue()));
      byteCode.set(
          jumpiSource + 4,
          jumpDestinationsToPC.get(
              jumpDestinations.get(i + 1))); // Jumpi to the next jump destination
      byteCode.set(jumpiSource + 5, Bytes.of(OpCode.JUMPI.byteValue()));
    }

    // Last jump destination
    byteCode.set(
        jumpDestinations.get(jumpDestinations.size() - 1), Bytes.of(OpCode.JUMPDEST.byteValue()));

    // Run the generated bytecode
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(Bytes.concatenate(byteCode));
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();
    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  @Test
  void testJumpiSequenceSuccessRandomBackAndForth(TestInfo testInfo) {
    final int N_JUMPIS = 200;
    final int MAX_JUMPDESTINATION = 256;
    final int SPREADING_FACTOR = 256;

    // Generate N_JUMPIS random jump destinations
    List<Integer> jumpDestinations =
        generateJumpDestinations(N_JUMPIS, MAX_JUMPDESTINATION, SPREADING_FACTOR);

    // Compute the PC of each jump destination
    Map<Integer, EWord> jumpDestinationsToPC =
        computeJumpDestinationsToPCForJUMPI(jumpDestinations);

    // Init a byteCode with all STOPs (0x00)
    int byteCodeNumberOfElements = jumpDestinations.get(jumpDestinations.size() - 1) + 1;
    List<Bytes> byteCode = initByteCode(byteCodeNumberOfElements);

    // Define a permutation of the order of the jump destinations
    List<Integer> permutation = generatePermutation(jumpDestinations.size());

    // First jumpi
    byteCode.set(0, Bytes.of(OpCode.PUSH32.byteValue()));
    byteCode.set(1, EWord.of(1));
    byteCode.set(2, Bytes.of(OpCode.PUSH32.byteValue()));
    byteCode.set(3, jumpDestinationsToPC.get(jumpDestinations.get(permutation.get(0))));
    byteCode.set(4, Bytes.of(OpCode.JUMPI.byteValue()));

    // Jumpis in the middle
    for (int i = 0; i < jumpDestinations.size() - 1; i++) {
      int jumpiSource = jumpDestinations.get(permutation.get(i));
      byteCode.set(jumpiSource, Bytes.of(OpCode.JUMPDEST.byteValue()));
      byteCode.set(jumpiSource + 1, Bytes.of(OpCode.PUSH32.byteValue()));
      byteCode.set(jumpiSource + 2, EWord.of(1));
      byteCode.set(jumpiSource + 3, Bytes.of(OpCode.PUSH32.byteValue()));
      byteCode.set(
          jumpiSource + 4,
          jumpDestinationsToPC.get(
              jumpDestinations.get(
                  permutation.get(i + 1)))); // Jump to the next jump destination wrt permutation
      byteCode.set(jumpiSource + 5, Bytes.of(OpCode.JUMPI.byteValue()));
    }

    // Last jump destination
    byteCode.set(
        jumpDestinations.get(jumpDestinations.size() - 1), Bytes.of(OpCode.JUMPDEST.byteValue()));

    // Run the generated bytecode
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(Bytes.concatenate(byteCode));
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();
    assertFalse(Exceptions.jumpFault(hub.pch().exceptions()));
  }

  // Support methods
  private Random random = new Random(1);

  private List<Integer> generateJumpDestinations(
      int N_JUMPS, int MAX_JUMPDESTINATION, int SPREADING_FACTOR) {
    return random
        .ints(1, MAX_JUMPDESTINATION)
        .distinct()
        .limit(N_JUMPS)
        .sorted()
        .map(x -> x * SPREADING_FACTOR)
        .boxed()
        .toList();
  }

  private List<Integer> generatePermutation(int jumpDestinationsSize) {
    List<Integer> permutation =
        random
            .ints(0, jumpDestinationsSize - 1)
            .distinct()
            .limit(jumpDestinationsSize - 1)
            .boxed()
            .collect(Collectors.toList());
    permutation.add(
        jumpDestinationsSize - 1); // The last jump has to be always to the last instruction
    return permutation;
  }

  private Map<Integer, EWord> computeJumpDestinationsToPCForJUMP(List<Integer> jumpDestinations) {
    Map<Integer, EWord> jumpDestinationsToPC = new HashMap<>();
    for (int i = 0; i < jumpDestinations.size(); i++) {
      jumpDestinationsToPC.put(
          jumpDestinations.get(i), EWord.of(jumpDestinations.get(i) + (i + 1) * 31L));
    }
    return jumpDestinationsToPC;
  }

  private Map<Integer, EWord> computeJumpDestinationsToPCForJUMPI(List<Integer> jumpDestinations) {
    Map<Integer, EWord> jumpDestinationsToPC = new HashMap<>();
    for (int i = 0; i < jumpDestinations.size(); i++) {
      jumpDestinationsToPC.put(
          jumpDestinations.get(i), EWord.of(jumpDestinations.get(i) + (i + 1) * 62L));
    }
    return jumpDestinationsToPC;
  }

  private List<Bytes> initByteCode(int byteCodeNumberOfElements) {
    return IntStream.range(0, byteCodeNumberOfElements)
        .mapToObj(i -> Bytes.of(OpCode.STOP.byteValue()))
        .collect(Collectors.toCollection(ArrayList::new));
  }

  private void appendJump(EWord pcNew, BytecodeCompiler program) {
    program.push(pcNew);
    program.op(OpCode.JUMP);
    System.out.println("Added JUMP at PC: " + program.compile().bitLength() / 8);
    // 32 + 1 + 1 bytes
  }

  private void appendJumpi(EWord pcNew, EWord jumpCondition, BytecodeCompiler program) {
    program.push(jumpCondition);
    program.push(pcNew);
    program.op(OpCode.JUMPI);
    System.out.println("Added JUMPI at PC: " + program.compile().bitLength() / 8);
    // 32 + 1 + 32 + 1 + 1 bytes
  }

  private void appendJumpDest(BytecodeCompiler program) {
    program.op(OpCode.JUMPDEST);
    System.out.println("Added JUMPDEST at PC: " + program.compile().bitLength() / 8);
    // 1 byte
  }

  private void appendStop(BytecodeCompiler program) {
    program.op(OpCode.STOP);
    System.out.println("Added STOP at PC: " + program.compile().bitLength() / 8);
    // 1 byte
  }
}
