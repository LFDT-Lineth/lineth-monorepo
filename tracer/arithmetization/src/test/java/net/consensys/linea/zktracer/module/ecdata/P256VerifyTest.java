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

import static net.consensys.linea.zktracer.Fork.isPostOsaka;
import static net.consensys.linea.zktracer.Trace.EIP_7825_TRANSACTION_GAS_LIMIT_CAP;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_RETURN_DATA_SIZE___P256_VERIFY;
import static net.consensys.linea.zktracer.module.ecdata.EcDataOperation.P_R1;
import static net.consensys.linea.zktracer.module.ecdata.EcDataOperation.SECP256R1N;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.IOException;
import java.io.InputStream;
import java.time.LocalDate;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.Random;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.fixedSizeFixedGasCost.P256VerifyOobCall;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class P256VerifyTest extends TracerTestBase {

  @ParameterizedTest
  @MethodSource("p256VerifySource")
  void testP256Verify(String inputAsAsString, String expectedAsString, TestInfo testInfo) {
    BytecodeRunner bytecodeRunner = testP256VerifyBody(inputAsAsString, testInfo);
    if (isPostOsaka(fork)) {
      assertEquals(
          Bytes.fromHexString(expectedAsString),
          bytecodeRunner.getHub().ecData().ecDataOperation().returnData());
    }
  }

  @Tag("nightly")
  @ParameterizedTest
  @MethodSource("p256VerifySourceNightly")
  void testP256VerifyNightly(String inputAsAsString, String expectedAsString, TestInfo testInfo) {
    BytecodeRunner bytecodeRunner = testP256VerifyBody(inputAsAsString, testInfo);
    if (isPostOsaka(fork)) {
      assertEquals(
          Bytes.fromHexString(expectedAsString),
          bytecodeRunner.getHub().ecData().ecDataOperation().returnData());
    }
  }

  private BytecodeRunner testP256VerifyBody(String inputAsAsString, TestInfo testInfo) {
    return testP256VerifyBody(inputAsAsString, Bytes.EMPTY, testInfo);
  }

  private BytecodeRunner testP256VerifyBody(
      String inputAsAsString, Bytes trailingProgram, TestInfo testInfo) {
    final Bytes input = Bytes.fromHexString(inputAsAsString);

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
        .push(PRECOMPILE_RETURN_DATA_SIZE___P256_VERIFY) // retSize
        .push(input.size()) // retOffset
        .push(input.size()) // argSize
        .push(0) // argOffset
        .push(Address.P256_VERIFY) // address
        .push(EIP_7825_TRANSACTION_GAS_LIMIT_CAP) // gas
        .op(OpCode.STATICCALL);
    if (!trailingProgram.isEmpty()) {
      program.immediate(trailingProgram);
    }
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(List.of(codeOwnerAccount), chainConfig, testInfo);
    return bytecodeRunner;
  }

  private static Stream<Arguments> p256VerifySource() throws IOException {
    List<Arguments> arguments = new ArrayList<>(p256VerifySourceNightly().toList());
    Collections.shuffle(arguments, new Random(LocalDate.now().toEpochDay()));
    return arguments.stream().limit(arguments.size() / 20); // Execute 5 % of the tests
  }

  private static Stream<Arguments> p256VerifySourceNightly() throws IOException {
    // Read json
    // Test vector comes from https://eips.ethereum.org/assets/eip-7951/test-vectors.json
    InputStream inputStream =
        P256VerifyTest.class.getResourceAsStream("/p256_verify_test_vectors.json");
    ObjectMapper mapper = new ObjectMapper();
    JsonNode root = mapper.readTree(inputStream);
    // Fill list of arguments
    List<Arguments> arguments = new ArrayList<>();
    for (JsonNode node : root) {
      String input = node.get("Input").asText();
      String expected = node.get("Expected").asText();
      arguments.add(Arguments.of(input, expected));
    }
    return arguments.stream();
  }

  final String h = "00".repeat(32);

  static final List<String> rs =
      Stream.of(EWord.ZERO, EWord.ONE, SECP256R1N.subtract(1), SECP256R1N, SECP256R1N.add(1))
          .map(e -> e.toHexString().substring(2))
          .toList();
  static final List<String> qXqY =
      Stream.of(EWord.ZERO, EWord.ONE, P_R1.subtract(1), P_R1, P_R1.add(1))
          .map(e -> e.toHexString().substring(2))
          .toList();

  @Tag("nightly")
  @ParameterizedTest
  @MethodSource("testP256VerifyExploreEdgeCasesSource")
  void testP256VerifyExploreEdgeCases(String r, String s, String qX, String qY, TestInfo testInfo) {
    testP256VerifyBody(h + r + s + qX + qY, testInfo);
  }

  private static Stream<Arguments> testP256VerifyExploreEdgeCasesSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (String r : rs) {
      for (String s : rs) {
        for (String qX : qXqY) {
          for (String qY : qXqY) {
            arguments.add(Arguments.of(r, s, qX, qY));
          }
        }
      }
    }
    return arguments.stream();
  }

  // Edge cases with checks over return data
  @Test
  void testInternalChecksFailP256Verify(TestInfo testInfo) {
    Bytes trailingProgram =
        BytecodeCompiler.newProgram(chainConfig)
            .op(OpCode.RETURNDATASIZE)
            .op(OpCode.JUMPDEST, 32) // TODO: temporary workaround for go-corset issue
            .compile();
    BytecodeRunner bytecodeRunner =
        testP256VerifyBody(
            h + rs.getLast() + rs.getLast() + qXqY.getLast() + qXqY.getLast(),
            trailingProgram,
            testInfo);

    if (isPostOsaka(fork)) {
      final Bytes callSuccess = bytecodeRunner.getHub().currentFrame().frame().getStackItem(1);
      assertFalse(callSuccess.isZero());

      P256VerifyOobCall p256VerifyOobCall =
          (P256VerifyOobCall)
              bytecodeRunner.getHub().oob().operations().stream().toList().getLast();
      assertTrue(p256VerifyOobCall.isHubSuccess());

      EcDataOperation ecDataOperation = bytecodeRunner.getHub().ecData().ecDataOperation();
      assertFalse(ecDataOperation.internalChecksPassed());

      final Bytes returnDataSize = bytecodeRunner.getHub().currentFrame().frame().getStackItem(0);
      assertTrue(returnDataSize.isZero());
    }
  }

  @Test
  void testInternalChecksSucceedButSignatureVerificationFailsP256Verify(TestInfo testInfo) {
    Bytes trailingProgram =
        BytecodeCompiler.newProgram(chainConfig)
            .op(OpCode.RETURNDATASIZE)
            .op(OpCode.JUMPDEST, 32) // TODO: temporary workaround for go-corset issue
            .compile();
    BytecodeRunner bytecodeRunner =
        testP256VerifyBody(
            "bb5a52f42f9c9261ed4361f59422a1e30036e7c32b270c8807a419feca605023d45c5740946b2a147f59262ee6f5bc90bd01ed280528b62b3aed5fc93f06f739b329f479a2bbd0a5c384ee1493b1f5186a87139cac5df4087c134b49156847db2927b10512bae3eddcfe467828128bad2903269919f7086069c8c4df6c732838c7787964eaac00e5921fb1498a60f4606766b3d9685001558d1a974e7341513e",
            trailingProgram,
            testInfo);

    if (isPostOsaka(fork)) {
      final Bytes callSuccess = bytecodeRunner.getHub().currentFrame().frame().getStackItem(1);
      assertFalse(callSuccess.isZero());

      P256VerifyOobCall p256VerifyOobCall =
          (P256VerifyOobCall)
              bytecodeRunner.getHub().oob().operations().stream().toList().getLast();
      assertTrue(p256VerifyOobCall.isHubSuccess());

      EcDataOperation ecDataOperation = bytecodeRunner.getHub().ecData().ecDataOperation();
      assertTrue(ecDataOperation.internalChecksPassed());

      final Bytes returnDataSize = bytecodeRunner.getHub().currentFrame().frame().getStackItem(0);
      assertTrue(returnDataSize.isZero());
    }
  }

  @Test
  void validP256Verify(TestInfo testInfo) {
    Bytes trailingProgram =
        BytecodeCompiler.newProgram(chainConfig)
            .op(OpCode.RETURNDATASIZE)
            .push(PRECOMPILE_RETURN_DATA_SIZE___P256_VERIFY)
            .push(0)
            .push(0xff)
            .op(OpCode.RETURNDATACOPY)
            .op(OpCode.JUMPDEST, 32) // TODO: temporary workaround for go-corset issue
            .compile();
    // input from p256_verify_test_vectors.json
    BytecodeRunner bytecodeRunner =
        testP256VerifyBody(
            "bb5a52f42f9c9261ed4361f59422a1e30036e7c32b270c8807a419feca6050232ba3a8be6b94d5ec80a6d9d1190a436effe50d85a1eee859b8cc6af9bd5c2e184cd60b855d442f5b3c7b11eb6c4e0ae7525fe710fab9aa7c77a67f79e6fadd762927b10512bae3eddcfe467828128bad2903269919f7086069c8c4df6c732838c7787964eaac00e5921fb1498a60f4606766b3d9685001558d1a974e7341513e",
            trailingProgram,
            testInfo);

    if (isPostOsaka(fork)) {
      final Bytes callSuccess = bytecodeRunner.getHub().currentFrame().frame().getStackItem(1);
      assertFalse(callSuccess.isZero());

      P256VerifyOobCall p256VerifyOobCall =
          (P256VerifyOobCall) bytecodeRunner.getHub().oob().operations().stream().toList().get(1);
      assertTrue(p256VerifyOobCall.isHubSuccess());

      EcDataOperation ecDataOperation = bytecodeRunner.getHub().ecData().ecDataOperation();
      assertTrue(ecDataOperation.internalChecksPassed());

      final Bytes returnDataSize = bytecodeRunner.getHub().currentFrame().frame().getStackItem(0);
      assertFalse(returnDataSize.isZero());

      assertEquals(
          Bytes.fromHexString("0000000000000000000000000000000000000000000000000000000000000001"),
          ecDataOperation.returnData());
    }
  }
}
