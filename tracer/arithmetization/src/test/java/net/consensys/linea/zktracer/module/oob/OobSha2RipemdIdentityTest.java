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

import static org.junit.jupiter.api.Assertions.assertEquals;

import com.google.common.io.BaseEncoding;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.util.Random;
import java.util.stream.Collectors;
import java.util.stream.IntStream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.bouncycastle.crypto.digests.RIPEMD160Digest;
import org.bouncycastle.util.encoders.Hex;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class OobSha2RipemdIdentityTest extends TracerTestBase {
  Random random = new Random(1L);
  static final int[] argSizes =
      new int[] {0, 1, 10, 20, 31, 32, 33, 63, 64, 65, 95, 96, 97, 127, 128, 129, 1000, 2000};

  // https://coderpad.io/blog/development/writing-a-parameterized-test-in-junit-with-examples/
  // https://stackoverflow.com/questions/76124016/pass-externally-defined-variable-to-junit-valuesource-annotation-in-a-paramete
  static int[] argSizesSource() {
    return argSizes;
  }

  @ParameterizedTest
  @MethodSource("argSizesSource")
  void testSha2(int argSize, TestInfo testInfo) throws NoSuchAlgorithmException {
    String data = generateHexString(argSize);
    ProgramAndRetInfo programAndRetInfo = initProgramInvokingPrecompile(data, Address.SHA256);
    BytecodeCompiler program = programAndRetInfo.program();

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    String referenceComputedHash = sha256(data);
    final Hub hub = bytecodeRunner.getHub();
    String prcComputedHash = hub.currentFrame().frame().getReturnData().toString();

    // String returnDataSizeMaybe = hub.currentFrame().frame().getStackItem(0).toString();
    // System.out.println("RETURNDATASIZE after a SHA2 CALL:" + returnDataSizeMaybe);
    // System.out.println("Test SHA2-256 with random argSize = " + argSize);
    // System.out.println("Inp: 0x" + data);
    // System.out.println("Ref: " + referenceComputedHash);
    // System.out.println("Com: " + prcComputedHash);

    assertEquals(referenceComputedHash, prcComputedHash);
  }

  @ParameterizedTest
  @MethodSource("argSizesSource")
  void testIdentity(int argSize, TestInfo testInfo) {
    String data = generateHexString(argSize);
    ProgramAndRetInfo programAndRetInfo = initProgramInvokingPrecompile(data, Address.ID);
    BytecodeCompiler program = programAndRetInfo.program();

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    String returnedData = bytecodeRunner.getHub().currentFrame().frame().getReturnData().toString();
    // System.out.println(returnedData);
    // System.out.println("Test IDENTITY with random argSize = " + argSize);
    // System.out.println("Inp: 0x" + data);
    // System.out.println("Ret: " + returnedData);
    assertEquals("0x" + data.toLowerCase(), returnedData);
  }

  @ParameterizedTest
  @MethodSource("argSizesSource")
  void testRipmd(int argSize, TestInfo testInfo) {
    String data = generateHexString(argSize);
    ProgramAndRetInfo programAndRetInfo = initProgramInvokingPrecompile(data, Address.RIPEMD160);
    BytecodeCompiler program = programAndRetInfo.program();

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    String referenceComputedHash = ripemd160(data);
    String prcComputedHash =
        bytecodeRunner.getHub().currentFrame().frame().getReturnData().toString();

    // System.out.println("Test RIPEMD-160 with random argSize = " + argSize);
    // System.out.println("Inp: 0x" + data);
    // System.out.println("Ref: " + referenceComputedHash);
    // System.out.println("Com: " + prcComputedHash);

    assertEquals(referenceComputedHash, prcComputedHash);
  }

  // Support methods
  private String generateHexString(int size) {
    return IntStream.range(0, size)
        .mapToObj(i -> String.format("%02x", random.nextInt(256)))
        .collect(Collectors.joining());
  }

  private String sha256(String hexString) throws NoSuchAlgorithmException {
    byte[] byteInput = BaseEncoding.base16().decode(hexString.toUpperCase());
    MessageDigest digest = MessageDigest.getInstance("SHA-256");
    byte[] hash = digest.digest(byteInput);
    return "0x" + BaseEncoding.base16().encode(hash).toLowerCase();
  }

  private static String ripemd160(String hexString) {
    byte[] byteInput = BaseEncoding.base16().decode(hexString.toUpperCase());
    RIPEMD160Digest digest = new RIPEMD160Digest();
    digest.update(byteInput, 0, byteInput.length);
    byte[] hash = new byte[digest.getDigestSize()];
    digest.doFinal(hash, 0);
    return "0x000000000000000000000000" + Hex.toHexString(hash);
  }

  @Test
  void testPrcSupportMethods() throws NoSuchAlgorithmException {
    String data = generateHexString(32);
    System.out.println("SHA2-256 of random data: " + sha256(data));
    System.out.println("RIPEMD-160 of random data: " + ripemd160(data));
    assert (sha256("")
        .equals("0xe3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"));
    assert (sha256("00")
        .equals("0x6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d"));
    assert (sha256("0000")
        .equals("0x96a296d224f285c67bee93c30f8a309157f0daa35dc5b87e410b78630a09cfc7"));
    assert (sha256("ff")
        .equals("0xa8100ae6aa1940d0b663bb31cd466142ebbdbd5187131b92d93818987832eb89"));
    assert (sha256(
            "aaaaaaaaaa9999999999bbbbbbbbbb8888888888cccccccccc7777777777ddddaaaaaaaaaa9999999999bbbbbbbbbb8888888888cccccccccc7777777777dddd")
        .equals("0xfa3695ebdadeb06f552f983ff12deea809391ca80d10f0dd27fea25ae8a6daa7"));
    assert (ripemd160("")
        .equals("0x0000000000000000000000009c1185a5c5e9fc54612808977ee8f548b2258d31"));
    assert (ripemd160("00")
        .equals("0x000000000000000000000000c81b94933420221a7ac004a90242d8b1d3e5070d"));
    assert (ripemd160("0000")
        .equals("0x000000000000000000000000f7d50d120d655be4b88750873e00caf147f28a1b"));
    assert (ripemd160("ff")
        .equals("0x0000000000000000000000002c0c45d3ecab80fe060e5f1d7057cd2f8de5e557"));
    assert (ripemd160(
            "aaaaaaaaaa9999999999bbbbbbbbbb8888888888cccccccccc7777777777ddddaaaaaaaaaa9999999999bbbbbbbbbb8888888888cccccccccc7777777777dddd")
        .equals("0x0000000000000000000000009c08e833ee0d5d3e42f332e2d22563b68617bfba"));
  }

  private String padToEWord(String input, boolean toTheLeft) {
    if (toTheLeft) {
      return "00".repeat(32 - input.length() / 2) + input;
    } else {
      return input + "00".repeat(32 - input.length() / 2);
    }
  }

  private String subHexString(String input, int from, int to) {
    return input.substring(from * 2, to * 2);
  }

  private String subHexString(String input, int from) {
    return input.substring(from * 2);
  }

  private record ProgramAndRetInfo(
      BytecodeCompiler program, int argSize, int argOffset, int retSize, int retOffset) {}

  ProgramAndRetInfo initProgramInvokingPrecompile(String data, Address address) {
    int argSize = data.length() / 2;
    int argOffset = 0;

    int retSize = address == Address.ID ? argSize : 32;
    int retOffset = 0;

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    // MSTORE data if argSize > 0
    if (argSize > 0) {
      // Note that argSize <= 32 is treated in a slightly different way than argSize > 32 to avoid
      // splitting the input and padding
      if (argSize <= 32) {
        // The random offset is applied before the EWord
        // Generate a small random offset
        int randomOffset = random.nextInt(1, 5);
        argOffset = randomOffset + 32 - argSize;
        retOffset = address == Address.ID ? randomOffset + 64 - argSize : randomOffset + 32;

        program
            .push(data)
            . // data
            push(randomOffset)
            .op(OpCode.MSTORE);
      } else { // argSize > 32
        // The random offset is applied between the beginning of the first EWord and the input
        // Generate a small random offset
        argOffset = random.nextInt(1, 5);
        retOffset = argOffset + argSize;

        // MSTORE first EWord
        String firstEWord = padToEWord(subHexString(data, 0, 32 - argOffset), true);
        program
            .push(firstEWord)
            . // data
            push(
                0) // The argOffset is already taken into consideration by padding zeros to the left
            .op(OpCode.MSTORE);

        // MSTORE EWord in the middle
        int i = 1;
        while (32 * i - argOffset + 32 < argSize) {
          String middleEWord = subHexString(data, 32 * i - argOffset, 32 * i - argOffset + 32);
          program
              .push(middleEWord)
              . // data
              push(32 * i)
              .op(OpCode.MSTORE);
          i++;
        }

        // MSTORE last EWord
        String lastEWord = padToEWord(subHexString(data, 32 * i - argOffset), false);
        program
            .push(lastEWord)
            . // data
            push(32 * i)
            .op(OpCode.MSTORE);
      }
    }

    program
        .push(retSize)
        . // retSize
        push(retOffset)
        . // retOffset
        push(argSize)
        . // argSize
        push(argOffset)
        . // argOffset
        push(address.getBytes())
        . // address
        push("FFFFFFFF")
        . // gas
        op(OpCode.STATICCALL)
        .op(OpCode.RETURNDATASIZE);

    return new ProgramAndRetInfo(program, argSize, argOffset, retSize, retOffset);
  }

  @ParameterizedTest
  @MethodSource("argSizesSource")
  void testInitProgramInvokingPrecompileDataInMemorySupportMethod(int argSize, TestInfo testInfo) {
    // This test is to ensure that the data written in memory is the same as the input data
    String data = generateHexString(argSize);

    ProgramAndRetInfo programAndRetInfo = initProgramInvokingPrecompile(data, Address.ZERO);
    BytecodeCompiler program = programAndRetInfo.program();

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    String dataInMemory =
        bytecodeRunner
            .getHub()
            .currentFrame()
            .frame()
            .shadowReadMemory(programAndRetInfo.argOffset, programAndRetInfo.argSize)
            .toString();
    System.out.println("0x" + data);
    System.out.println(dataInMemory);
    assertEquals("0x" + data.toLowerCase(), dataInMemory);
  }
}
