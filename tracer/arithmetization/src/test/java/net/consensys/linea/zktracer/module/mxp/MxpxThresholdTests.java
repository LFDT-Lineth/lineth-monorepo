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

package net.consensys.linea.zktracer.module.mxp;

import static net.consensys.linea.zktracer.Fork.isPostCancun;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.LLARGEMO;
import static net.consensys.linea.zktracer.Trace.LLARGEPO;
import static net.consensys.linea.zktracer.Trace.Mxp.CANCUN_MXPX_THRESHOLD;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE_MO;
import static net.consensys.linea.zktracer.opcode.OpCode.MLOAD;
import static net.consensys.linea.zktracer.opcode.OpCode.MSTORE;
import static net.consensys.linea.zktracer.opcode.OpCode.POP;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

public class MxpxThresholdTests extends TracerTestBase {

  static final BigInteger MAX_UINT256 =
      BigInteger.TWO.pow(256).subtract(BigInteger.ONE); // 2^256 - 1
  static final BigInteger CANCUN_MXPX_THRESHOLD_BI = BigInteger.valueOf(CANCUN_MXPX_THRESHOLD);
  static final BigInteger SMALL = BigInteger.valueOf(32);

  static final List<OpCode> oneOffsetOpCodes = List.of(OpCode.MSTORE, OpCode.MSTORE8);
  static final List<OpCode> oneOffsetSizePairOpCodes = List.of(OpCode.CODECOPY);
  static final List<OpCode> twoOffsetSizePairsOpCodes = List.of(OpCode.CALL);

  /**
   * The following tests were written to test a bug fix in the tracer <a
   * href="https://github.com/Consensys/linea-tracer/pull/2251">PR</a>. The `HUB` and the `MXP`
   * modules used to have different, incompatible criteria by which they recognized of memory
   * expansion exception.
   *
   * <p>One issue was that the London `HUB` compared a sum `offset + size` to some threshold, while
   * the `MXP` module did the same but with a different comparison operator (≤ vs <). The issue
   * arose whenever opcodes had _precisely_ `offset + size = 256**4`.
   *
   * <p>Another issue was that the Cancun `HUB` still used sums `offset + size` as its gauge rather
   * than the individual offsets and sizes.
   *
   * <p>As such we are testing with parameters in that neighbourhood. We distinguish 4 classes of
   * MXP instructions:
   * <li>constant size opcodes (e.g. `MSTORE`, `MSTORE8` with respective 'sizes' 32 and 1)
   *
   *     <p>and variable size opcodes:
   * <li>single offset, single size (e.g., `CODECOPY`)
   * <li>double offset, single size (only `MCOPY`), which is covered in a separated test
   * <li>double offset, double size (the `CALL`'s)
   */
  @Tag("nightly")
  @ParameterizedTest
  @MethodSource({"testMxpxThresholdSource"})
  void testMxpxThreshold(
      OpCode opCode,
      BigInteger offset1,
      BigInteger offset2,
      BigInteger size1,
      BigInteger size2,
      TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    switch (opCode) {
      // 1 offset
      case MSTORE, MSTORE8 ->
          program
              .push(0) // value
              .push(offset1)
              .op(opCode);
      // 1 offset, 1 size
      case CODECOPY -> program.push(size1).push(0).push(offset1).op(opCode);
      // Note that CODECOPY has 2 offsets, but only one is relevant from the MXP perspective
      // 2 offsets, 2 sizes
      case CALL ->
          program
              .push(size2)
              .push(offset2)
              .push(size1)
              .push(offset1)
              .push(0) // value
              .push(0) // address
              .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
              .op(opCode);
      default -> throw new IllegalArgumentException("Unsupported opCode: " + opCode);
    }

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  static Stream<Arguments> testMxpxThresholdSource() {
    final BigInteger MXPX_THRESHOLD = mxpxThreshold();
    List<Arguments> arguments = new ArrayList<>();
    List<BigInteger> values =
        List.of(
            BigInteger.ZERO,
            BigInteger.ONE,
            MXPX_THRESHOLD,
            MXPX_THRESHOLD.add(BigInteger.ONE),
            MAX_UINT256.subtract(BigInteger.valueOf(123)), // random huge number
            MAX_UINT256);

    for (OpCode opCode : oneOffsetOpCodes) {
      for (BigInteger offset1 : values) {
        arguments.add(Arguments.of(opCode, offset1, null, null, null));
      }
    }

    for (OpCode opCode : oneOffsetSizePairOpCodes) {
      for (BigInteger offset1 : values) {
        for (BigInteger size1 : values) {
          arguments.add(Arguments.of(opCode, offset1, null, size1, null));
        }
      }
    }

    for (OpCode opCode : twoOffsetSizePairsOpCodes) {
      for (BigInteger offset1 : values) {
        for (BigInteger offset2 : values) {
          for (BigInteger size1 : values) {
            for (BigInteger size2 : values) {
              arguments.add(Arguments.of(opCode, offset1, offset2, size1, size2));
            }
          }
        }
      }
    }

    final BigInteger MXP_THRESHOLD_DIVIDED_BY_TWO = MXPX_THRESHOLD.divide(BigInteger.TWO);
    final BigInteger MXP_THRESHOLD_MINUS_MXP_THRESHOLD_DIVIDED_BY_TWO =
        MXPX_THRESHOLD.subtract(MXP_THRESHOLD_DIVIDED_BY_TWO);

    for (OpCode opCode : oneOffsetSizePairOpCodes) {
      // offset1 + size1 == MXPX_THRESHOLD
      arguments.add(Arguments.of(opCode, MXPX_THRESHOLD.subtract(SMALL), null, SMALL, null));
      arguments.add(Arguments.of(opCode, SMALL, null, MXPX_THRESHOLD.subtract(SMALL), null));
      arguments.add(
          Arguments.of(
              opCode,
              MXP_THRESHOLD_DIVIDED_BY_TWO,
              SMALL,
              MXP_THRESHOLD_MINUS_MXP_THRESHOLD_DIVIDED_BY_TWO,
              null));
    }

    for (OpCode opCode : twoOffsetSizePairsOpCodes) {
      // offset1 + size1 == MXPX_THRESHOLD
      arguments.add(Arguments.of(opCode, MXPX_THRESHOLD.subtract(SMALL), SMALL, SMALL, SMALL));
      arguments.add(Arguments.of(opCode, SMALL, SMALL, MXPX_THRESHOLD.subtract(SMALL), SMALL));
      arguments.add(
          Arguments.of(
              opCode,
              MXP_THRESHOLD_DIVIDED_BY_TWO,
              SMALL,
              MXP_THRESHOLD_MINUS_MXP_THRESHOLD_DIVIDED_BY_TWO,
              SMALL));
      // offset2 + size2 == MXPX_THRESHOLD
      arguments.add(Arguments.of(opCode, SMALL, MXPX_THRESHOLD.subtract(SMALL), SMALL, SMALL));
      arguments.add(Arguments.of(opCode, SMALL, SMALL, SMALL, MXPX_THRESHOLD.subtract(SMALL)));
      arguments.add(
          Arguments.of(
              opCode,
              SMALL,
              MXP_THRESHOLD_DIVIDED_BY_TWO,
              SMALL,
              MXP_THRESHOLD_MINUS_MXP_THRESHOLD_DIVIDED_BY_TWO));
    }

    return arguments.stream();
  }

  static BigInteger mxpxThreshold() {
    return CANCUN_MXPX_THRESHOLD_BI;
  }

  @ParameterizedTest
  @MethodSource({
    "testCodeCopyOverflowWithOneTinyParameterSource",
    "testCodeCopyOverflowWithTwoSimilarValuesSource",
    "testCodeCopyOverflowWithTwoLargeValuesSource"
  })
  void testCodeCopyOverflow(BigInteger size, BigInteger destOffset, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .push(size)
        .push(0) // offset (arbitrary value)
        .push(destOffset) // destOffset
        .op(OpCode.CODECOPY);
    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  static Stream<Arguments> testCodeCopyOverflowWithOneTinyParameterSource() {
    List<Arguments> arguments = new ArrayList<>();
    List<BigInteger> aValues =
        List.of(BigInteger.valueOf(16), BigInteger.valueOf(17), BigInteger.valueOf(18));
    BigInteger b =
        CANCUN_MXPX_THRESHOLD_BI.subtract(BigInteger.valueOf(32)).add(BigInteger.valueOf(15));
    for (BigInteger a : aValues) {
      arguments.add(Arguments.of(a, b));
      arguments.add(Arguments.of(b, a));
    }
    return arguments.stream();
  }

  static Stream<Arguments> testCodeCopyOverflowWithTwoSimilarValuesSource() {
    List<Arguments> arguments = new ArrayList<>();
    List<BigInteger> aValues =
        List.of(
            BigInteger.TWO.pow(31),
            BigInteger.TWO.pow(31).subtract(BigInteger.valueOf(1)),
            BigInteger.TWO.pow(31).add(BigInteger.valueOf(1)));
    BigInteger b = BigInteger.TWO.pow(31);
    for (BigInteger a : aValues) {
      arguments.add(Arguments.of(a, b));
      arguments.add(Arguments.of(b, a));
    }
    return arguments.stream();
  }

  static Stream<Arguments> testCodeCopyOverflowWithTwoLargeValuesSource() {
    List<Arguments> arguments = new ArrayList<>();
    List<BigInteger> values =
        List.of(
            CANCUN_MXPX_THRESHOLD_BI,
            CANCUN_MXPX_THRESHOLD_BI.subtract(BigInteger.valueOf(1)),
            CANCUN_MXPX_THRESHOLD_BI.add(BigInteger.valueOf(1)));
    for (BigInteger a : values) {
      for (BigInteger b : values) {
        arguments.add(Arguments.of(a, b));
      }
    }
    return arguments.stream();
  }

  // Specialized tests for MCOPY
  @ParameterizedTest
  @MethodSource("inputParamsUnit")
  void McopyLight(Bytes targetOffset, Bytes sourceOffset, Bytes size, TestInfo testInfo) {
    singleMcopy(targetOffset, sourceOffset, size, testInfo);
  }

  @Tag("nightly")
  @ParameterizedTest
  @MethodSource("inputParamsNightly")
  void McopyExtensive(Bytes targetOffset, Bytes sourceOffset, Bytes size, TestInfo testInfo) {
    singleMcopy(targetOffset, sourceOffset, size, testInfo);
  }

  private static Stream<Arguments> inputs(List<Bytes32> inputsValues) {
    final List<Arguments> arguments = new ArrayList<>();
    for (Bytes32 targetOffset : inputsValues) {
      for (Bytes32 sourceOffset : inputsValues) {
        for (Bytes32 size : inputsValues) {
          arguments.add(Arguments.of(targetOffset, sourceOffset, size));
        }
      }
    }
    return arguments.stream();
  }

  private static Stream<Arguments> inputParamsNightly() {
    return inputs(inputsValuesNightly);
  }

  private static Stream<Arguments> inputParamsUnit() {
    return inputs(inputsValuesUnit);
  }

  private static final List<Bytes32> inputsValuesUnit =
      List.of(
          Bytes32.ZERO,
          Bytes32.leftPad(Bytes.ofUnsignedInt(1)),
          Bytes32.leftPad(Bytes.ofUnsignedLong(CANCUN_MXPX_THRESHOLD - 1)),
          Bytes32.leftPad(Bytes.ofUnsignedLong(CANCUN_MXPX_THRESHOLD)),
          Bytes32.repeat((byte) 0xff));

  private static final List<Bytes32> inputsValuesNightly =
      Stream.concat(
              inputsValuesUnit.stream(),
              Stream.of(
                  Bytes32.leftPad(Bytes.ofUnsignedInt(LLARGEMO)),
                  Bytes32.leftPad(Bytes.ofUnsignedInt(LLARGE)),
                  Bytes32.leftPad(Bytes.ofUnsignedInt(LLARGEPO)),
                  Bytes32.leftPad(Bytes.ofUnsignedInt(WORD_SIZE_MO)),
                  Bytes32.leftPad(Bytes.ofUnsignedInt(WORD_SIZE)),
                  Bytes32.leftPad(Bytes.ofUnsignedInt(33)),
                  Bytes32.leftPad(Bytes.ofUnsignedLong(Long.MAX_VALUE)),
                  Bytes32.leftPad(Bytes.ofUnsignedLong(CANCUN_MXPX_THRESHOLD + 1))))
          .toList();

  // Main test
  private void singleMcopy(Bytes targetOffset, Bytes sourceOffset, Bytes size, TestInfo testInfo) {
    if (!isPostCancun(chainConfig.fork)) {
      return; // MCOPY is introduced in Cancun
    }
    final Address codeOwnerAddress = Address.fromHexString("0xC0DE");
    final ToyAccount codeOwnerAccount =
        ToyAccount.builder()
            .balance(Wei.of(0))
            .nonce(1)
            .address(codeOwnerAddress)
            .code(
                Bytes.fromHexString(
                    "fb8a7292db854038692b1217d9df4482d2fcf5650e39ab0072fdbd008765ac55651e36962cb05b57fafa8b90a36bca49eb2c4017a44e6f90d74f81bcee025dab99fa957d4e0034e018cf80d1272129aa378cc98674bb44e62d9cbc0ac1835198756e3c147dd4df5ac4ce51bdde0c3c641d5f27eff5c7abeec71265a0372da839447fa7f0d5c25425900783b62966ff8313b1211218480f9dfcc0c00578a9ca86bfc4ffad37c99c97c96dc66632ae14590cb0f4bbb5ba7937c5fa83e89efe4bd9f564bbc8dd715fd70caee0c89d647fa3dd46676522c24b13f6f722e71cf40592938a085cfb7cb11a70be32b345a4168686c6df6dd8e6d9c221854aecd5851b8c4a8631dfd81733712ebe0390106d564ec331846fc8b0c19bc3bf0d0631cf6ed0165ef49f2b355b0dc1596f0bd6ba410a80a3cf2c54e1b7fa2431e21c5951816e0e93116852f38d9b3be3a7a5dd9826941792fb39d0dfd8e178ec0752741ab2ea9bb3a450d269a569b4f50b8fc32707602a8b3bad3b8aaf0645d5f61555e9c1133340ec16c8c9c254e2274081b0c2b3b453a1c2843e6e0ac91b50272a66db85400669f1fd0bb9bc0f55603972a514129561545eaa9b835cb3445f334a3a35889d9655c632886daefc44926ad53c850081da932673395eb78bcdc8b7914c35b361ae19c4b37fc210090df271a63c95d92045576b62f0bfb2dd608a31704e94dfa66624d634f5c757706b2edf2de4e7c7f8703e9745aa3ce0ba6fed4de3deb2cabbc8c5a9589189392755882ea5e3def75868ac879b7f9a52335509b7dc8e8f5dee85edcb1cf7ca446dc60ca3f23b15c338fa2a6baa0df7c9ec3d5312a8f9e453590eda7a3cee53927bad4f4b8984ec06f9fa9ad40a7d9aa86c671865851af05771a1acd336eb134e21fd5328d9497fe5c7c38aa195a6af3b0f7901a8e09de19b7dca48011b26ff0017f3ca1880a1e3c760c78c0946e1ab7c5030ec773190cffd89deb713918e521794fc9ad04414d1552b8aca294aef5f4173074b6043ed8486d420d9456273d3dc6ee06efca9c0c5362bedfec8bd98a111d06ec4e4f1f7fee52078f7fabcbadac6ef29d311d52232f8e3a538ca77a889f00b35a20955e028b8d0cdece8881c435e35719e9fd81ddc9f41f8f1723d375db5971e1eb45fbd201b561ad4e81a4a7ee8791f15d8d43744ca397bd1e75cbe9c1c49ca94733dcf5bc0a0aab0a65bb4fd22aedc431c7973c7c2bfe9be5a7c0f45e46c4e278e9d6bbdc391636ced707569df041ac1ec2c70f4650605dcd5905b302ab5208204cdb21b1f8ab1039a2e8175bde13759aebf209eec905a9262c99e05932b78cdd88934e3aea5967e57d0a8c67222dc875b608ae5d5e3d507215d2047061f01638ddcbf5a59c960c7945d2fc27644a04d402d8dcf9d2e034991d5188baef40148b71858a731ee"))
            .build();

    // First place the parameters in memory
    // Copy to targetOffset the code of codeOwnerAccount
    final Bytes FILL_MEMORY =
        BytecodeCompiler.newProgram(chainConfig)
            .push(codeOwnerAddress)
            .op(OpCode.EXTCODESIZE) // size
            .push(0) // offset
            .push(0) // targetOffset
            .push(codeOwnerAddress) // address
            .op(OpCode.EXTCODECOPY)
            .compile();

    final Bytes MLOADS =
        BytecodeCompiler.newProgram(chainConfig)
            .push(0)
            .op(MLOAD)
            .op(POP)
            .push(WORD_SIZE)
            .op(MLOAD)
            .op(POP)
            .push(2 * WORD_SIZE)
            .op(MLOAD)
            .op(POP)
            .compile();

    BytecodeRunner.of(
            Bytes.concatenate(
                FILL_MEMORY, // We fill the first 1024 bytes of memory with a non-trivial value
                pushAndMcopy(targetOffset, sourceOffset, size), // We perform the MCOPY
                MLOADS // We load the first 3 words of memory to check the result
                ))
        .run(List.of(codeOwnerAccount), chainConfig, testInfo);
  }

  private Bytes pushAndMcopy(Bytes targetOffset, Bytes sourceOffset, Bytes size) {
    return BytecodeCompiler.newProgram(chainConfig)
        .push(size)
        .push(sourceOffset)
        .push(targetOffset)
        .op(OpCode.MCOPY)
        .compile();
  }
}
