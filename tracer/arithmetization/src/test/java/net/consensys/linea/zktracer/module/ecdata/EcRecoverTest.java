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

package net.consensys.linea.zktracer.module.ecdata;

import static net.consensys.linea.zktracer.module.ecdata.EcDataOperation.SECP256K1N;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class EcRecoverTest extends TracerTestBase {

  static final EWord h =
      EWord.ofHexString("0x456e9aea5e197a1f1af7a3e85a3212fa4049a3ba34c2289b4c860fc0b0c64ef3");
  static final List<EWord> v =
      List.of(
          EWord.of(28),
          EWord.ZERO,
          EWord.of(BigInteger.ONE, BigInteger.valueOf(27)),
          EWord.of(BigInteger.ONE, BigInteger.valueOf(28)));
  static final List<EWord> r =
      List.of(
          EWord.ofHexString("0x9242685bf161793cc25603c231bc2f568eb630ea16aa137d2664ac8038825608"),
          EWord.ZERO,
          SECP256K1N,
          SECP256K1N.add(EWord.of(1)));
  static final List<EWord> s =
      List.of(
          EWord.ofHexString("0x4f8ae3bd7535248d0bd448298cc2e2071e56992d0774dc340c368ae950852ada"),
          EWord.ZERO,
          SECP256K1N,
          SECP256K1N.add(EWord.of(1)));

  @Test
  void testEcRecoverWithEmptyExt(TestInfo testInfo) {
    BytecodeRunner.of(
            Bytes.fromHexString(
                "6080604052348015600f57600080fd5b5060476001601b6001620f00007ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe609360201b60201c565b605157605060ce565b5b60006040518060400160405280600e81526020017f7a6b2d65766d206973206c6966650000000000000000000000000000000000008152509050805160208201f35b600060405186815285602082015284604082015283606082015260008084608001836001610bb8fa9150608081016040525095945050505050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052600160045260246000fdfe"))
        .run(chainConfig, testInfo);
  }

  @ParameterizedTest
  @MethodSource({"ecRecoverSource", "ecRecoverSourceForSLimits"})
  void testEcRecover(
      String description,
      EWord h,
      EWord v,
      EWord r,
      EWord s,
      Boolean expectedInternalChecksPassed,
      Boolean expectedSuccessBit,
      TestInfo testInfo) {
    testEcRecoverBody(
        description, h, v, r, s, expectedInternalChecksPassed, expectedSuccessBit, testInfo);
  }

  @Tag("nightly")
  @ParameterizedTest
  @MethodSource({"ecRecoverSourceNightly", "ecRecoverSourceForSLimits"})
  void testEcRecoverNightly(
      String description,
      EWord h,
      EWord v,
      EWord r,
      EWord s,
      Boolean expectedInternalChecksPassed,
      Boolean expectedSuccessBit,
      TestInfo testInfo) {
    testEcRecoverBody(
        description, h, v, r, s, expectedInternalChecksPassed, expectedSuccessBit, testInfo);
  }

  private void testEcRecoverBody(
      String description,
      EWord h,
      EWord v,
      EWord r,
      EWord s,
      Boolean expectedInternalChecksPassed,
      Boolean expectedSuccessBit,
      TestInfo testInfo) {
    BytecodeCompiler program =
        BytecodeCompiler.newProgram(chainConfig)
            // First place the parameters in memory
            .push(h)
            .push(0)
            .op(OpCode.MSTORE)
            .push(v) // v
            .push(0x20)
            .op(OpCode.MSTORE)
            .push(r) // r
            .push(0x40)
            .op(OpCode.MSTORE)
            .push(s) // s
            .push(0x60)
            .op(OpCode.MSTORE)
            // Do the call
            .push(32) // retSize
            .push(0x80) // retOffset
            .push(0x80) // argSize
            .push(0) // argOffset
            .push(Address.ECREC) // address
            .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
            .op(OpCode.STATICCALL);

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    final EcData ecData = bytecodeRunner.getHub().ecData();

    // Assert internalChecksPassed and successBit are what expected
    final EcDataOperation ecDataOperation = ecData.operations().get(0);
    boolean internalChecksPassed = ecDataOperation.internalChecksPassed();
    boolean successBit = ecDataOperation.successBit();

    assertEquals(expectedInternalChecksPassed, internalChecksPassed);
    if (expectedSuccessBit != null) {
      assertEquals(expectedSuccessBit, successBit);
    }

    // Check that the line count is made
    assertEquals(
        internalChecksPassed ? 1 : 0, bytecodeRunner.getHub().ecRecoverEffectiveCall().lineCount());
  }

  private static Stream<Arguments> ecRecoverSource() {
    List<Arguments> arguments = new ArrayList<>();

    // Test cases where ICP = successBit = 1
    arguments.add(
        ecRecoverArgumentsFromStrings(
            "[ICP = 1, successBit = 1]",
            "0x279d94621558f755796898fc4bd36b6d407cae77537865afe523b79c74cc680b",
            "0x1b",
            "0xc2ff96feed8749a5ad1c0714f950b5ac939d8acedbedcbc2949614ab8af06312",
            "0x1feecd50adc6273fdd5d11c6da18c8cfe14e2787f5a90af7c7c1328e7d0a2c42",
            true,
            true));

    arguments.add(
        ecRecoverArgumentsFromStrings(
            "[ICP = 1, successBit = 1]",
            "0x4be146e06cc1b37342b6b7b1fa8542ae58a62103b8af0f7d58f8a1ffffcf7914",
            "0x1b",
            "0xa7b0f504b652b3a621921c78c587fdf80a3ab590e22c304b0b0930e90c4e081d",
            "0x5428459ef7e6bd079fbbb7c6fd95cc6c7fe68c93ed4ae75cee36810e79e8a0e5",
            true,
            true));

    arguments.add(
        ecRecoverArgumentsFromStrings(
            "[ICP = 1, successBit = 1]",
            "0xca3e75570aea0e3dd8e7a9d38c2efa866f5ee2b18bf527a0f4e3248b7c7cf376",
            "0x1c",
            "0xf1136900c2cd16eacc676f2c7b70f3dfec13fd16a426aab4eda5d8047c30a9e9",
            "0x4dad8f009ebe31bdc38133bc5fa60e9dca59d0366bd90e2ef12b465982c696aa",
            true,
            true));

    arguments.add(
        ecRecoverArgumentsFromStrings(
            "[ICP = 1, successBit = 1]",
            "0x9a3fa82837622a34408888b40af937f21f4e6d051f04326d3d7717848c434448",
            "0x1b",
            "0x52a734f01d14d161795ba3b38ce329eba468e109b4f2e330af671649ffef4e0e",
            "0xe3e2a22b830edf61554ab6c18c7efb9e37e1953c913784db0ef74e1e07c227d3",
            true,
            true));

    // Test cases where ICP = 1 but successBit = 0
    // Failing reason QNR
    arguments.add(
        ecRecoverArgumentsFromStrings(
            "[ICP = 1, successBit = 0 due to QNR]",
            "0x94f66d57fb0a3854a44d94956447e01f8b3f09845860f18856e792c821359162",
            "0x1c",
            "0x000000000000000000000000000000014551231950b75fc4402da1722fc9baed",
            "0x44c819d6b971e456562fefc2408536bdfd9567ee1c6c7dd2a7076625953a1859",
            true,
            false));

    // Failing reason INFINITY
    arguments.add(
        ecRecoverArgumentsFromStrings(
            "[ICP = 1, successBit = 0 due to INFINITY]",
            "0xd33cfae367f4f7413985ff82dc7db3ffbf7a027fb5dad7097b4a15cc85ab6580",
            "0x1c",
            "0xa12b54d413c4ffaaecd59468de6a7d414d2fa7f2ba700d8e0753ca226410c806",
            "0xe9956ef412dceeda0016fe0edfc4746452a8f4d02f21e28cfa6019ee1a8976e8",
            true,
            false));

    // Failing reason INFINITY
    arguments.add(
        ecRecoverArgumentsFromStrings(
            "[ICP = 1, successBit = 0 due to INFINITY]",
            "0x6ec17edf5cecd83ed50c08adfeba8146f69769231f4b7903eba38c2e7e98e173",
            "0x1b",
            "0xaeb8ffe3655e07edd6bde0ab79edd92d4e7a155385c3d8c8ca117bfd13633516",
            "0x4da31701c798fe3078ee9de6e4d892242e235dc078df76b15a9ad82137c6250e",
            true,
            false));

    // Test cases where ICP = successBit = 0
    arguments.add(
        Arguments.of("[ICP = 0, successBit = 0]", h, v.get(1), r.get(1), s.get(1), false, false));

    arguments.add(
        Arguments.of("[ICP = 0, successBit = 0]", h, v.get(2), r.get(2), s.get(2), false, false));

    return arguments.stream();
  }

  private static Stream<Arguments> ecRecoverSourceNightly() {
    List<Arguments> arguments = new ArrayList<>();

    // Test cases where ICP = successBit = 1 (first one) or ICP = successBit = 0 (all the others)
    for (int i = 0; i < v.size(); i++) {
      for (int j = 0; j < r.size(); j++) {
        for (int k = 0; k < s.size(); k++) {
          arguments.add(
              Arguments.of(
                  i + j + k == 0 ? "[ICP = 1, successBit = 1]" : "[ICP = 0, successBit = 0]",
                  h,
                  v.get(i),
                  r.get(j),
                  s.get(k),
                  i + j + k == 0,
                  i + j + k == 0));
        }
      }
    }

    return arguments.stream();
  }

  /**
   * The test cases generated in this method are meant to explore the corner cases of the 's'
   * parameter. We do not require ECRECOVER to succeed, as such the other parameters h and v are
   * irrelevant, albeit well-formed.
   */
  private static Stream<Arguments> ecRecoverSourceForSLimits() {
    EWord h =
        EWord.ofHexString("0x456e9aea5e197a1f1af7a3e85a3212fa4049a3ba34c2289b4c860fc0b0c64ef3");
    EWord v = EWord.of(28);
    EWord r =
        EWord.ofHexString("0x9242685bf161793cc25603c231bc2f568eb630ea16aa137d2664ac8038825608");
    List<EWord> s =
        List.of(
            ((SECP256K1N.subtract(1)).divide(2)).subtract(1), // universally accepted
            ((SECP256K1N.subtract(1)).divide(2)), // universally accepted
            ((SECP256K1N.subtract(1)).divide(2)).add(1), // universally accepted
            ((SECP256K1N.subtract(1)).divide(2))
                .add(2), // rejected for transactions, acceptable in ECRECOVER
            SECP256K1N.subtract(2), // acceptable in ECRECOVER
            SECP256K1N.subtract(1), // acceptable in ECRECOVER
            SECP256K1N, // universally rejected
            SECP256K1N.add(1)); // universally rejected

    List<Arguments> arguments = new ArrayList<>();

    for (int i = 0; i < s.size(); i++) {
      arguments.add(
          Arguments.of(
              s.get(i).lessThan(SECP256K1N) ? "[ICP = 1]" : "[ICP = 0]",
              h,
              v,
              r,
              s.get(i),
              s.get(i).lessThan(SECP256K1N),
              null)); // The assertion over successBit is not relevant here
    }

    return arguments.stream();
  }

  private static Arguments ecRecoverArgumentsFromStrings(
      String description,
      String h,
      String v,
      String r,
      String s,
      boolean expectedInternalChecksPassed,
      boolean expectedSuccessBit) {
    return Arguments.of(description, h, v, r, s, expectedInternalChecksPassed, expectedSuccessBit);
  }

  @Test
  void testEcRecoverInternalChecksFailSingleCase(TestInfo testInfo) {
    BytecodeCompiler program =
        BytecodeCompiler.newProgram(chainConfig)
            // First place the parameters in memory
            .push("1111111111111111111111111111111111111111111111111111111111111111") // h
            .push(0)
            .op(OpCode.MSTORE)
            .push("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff") // v
            .push(0x20)
            .op(OpCode.MSTORE)
            .push("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc") // r
            .push(0x40)
            .op(OpCode.MSTORE)
            .push("cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc") // s
            .push(0x60)
            .op(OpCode.MSTORE)
            // Do the call
            .push(32) // retSize
            .push(0x80) // retOffset
            .push(0x80) // argSize
            .push(0) // argOffset
            .push(1) // address
            .push(3000)
            .op(OpCode.STATICCALL);

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);

    final EcData ecData = bytecodeRunner.getHub().ecData();

    // Assert internalChecksPassed and successBit are what expected
    final EcDataOperation ecDataOperation = ecData.operations().get(0);
    boolean internalChecksPassed = ecDataOperation.internalChecksPassed();
    boolean successBit = ecDataOperation.successBit();

    assertFalse(internalChecksPassed);
    assertFalse(successBit);

    // Check that the line count is made
    assertEquals(0, bytecodeRunner.getHub().ecRecoverEffectiveCall().lineCount());
  }
}
