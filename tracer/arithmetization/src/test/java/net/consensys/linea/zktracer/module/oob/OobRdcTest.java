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

import static net.consensys.linea.zktracer.module.hub.signals.TracedException.RETURN_DATA_COPY_FAULT;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.math.BigInteger;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.signals.Exceptions;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class OobRdcTest extends TracerTestBase {

  public static final BigInteger TWO_POW_128_LEFT =
      BigInteger.ONE.shiftLeft(128).subtract(BigInteger.valueOf(100));

  public static final BigInteger TWO_POW_128_RIGHT =
      BigInteger.ONE.shiftLeft(128).subtract(BigInteger.valueOf(100));

  @Test
  void testReturnDataCopyMaxPosZero(TestInfo testInfo) {
    // maxPos = offset + size = 0 + 0 < rds = 32
    BytecodeCompiler program = initReturnDataCopyProgram(BigInteger.ZERO, BigInteger.ZERO);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
  }

  @Test
  void testReturnDataCopyMaxPosRds(TestInfo testInfo) {
    // maxPos = offset + size = 12 + 20 = rds = 32
    BytecodeCompiler program =
        initReturnDataCopyProgram(BigInteger.valueOf(12), BigInteger.valueOf(20));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
  }

  @Test
  void testReturnDataCopyMaxPosSmallerThanRds(TestInfo testInfo) {
    // maxPos = offset + size = 3 + 4 < rds = 32
    BytecodeCompiler program =
        initReturnDataCopyProgram(BigInteger.valueOf(3), BigInteger.valueOf(4));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
  }

  @Test
  void testReturnDataCopyMaxPosSmallerThanRdsAndOffsetZero(TestInfo testInfo) {
    // maxPos = offset + size = 0 + 4 < rds = 32
    BytecodeCompiler program =
        initReturnDataCopyProgram(BigInteger.valueOf(0), BigInteger.valueOf(4));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
  }

  @Test
  void testReturnDataCopyMaxPosSmallerThanRdsAndSizeZero(TestInfo testInfo) {
    // maxPos = offset + size = 3 + 0 < rds = 32
    BytecodeCompiler program =
        initReturnDataCopyProgram(BigInteger.valueOf(3), BigInteger.valueOf(0));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
  }

  // Failing cases

  // offset smaller cases
  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetSmallerAndSizeSmall(TestInfo testInfo) {
    // maxPos = offset + size = 10 + 23 > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgram(BigInteger.valueOf(10), BigInteger.valueOf(23));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetSmallerAndSizeBigLeft(TestInfo testInfo) {
    // maxPos = offset + size = 10 + TWO_POW_128_LEFT > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(BigInteger.valueOf(10), TWO_POW_128_LEFT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetSmallerAndSizeBigRight(TestInfo testInfo) {
    // maxPos = offset + size = 10 + TWO_POW_128_RIGHT > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(BigInteger.valueOf(10), TWO_POW_128_RIGHT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  // offset just greater cases
  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetJustGreaterAndSizeZero(TestInfo testInfo) {
    // maxPos = offset + size = 33 + 0 > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgram(BigInteger.valueOf(33), BigInteger.valueOf(0));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetJustGreaterAndSizeSmall(TestInfo testInfo) {
    // maxPos = offset + size = 33 + 23 > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgram(BigInteger.valueOf(33), BigInteger.valueOf(23));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetJustGreaterAndSizeBigLeft(TestInfo testInfo) {
    // maxPos = offset + size = 33 + TWO_POW_128_LEFT > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(BigInteger.valueOf(33), TWO_POW_128_LEFT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetJustGreaterAndSizeBigRight(
      TestInfo testInfo) {
    // maxPos = offset + size = 33 + TWO_POW_128_RIGHT > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(BigInteger.valueOf(33), TWO_POW_128_RIGHT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  // offset big left cases
  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetBigLeftAndSizeZero(TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_LEFT + 0 > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(TWO_POW_128_LEFT, BigInteger.valueOf(0));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetBigLeftAndSizeSmall(TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_LEFT + 23 > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(TWO_POW_128_LEFT, BigInteger.valueOf(23));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetBigLeftAndSizeBigLeft(TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_LEFT + TWO_POW_128_LEFT > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(TWO_POW_128_LEFT, TWO_POW_128_LEFT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetBigLeftAndSizeBigRight(TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_LEFT + TWO_POW_128_RIGHT > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(TWO_POW_128_LEFT, TWO_POW_128_RIGHT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  // offset big right cases
  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetBigRightAndSizeZero(TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_RIGHT + 0 > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(TWO_POW_128_RIGHT, BigInteger.valueOf(0));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetBigRightAndSizeSmall(TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_RIGHT + 23 > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(TWO_POW_128_RIGHT, BigInteger.valueOf(23));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetBigRightAndSizeBigLeft(TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_Right + TWO_POW_128_LEFT > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(TWO_POW_128_RIGHT, TWO_POW_128_LEFT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyMaxPosGreaterThanRdsAndOffsetBigRightAndSizeBigRight(TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_RIGHT + TWO_POW_128_RIGHT > 32 = rds
    BytecodeCompiler program = initReturnDataCopyProgram(TWO_POW_128_RIGHT, TWO_POW_128_RIGHT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  // Same cases but using identity precompile
  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosZero(TestInfo testInfo) {
    // maxPos = offset + size = 0 + 0 < rds = 32
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(BigInteger.ZERO, BigInteger.ZERO);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
  }

  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosRds(TestInfo testInfo) {
    // maxPos = offset + size = 12 + 20 = rds = 32
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(
            BigInteger.valueOf(12), BigInteger.valueOf(20));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();
    System.out.println(bytecodeRunner.getHub().currentFrame().frame().getReturnData());

    assertFalse(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
  }

  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosSmallerThanRds(TestInfo testInfo) {
    // maxPos = offset + size = 3 + 4 < rds = 32
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(
            BigInteger.valueOf(3), BigInteger.valueOf(4));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
  }

  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosSmallerThanRdsAndOffsetZero(
      TestInfo testInfo) {
    // maxPos = offset + size = 0 + 4 < rds = 32
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(
            BigInteger.valueOf(0), BigInteger.valueOf(4));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
  }

  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosSmallerThanRdsAndSizeZero(TestInfo testInfo) {
    // maxPos = offset + size = 3 + 0 < rds = 32
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(
            BigInteger.valueOf(3), BigInteger.valueOf(0));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertFalse(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
  }

  // Failing cases

  // offset smaller cases
  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetSmallerAndSizeSmall(
      TestInfo testInfo) {
    // maxPos = offset + size = 10 + 23 > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(
            BigInteger.valueOf(10), BigInteger.valueOf(23));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();
    System.out.println(bytecodeRunner.getHub().currentFrame().frame().getReturnData());

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetSmallerAndSizeBigLeft(
      TestInfo testInfo) {
    // maxPos = offset + size = 10 + TWO_POW_128_LEFT > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(BigInteger.valueOf(10), TWO_POW_128_LEFT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetSmallerAndSizeBigRight(
      TestInfo testInfo) {
    // maxPos = offset + size = 10 + TWO_POW_128_RIGHT > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(BigInteger.valueOf(10), TWO_POW_128_RIGHT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  // offset just greater cases
  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetJustGreaterAndSizeZero(
      TestInfo testInfo) {
    // maxPos = offset + size = 33 + 0 > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(
            BigInteger.valueOf(33), BigInteger.valueOf(0));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void
      testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetJustGreaterAndSizeSmall(
          TestInfo testInfo) {
    // maxPos = offset + size = 33 + 23 > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(
            BigInteger.valueOf(33), BigInteger.valueOf(23));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void
      testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetJustGreaterAndSizeBigLeft(
          TestInfo testInfo) {
    // maxPos = offset + size = 33 + TWO_POW_128_LEFT > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(BigInteger.valueOf(33), TWO_POW_128_LEFT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void
      testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetJustGreaterAndSizeBigRight(
          TestInfo testInfo) {
    // maxPos = offset + size = 33 + TWO_POW_128_RIGHT > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(BigInteger.valueOf(33), TWO_POW_128_RIGHT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  // offset big left cases
  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetBigLeftAndSizeZero(
      TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_LEFT + 0 > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(TWO_POW_128_LEFT, BigInteger.valueOf(0));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetBigLeftAndSizeSmall(
      TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_LEFT + 23 > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(TWO_POW_128_LEFT, BigInteger.valueOf(23));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetBigLeftAndSizeBigLeft(
      TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_LEFT + TWO_POW_128_LEFT > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(TWO_POW_128_LEFT, TWO_POW_128_LEFT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetBigLeftAndSizeBigRight(
      TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_LEFT + TWO_POW_128_RIGHT > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(TWO_POW_128_LEFT, TWO_POW_128_RIGHT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  // offset big right cases
  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetBigRightAndSizeZero(
      TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_RIGHT + 0 > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(TWO_POW_128_RIGHT, BigInteger.valueOf(0));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetBigRightAndSizeSmall(
      TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_RIGHT + 23 > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(TWO_POW_128_RIGHT, BigInteger.valueOf(23));
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetBigRightAndSizeBigLeft(
      TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_Right + TWO_POW_128_LEFT > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(TWO_POW_128_RIGHT, TWO_POW_128_LEFT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void
      testReturnDataCopyUsingIdentityPrecompileMaxPosGreaterThanRdsAndOffsetBigRightAndSizeBigRight(
          TestInfo testInfo) {
    // maxPos = offset + size = TWO_POW_128_RIGHT + TWO_POW_128_RIGHT > 32 = rds
    BytecodeCompiler program =
        initReturnDataCopyProgramUsingIdentityPrecompile(TWO_POW_128_RIGHT, TWO_POW_128_RIGHT);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    Hub hub = bytecodeRunner.getHub();

    assertTrue(Exceptions.returnDataCopyFault(hub.pch().exceptions()));
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  // Support methods
  BytecodeCompiler initReturnDataCopyProgram(BigInteger offset, BigInteger size) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    // Creates a constructor that creates a contract which returns 32 FF
    program
        .push("7F7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
        .push(0)
        .op(OpCode.MSTORE)
        .push("FF6000527FFF60005260206000F3000000000000000000000000000000000000")
        .push(32)
        .op(OpCode.MSTORE)
        .push("000000000060205260296000F300000000000000000000000000000000000000")
        .push(64)
        .op(OpCode.MSTORE);

    // Create the contract with the constructor code above
    program
        .push(77)
        .push(0)
        .push(0)
        .op(OpCode.CREATE); // Puts the new contract address on the stack

    // Call the deployed contract
    program.push(0).push(0).push(0).push(0).op(OpCode.DUP5).push("FFFFFFFF").op(OpCode.STATICCALL);

    // Clear the stack
    program.op(OpCode.POP).op(OpCode.POP);

    // Clear the memory
    program
        .push(0)
        .push(0)
        .op(OpCode.MSTORE)
        .push(0)
        .push(32)
        .op(OpCode.MSTORE)
        .push(0)
        .push(64)
        .op(OpCode.MSTORE);

    // Invoke RETURNDATACOPY
    program.push(size).push(offset).push(0).op(OpCode.RETURNDATACOPY);

    return program;
  }

  BytecodeCompiler initReturnDataCopyProgramUsingIdentityPrecompile(
      BigInteger offset, BigInteger size) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    // First place the parameters in memory
    program
        .push("AAAAAAAAAA9999999999BBBBBBBBBB8888888888CCCCCCCCCC7777777777DDDD")
        . // data
        push(0)
        .op(OpCode.MSTORE);

    // Do the call
    program
        .push(0)
        . // retSize
        push(0)
        . // retOffset
        push(32)
        . // argSize
        push(0)
        . // argOffset
        push(4)
        . // address
        push("FFFFFFFF")
        . // gas
        op(OpCode.STATICCALL);

    // Clear the stack
    program.op(OpCode.POP);

    // Clear the memory
    program.push(0).push(0).op(OpCode.MSTORE);

    // Invoke RETURNDATACOPY
    program.push(size).push(offset).push(0).op(OpCode.RETURNDATACOPY);

    return program;
  }
}
