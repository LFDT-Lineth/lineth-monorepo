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

import static net.consensys.linea.zktracer.Fork.forkPredatesOsaka;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.keyPair;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.userAccount;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.BASE_MIN_OFFSET;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.BBS_MIN_OFFSET;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.EBS_MIN_OFFSET;
import static net.consensys.linea.zktracer.module.hub.precompiles.ModexpMetadata.MBS_MIN_OFFSET;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.*;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class ModexpTests extends TracerTestBase {

  // some 10 decimal digit primes in the range [256 ** 3, 256 ** 4[
  // * 1081914797 ≡ 0x407CB5AD
  // * 1086173677 ≡ 0x40BDB1ED
  // * 1219462969 ≡ 0x48AF8739
  // * 2198809297 ≡ 0x830F2AD1
  // * 3752945107 ≡ 0xDFB165D3
  // * 3772857469 ≡ 0xE0E13C7D
  // * 3952396501 ≡ 0xEB94C8D5
  // * 4171332133 ≡ 0xF8A17A25

  @Test
  void basicModexpTest(TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(0)
            .push(0)
            .push(0)
            .push(0)
            .push(0)
            .push(Address.MODEXP) // address
            .push(0xffff) // gas
            .op(OpCode.CALL)
            .op(OpCode.POP)
            .compile();

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(bytecode);
    bytecodeRunner.run(chainConfig, testInfo);

    // check precompile limits line count
    assertEquals(1, bytecodeRunner.getHub().modexpEffectiveCall().lineCount());
  }

  @Test
  void basicNonTrivialModexpTest(TestInfo testInfo) {

    final int modexpComponentByteSize = 1024;

    final int base = 2;
    final int exp = 5;
    final int mod = 7;
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(1)
            .push(BBS_MIN_OFFSET)
            .op(OpCode.MSTORE)
            .push(1)
            .push(EBS_MIN_OFFSET)
            .op(OpCode.MSTORE)
            .push(1)
            .push(MBS_MIN_OFFSET)
            .op(OpCode.MSTORE)
            .push(base)
            .push(BASE_MIN_OFFSET)
            .op(OpCode.MSTORE8)
            .push(exp)
            .push(BASE_MIN_OFFSET + 1)
            .op(OpCode.MSTORE8)
            .push(mod)
            .push(BASE_MIN_OFFSET + 2)
            .op(OpCode.MSTORE8)
            .push(modexpComponentByteSize) // retLength
            .push(0) // retOffset
            .push(BASE_MIN_OFFSET + 3) // argLength
            .push(0) // argOffset
            .push(0) // value
            .push(0x05) // address
            .push(0xffff) // gas
            .op(OpCode.CALL)
            .op(OpCode.POP)
            .compile();

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(bytecode);
    bytecodeRunner.run(chainConfig, testInfo);

    // check precompile limits line count
    assertEquals(1, bytecodeRunner.getHub().modexpEffectiveCall().lineCount());
  }

  @Test
  void testSingleUnpaddedModexp(TestInfo testInfo) {

    String hexBase = "407CB5AD";
    String hexExpn = "40BDB1ED";
    String hexModl = "48AF8739";

    BytecodeCompiler program =
        preparingSingleBaseExponentAndModulusForModexp(hexBase, hexExpn, hexModl);

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    // check precompile limits line count
    assertTrue(bytecodeRunner.getHub().modexpEffectiveCall().lineCount() > 0);
  }

  @Test
  void testUnpaddedModexp(TestInfo testInfo) {

    String hexBase = "407CB5AD";
    String hexExpn = "40BDB1ED";
    String hexModl = "48AF8739";

    BytecodeCompiler program =
        preparingBaseExponentAndModulusForModexpAndRunningVariousModexps(hexBase, hexExpn, hexModl);

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    // check precompile limits line count
    assertTrue(bytecodeRunner.getHub().modexpEffectiveCall().lineCount() > 0);
  }

  @Test
  void testSinglePaddedModexp(TestInfo testInfo) {

    String hexBase = "00407CB5AD";
    String hexExpn = "40BDB1ED";
    String hexModl = "000048AF8739";

    BytecodeCompiler program =
        preparingSingleBaseExponentAndModulusForModexp(hexBase, hexExpn, hexModl);

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    // check precompile limits line count
    assertTrue(bytecodeRunner.getHub().modexpEffectiveCall().lineCount() > 0);
  }

  BytecodeCompiler preparingSingleBaseExponentAndModulusForModexp(
      String hexBase, String hexExpn, String hexModl) {

    BytecodeCompiler program = preparingBaseExponentAndModulusForModexp(hexBase, hexExpn, hexModl);

    appendParametrizedModexpCall(program, 32, 32);

    return program;
  }

  @Test
  void testPaddedModexp(TestInfo testInfo) {

    String hexBase = "00407CB5AD";
    String hexExpn = "40BDB1ED";
    String hexModl = "000048AF8739";

    BytecodeCompiler program =
        preparingBaseExponentAndModulusForModexpAndRunningVariousModexps(hexBase, hexExpn, hexModl);

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    // check precompile limits line count
    assertTrue(bytecodeRunner.getHub().modexpEffectiveCall().lineCount() > 0);
  }

  int byteSize(String hexString) {
    return (hexString.length() + 1) / 2;
  }

  BytecodeCompiler preparingBaseExponentAndModulusForModexpAndRunningVariousModexps(
      String hexBase, String hexExpn, String hexModl) {

    BytecodeCompiler program = preparingBaseExponentAndModulusForModexp(hexBase, hexExpn, hexModl);

    int bbs = byteSize(hexBase);
    int ebs = byteSize(hexExpn);
    int mbs = byteSize(hexModl);

    List<Integer> returnAtCapacityList = List.of(0, 1, mbs - 1, mbs, mbs + 1, 31, 32);
    List<Integer> callDataSizeList =
        List.of(
            0,
            1,
            31,
            32,
            33,
            63,
            64,
            65,
            95,
            96,
            97,
            96 + (bbs - 1),
            96 + bbs,
            96 + bbs + 1,
            96 + bbs + (ebs - 1),
            96 + bbs + ebs,
            96 + bbs + ebs + 1,
            96 + bbs + ebs + (mbs - 1),
            96 + bbs + ebs + mbs,
            96 + bbs + ebs + mbs + 1);

    for (int returnAtCapacity : returnAtCapacityList) {
      for (int callDataSize : callDataSizeList) {
        appendParametrizedModexpCall(program, returnAtCapacity, callDataSize);
      }
    }

    return program;
  }

  void appendParametrizedModexpCall(
      BytecodeCompiler program, int returnAtCapacity, int callDataSize) {
    program
        .push(returnAtCapacity) // r@c
        .push(Bytes.fromHexString("0100")) // r@o
        .push(callDataSize) // cds
        .push(Bytes.fromHexString("")) // cdo
        .push(Address.MODEXP) // address (here: MODEXP)
        .push(Bytes.fromHexString("ffff")) // gas
        .op(OpCode.DELEGATECALL)
        .op(OpCode.POP);
  }

  BytecodeCompiler preparingBaseExponentAndModulusForModexp(
      String hexBase, String hexExpn, String hexModl) {

    int bbs = byteSize(hexBase);
    int ebs = byteSize(hexExpn);
    int mbs = byteSize(hexModl);
    int baseOffset = 64 + bbs;
    int expnOffset = 64 + bbs + ebs;
    int modlOffset = 64 + bbs + ebs + mbs;
    return BytecodeCompiler.newProgram(chainConfig)
        // modulus, exponent and base values at correct offsets
        .push(hexModl)
        .push(modlOffset)
        .op(OpCode.MSTORE) // this sets the modulus
        .push(hexExpn)
        .push(expnOffset)
        .op(OpCode.MSTORE) // this sets the exponent
        .push(hexBase)
        .push(baseOffset)
        .op(OpCode.MSTORE) // this sets the base
        // bbs, ebs, mbs at correct offsets
        .push(byteSize(hexBase))
        .push("00")
        .op(OpCode.MSTORE) // this sets bbs = 4
        .push(byteSize(hexExpn))
        .push("20")
        .op(OpCode.MSTORE) // this sets ebs = 4
        .push(byteSize(hexModl))
        .push("40")
        .op(OpCode.MSTORE) // this sets mbs = 4
    // to read call data 32 + 32 + 32 + 4 + 4 + 4 = 108 bytes are sufficient
    ;
  }

  @Test
  void variationsOnEmptyCalls(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    List<Integer> callDataSizeList = List.of(0, 1, 31, 32, 33, 63, 64, 65, 95, 96, 97, 128);
    for (int callDataSize : callDataSizeList) {
      appendAllZeroCallDataModexpCalls(program, callDataSize);
    }

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  /**
   * This test was extracted from {@link BlockchainReferenceTest_339}, specifically {@link
   * modexp_modsize0_returndatasize_d4g0v0_London}. It <b>FAILS</b> as our tests don't have a
   * popping mechanism.
   */
  @Disabled
  @Test
  void hugeMbsShortCdsModexpCallPlusReturnDataSize(TestInfo testInfo) {

    Bytes compiledCode =
        Bytes.fromHexString(
            "36600060003760206103e8366000600060055af26001556103e8516002553d60035500");
    Bytes transactionCallData =
        Bytes.fromHexString(
            "0x0000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000f3f140101010000000000000000000000000000000000000000000000000000000000");

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .nonce(59)
            .code(compiledCode)
            .balance(Wei.fromEth(1))
            .address(Address.fromHexString("dddddddddddddddddddddddddddddddddddddddd"))
            .build();

    final Transaction transaction =
        ToyTransaction.builder()
            .sender(userAccount)
            .to(recipientAccount)
            .keyPair(keyPair)
            .payload(transactionCallData)
            .gasPrice(Wei.of(8))
            .value(Wei.of(123))
            .build();

    List<Transaction> transactions = new ArrayList<>();
    transactions.add(transaction);

    List<ToyAccount> accounts = List.of(userAccount, recipientAccount);

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(accounts)
        .transactions(transactions)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  void appendAllZeroCallDataModexpCalls(BytecodeCompiler program, int callDataSize) {
    program
        .push(Bytes.fromHexString("0200")) // rds 0x0200 ≡ 512 in decimal
        .push(Bytes.fromHexString("0200")) // rdo
        .push(callDataSize) // cds
        .push(Bytes.fromHexString("00")) // cdo
        .push(0x04) // i.e. MODEXP
        .push(Bytes.fromHexString("ffff")) // gas
        .op(OpCode.STATICCALL)
        .op(OpCode.POP);
  }

  @Test
  // We trigger a ModexpData MMU Call where the sourceOffset = referenceSize for the MmuCall
  void referenceSizeEqualsSourceOffset(TestInfo testInfo) {
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            // bbs = 2
            .push(Bytes32.leftPad(Bytes.of(2)))
            .push(0) // offset
            .op(OpCode.MSTORE)
            // ebs = 3
            .push(Bytes32.leftPad(Bytes.of(3)))
            .push(32) // offset
            .op(OpCode.MSTORE)
            // mbs = 4
            .push(Bytes32.leftPad(Bytes.of(4)))
            .push(64) // offset
            .op(OpCode.MSTORE)
            // MSTORE ebm
            .push(Bytes32.rightPad(Bytes.fromHexString("0xba7e000ec70000080d")))
            .push(96)
            .op(OpCode.MSTORE)
            // Call Modexp
            .push(0) // returnSize
            .push(0) // returnOffset
            .push(98) // cds = 96 + bbs => trigger a MMU Call where the sourceOffset = referenceSize
            .push(0) // cdo
            .push(0) // value
            .push(Address.MODEXP) // address
            .push(0xffff) // gas
            .op(OpCode.CALL)
            .op(OpCode.POP)
            .compile();
    BytecodeRunner.of(bytecode).run(chainConfig, testInfo);
  }

  @Test
  // This test a modexp call with bbs > 512
  void unprovableModexp(TestInfo testInfo) {
    final int bbs = 513;
    final int ebs = 3;
    final int mbs = 4;

    // call data =
    // 0x
    // + 00 .. bbs
    // + 00 .. ebs
    // + 00 .. mbs
    // + base (bbs many bytes)
    // + exponent (ebs many bytes)
    // + modulus (mbs many bytes)
    // where base ≡ 0x 00 00 .. 00 00 ba 7e is 513 bytes long
    final Bytes bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            // MSTORE the modulus
            .push(Bytes32.rightPad(Bytes.fromHexString("0x0d0d0d0d")))
            .push(96 + bbs + ebs + mbs - WORD_SIZE)
            .op(OpCode.MSTORE)
            // MSTORE the exponent
            .push(Bytes32.rightPad(Bytes.fromHexString("0xeeeeee")))
            .push(96 + bbs + ebs - WORD_SIZE)
            .op(OpCode.MSTORE)
            // MSTORE the base = 0x00 .. 00 ba 7e (bbs many bytes)
            .push(Bytes32.rightPad(Bytes.fromHexString("0xba7e")))
            .push(96 + bbs - WORD_SIZE)
            .op(OpCode.MSTORE)
            // bbs = 513
            .push(Bytes32.leftPad(Bytes.minimalBytes(bbs)))
            .push(BBS_MIN_OFFSET) // offset
            .op(OpCode.MSTORE)
            // ebs = 3
            .push(Bytes32.leftPad(Bytes.of(ebs)))
            .push(EBS_MIN_OFFSET) // offset
            .op(OpCode.MSTORE)
            // mbs = 4
            .push(Bytes32.leftPad(Bytes.of(mbs)))
            .push(MBS_MIN_OFFSET) // offset
            .op(OpCode.MSTORE)
            // Call Modexp
            .push(mbs) // returnSize
            .push(96 + bbs + ebs + mbs) // returnOffset
            .push(
                96 + bbs + ebs
                    + mbs) // cds = 96 + bbs => trigger a MMU Call where the sourceOffset =
            .push(0) // cdo
            .push(0) // value
            .push(Address.MODEXP) // address
            .push(0xffffffff) // gas
            .op(OpCode.CALL)
            .op(OpCode.POP)
            .compile();
    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(bytecode);
    try {
      bytecodeRunner.run(chainConfig, testInfo);
    } catch (Exception e) {
      // This is expected as the modexp call is unprovable
      if (!e.getMessage().contains("Final CallScenario, CALL_PRC_UNDEFINED, is still undefined")) {
        throw e;
      }
    }
    if (forkPredatesOsaka(fork)) {
      assertEquals(Integer.MAX_VALUE, bytecodeRunner.getHub().modexpEffectiveCall().lineCount());
    } else {
      assertEquals(1, bytecodeRunner.getHub().modexpEffectiveCall().lineCount());
    }
  }
}
