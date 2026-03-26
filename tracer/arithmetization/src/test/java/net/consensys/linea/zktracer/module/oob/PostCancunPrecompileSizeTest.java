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

import static java.util.Map.entry;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_SIZE___FP2_TO_G2;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_SIZE___FP_TO_G1;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_SIZE___G1_ADD;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_SIZE___G2_ADD;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_SIZE___POINT_EVALUATION;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G1_MSM;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G2_MSM;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G1_ADD;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G1_MSM;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G2_ADD;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G2_MSM;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_MAP_FP2_TO_G2;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_MAP_FP_TO_G1;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_PAIRING_CHECK;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_P256_VERIFY;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_POINT_EVALUATION;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class PostCancunPrecompileSizeTest extends TracerTestBase {

  @Tag("nightly")
  @ParameterizedTest
  @MethodSource("postCancunPrecompileSizeTestSource")
  void postCancunPrecompileSizeTest(
      PrecompileScenarioFragment.PrecompileFlag precompileFlag, Integer size, TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    final Address codeOwnerAddress = Address.fromHexString("0xC0DE");
    final ToyAccount codeOwnerAccount =
        ToyAccount.builder()
            .balance(Wei.of(0))
            .nonce(1)
            .address(codeOwnerAddress)
            .code(Bytes.fromHexString("11".repeat(size)))
            .build();

    // First place the parameters in memory
    // Copy to targetOffset the code of codeOwnerAccount
    program
        .push(codeOwnerAddress.getBytes())
        .op(OpCode.EXTCODESIZE) // size
        .push(0) // offset
        .push(0) // targetOffset
        .push(codeOwnerAddress.getBytes()) // address
        .op(OpCode.EXTCODECOPY);

    // Do the call
    program
        .push(0) // retSize
        .push(0) // retOffset, note that we are really interested in the return data
        .push(size) // argSize
        .push(0) // argOffset
        .push(precompileFlag.getAddress().getBytes()) // address
        .push(Bytes.fromHexStringLenient("0xFFFFFFFF")) // gas
        .op(OpCode.STATICCALL);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(List.of(codeOwnerAccount), chainConfig, testInfo);
  }

  private static Stream<Arguments> postCancunPrecompileSizeTestSource() {
    final Map<PrecompileScenarioFragment.PrecompileFlag, Integer>
        FIXED_SIZE_PRECOMPILE_ADDRESS_TO_SIZE =
            Map.ofEntries(
                entry(PRC_POINT_EVALUATION, PRECOMPILE_CALL_DATA_SIZE___POINT_EVALUATION),
                entry(PRC_BLS_G1_ADD, PRECOMPILE_CALL_DATA_SIZE___G1_ADD),
                entry(PRC_BLS_G2_ADD, PRECOMPILE_CALL_DATA_SIZE___G2_ADD),
                entry(PRC_BLS_MAP_FP_TO_G1, PRECOMPILE_CALL_DATA_SIZE___FP_TO_G1),
                entry(PRC_BLS_MAP_FP2_TO_G2, PRECOMPILE_CALL_DATA_SIZE___FP2_TO_G2),
                entry(PRC_P256_VERIFY, PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY));

    final Map<PrecompileScenarioFragment.PrecompileFlag, Integer>
        VARIABLE_SIZE_PRECOMPILE_ADDRESS_TO_UNIT =
            Map.ofEntries(
                entry(PRC_BLS_G1_MSM, PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G1_MSM),
                entry(PRC_BLS_G2_MSM, PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G2_MSM),
                entry(PRC_BLS_PAIRING_CHECK, PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK));

    List<Arguments> arguments = new ArrayList<>();

    for (PrecompileScenarioFragment.PrecompileFlag precompileFlag :
        FIXED_SIZE_PRECOMPILE_ADDRESS_TO_SIZE.keySet()) {
      arguments.add(Arguments.of(precompileFlag, 0));
      arguments.add(Arguments.of(precompileFlag, 1));
      int size = FIXED_SIZE_PRECOMPILE_ADDRESS_TO_SIZE.get(precompileFlag);
      for (int cornerCase = -1; cornerCase <= 1; cornerCase++) {
        arguments.add(Arguments.of(precompileFlag, size + cornerCase));
      }
    }

    for (PrecompileScenarioFragment.PrecompileFlag precompileFlag :
        VARIABLE_SIZE_PRECOMPILE_ADDRESS_TO_UNIT.keySet()) {
      int unit = VARIABLE_SIZE_PRECOMPILE_ADDRESS_TO_UNIT.get(precompileFlag);
      arguments.add(Arguments.of(precompileFlag, 0));
      arguments.add(Arguments.of(precompileFlag, 1));
      arguments.add(Arguments.of(precompileFlag, 256 * unit));
      // We test call data sizes (130) that go slightly beyond the max discount of the BLS reference
      // table (128)
      for (int numberOfUnits = 1; numberOfUnits <= 130; numberOfUnits++) {
        for (int cornerCase = -1; cornerCase <= 1; cornerCase++) {
          arguments.add(Arguments.of(precompileFlag, numberOfUnits * unit + cornerCase));
        }
      }
    }

    return arguments.stream();
  }
}
