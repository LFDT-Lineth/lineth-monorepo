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

package net.consensys.linea.zktracer.instructionprocessing.callTests.sixtyThreeSixtyFourthsPrecompiles;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Fork.*;
import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpPricingOobCall.computeExponentLog;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLAKE2F;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G1_ADD;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G1_MSM;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G2_ADD;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G2_MSM;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_MAP_FP2_TO_G2;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_MAP_FP_TO_G1;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_PAIRING_CHECK;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_ECADD;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_ECMUL;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_ECPAIRING;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_ECRECOVER;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_IDENTITY;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_MODEXP;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_P256_VERIFY;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_POINT_EVALUATION;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_RIPEMD_160;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_SHA2_256;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.OUT_OF_GAS_EXCEPTION;
import static net.consensys.linea.zktracer.opcode.OpCode.CALL;
import static net.consensys.linea.zktracer.opcode.OpCode.MLOAD;
import static net.consensys.linea.zktracer.opcode.OpCode.POP;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.generateModexpInput;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.getBLAKE2FCost;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.getBlsG1AddCost;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.getBlsG2AddCost;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.getECADDCost;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.getMODEXPCost;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.getPrecompileCost;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.prepareBlake2F;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.writeInMemoryByteCodeOfCodeOwner;
import static net.consensys.linea.zktracer.types.AddressUtils.isBlsPrecompile;
import static org.junit.jupiter.api.Assertions.assertNotEquals;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.evm.precompile.KZGPointEvalPrecompiledContract;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

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

/**
 * <b>Note.</b> {@link #preCallProgramGasMap} In case <b>summonTargetAddressIntoExistence ≡
 * true</b>, an initial, value bearing CALL is made to the precompile, which summons it into
 * existence. For <b>POINT_EVALUATION</b> and <b>BLS_XXX</b>. This initial call is done via {@link
 * #successfullySummonIntoExistence}.
 *
 * <p>Note: there is no
 *
 * <ul>
 *   <li>2nd call (for summonTargetAddressIntoExistence ≡ <b>true</b>) or
 *   <li>1st call (for summonTargetAddressIntoExistence ≡ <b>false</b>)
 * </ul>
 *
 * call made to the precompile, as pushCallArguments call data size cds ≡ 0 would make POINT_EVAL /
 * BLS calls fail and consume 63/64-ths of the frame's gas IF A
 */
@ExtendWith(UnitTestWatcher.class)
public class SixtyThreeSixtyFourthsPrecompileTests extends TracerTestBase {

  /*
  Cases to cover:

  * value = 0
  If precompileGasCost >= 2300 then we are interested in:
   * value = 1, targetAddressExists = false
   * value = 1, targetAddressExists = true

  Note: BLAKE2F and MODEXP requires a non-zero non-trivial input to have a cost greater than 2300.
  Other precompiles only require a proper call data size at most.

  See https://github.com/Consensys/linea-tracer/issues/1153 for additional documentation.
  */

  static final Bytes INFINITE_GAS = Bytes.fromHexString("ff".repeat(32));

  // BLAKE2F specific parameters
  static final int rLeadingByte = 0x09;
  static final int r = rLeadingByte << 8;

  // MODEXP specific parameters
  static final int bbs = 2;
  static final int ebs = 6;
  static final int mbs = 128;
  static final Bytes modexpInput = generateModexpInput(bbs, mbs, ebs);
  static final Bytes pointEvaluationInput =
      Bytes.fromHexString(
          "010657f37554c781402a22917dee2f75def7ab966d7b770905398eba3c444014"
              + "0000000000000000000000000000000000000000000000000000000000000000"
              + "0000000000000000000000000000000000000000000000000000000000000000"
              + "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"
              + "c00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000");
  static final int multiplier = (forkPredatesOsaka(fork)) ? 8 : 16;
  static final int exponentLog =
      computeExponentLog(modexpInput, multiplier, 96 + bbs + ebs + mbs, bbs, ebs);
  static final Address modexpInputAsByteCodeOwnerAddress = Address.fromHexString("0xC0DE05");
  static final Address pointEvaluationInputAsByteCodeOwnerAddress =
      Address.fromHexString("0xC0DE0a");
  // modexpInputAsByteCodeOwnerAccount owns the bytecode that will be given as input to MODEXP
  // through EXTCODECOPY
  static final ToyAccount modexpInputAsByteCodeOwnerAccount =
      ToyAccount.builder()
          .balance(Wei.of(0))
          .nonce(1)
          .address(modexpInputAsByteCodeOwnerAddress)
          .code(modexpInput)
          .build();
  static final ToyAccount pointEvaluationInputAsByteCodeOwnerAccount =
      ToyAccount.builder()
          .balance(Wei.of(0))
          .nonce(1)
          .address(pointEvaluationInputAsByteCodeOwnerAddress)
          .code(pointEvaluationInput)
          .build();

  static final List<ToyAccount> additionalAccounts =
      List.of(modexpInputAsByteCodeOwnerAccount, pointEvaluationInputAsByteCodeOwnerAccount);

  // Cost of preCallProgram in different scenarios:
  // (address, transfersValue) -> gasCost

  static final Map<PrecompileScenarioFragment.PrecompileFlag, Map<Boolean, Long>>
      preCallProgramGasMap =
          Stream.of(
                  PRC_ECRECOVER,
                  PRC_SHA2_256,
                  PRC_RIPEMD_160,
                  PRC_IDENTITY,
                  PRC_MODEXP,
                  PRC_ECADD,
                  PRC_ECMUL,
                  PRC_ECPAIRING,
                  PRC_BLAKE2F,
                  PRC_POINT_EVALUATION,
                  PRC_BLS_G1_ADD,
                  PRC_BLS_G1_MSM,
                  PRC_BLS_G2_ADD,
                  PRC_BLS_G2_MSM,
                  PRC_BLS_PAIRING_CHECK,
                  PRC_BLS_MAP_FP_TO_G1,
                  PRC_BLS_MAP_FP2_TO_G2,
                  PRC_P256_VERIFY)
              .collect(
                  Collectors.toMap(
                      precompileFlag -> precompileFlag,
                      precompileFlag -> {
                        if (precompileFlag == PRC_POINT_EVALUATION)
                          KZGPointEvalPrecompiledContract.init();
                        final long gasCostTargetPrecompileDoesExist =
                            BytecodeRunner.of(preCallProgram(precompileFlag, false, true, 0))
                                .runOnlyForGasCost(
                                    (precompileFlag == PRC_MODEXP
                                            || precompileFlag == PRC_POINT_EVALUATION)
                                        ? additionalAccounts
                                        : List.of(),
                                    chainConfig,
                                    null);
                        final long gasCostTargetPrecompileDoesNotExist =
                            BytecodeRunner.of(preCallProgram(precompileFlag, false, false, 0))
                                .runOnlyForGasCost(
                                    (precompileFlag == PRC_MODEXP
                                            || precompileFlag == PRC_POINT_EVALUATION)
                                        ? additionalAccounts
                                        : List.of(),
                                    chainConfig,
                                    null);
                        return new HashMap<>() {
                          {
                            put(false, gasCostTargetPrecompileDoesNotExist);
                            put(true, gasCostTargetPrecompileDoesExist);
                          }
                        };
                      }));

  // Note: transferValue = false and cds = 0 as we are interested only in the cost of the
  // corresponding PUSHes here

  /**
   * Parameterized test for the ECADD, BLS_G1_ADD, BLS_G2_ADD precompiles, that have a fixed cost <
   * 2300.
   *
   * @param gasLimit the gas limit for the transaction. It is either as much as needed for the
   *     precompile call or slightly less.
   * @param insufficientGasForPrecompileExpected flag indicating if insufficient gas for precompile
   *     is expected.
   */
  @ParameterizedTest
  @MethodSource("fixedCostAddTestSource")
  void fixedCostAddTest(
      PrecompileScenarioFragment.PrecompileFlag precompileFlag,
      long gasLimit,
      boolean insufficientGasForPrecompileExpected,
      TestInfo testInfo) {
    // Whenever transferValue = true, gas is enough
    // so we only test the case in which transferValue = false

    final BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    program.immediate(preCallProgram(precompileFlag, false, false, 0)).op(CALL);

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(program);
    bytecodeRunner.run(gasLimit, chainConfig, testInfo);

    if (precompileFlag == PRC_ECADD || isPostPrague(fork)) {
      assertNotEquals(
          OUT_OF_GAS_EXCEPTION,
          bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
    }
  }

  static Stream<Arguments> fixedCostAddTestSource() {
    List<Arguments> arguments = new ArrayList<>();
    Map<PrecompileScenarioFragment.PrecompileFlag, Integer> precompileFlagToTargetCalleeGas =
        new HashMap<>() {
          {
            put(PRC_ECADD, getECADDCost());
            put(PRC_BLS_G1_ADD, getBlsG1AddCost());
            put(PRC_BLS_G2_ADD, getBlsG2AddCost());
          }
        };
    for (PrecompileScenarioFragment.PrecompileFlag precompileFlag :
        precompileFlagToTargetCalleeGas.keySet()) {
      for (int cornerCase : List.of(0, -1)) {
        final long gasLimit =
            getGasLimit(
                precompileFlagToTargetCalleeGas.get(precompileFlag) + cornerCase,
                false,
                false,
                preCallProgramGasMap.get(precompileFlag).get(false));
        arguments.add(Arguments.of(precompileFlag, gasLimit, cornerCase == -1));
      }
    }
    return arguments.stream();
  }

  /**
   * Parameterized test for precompile calls where the cost is greater than or equal to the stipend
   * (every precompile except ECADD, BLS_G1_ADD, BLS_G2_ADD, as long as cds and inputs are properly
   * selected).
   *
   * @param precompileFlag the precompile flag of the precompile contract.
   * @param gasLimit the gas limit for the transaction. It is either as much as needed for the
   *     precompile call or slightly less.
   * @param insufficientGasForPrecompileExpected flag indicating if insufficient gas for precompile
   *     call is expected.
   * @param transfersValue flag indicating if the call to the precompile transfers value.
   * @param targetAddressExists flag indicating if the precompile target address exists at the
   *     moment of the final call.
   * @param cds the call data size.
   */
  @ParameterizedTest
  @MethodSource("costGEQStipendTestSource")
  void costGEQStipendTest(
      PrecompileScenarioFragment.PrecompileFlag precompileFlag,
      long gasLimit,
      boolean insufficientGasForPrecompileExpected,
      boolean transfersValue,
      boolean targetAddressExists,
      int cds,
      TestInfo testInfo) {
    final BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program
        .immediate(preCallProgram(precompileFlag, transfersValue, targetAddressExists, cds))
        .op(CALL);

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(program);
    bytecodeRunner.run(
        gasLimit,
        precompileFlag == PRC_MODEXP || precompileFlag == PRC_POINT_EVALUATION
            ? additionalAccounts
            : List.of(),
        chainConfig,
        testInfo);

    if (!isBlsPrecompile(precompileFlag.getAddress())
        || (precompileFlag == PRC_POINT_EVALUATION && isPostCancun(fork))
        || (isBlsPrecompile(precompileFlag.getAddress()) && isPostPrague(fork))) {
      assertNotEquals(
          OUT_OF_GAS_EXCEPTION,
          bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
    }
  }

  static Stream<Arguments> costGEQStipendTestSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (PrecompileScenarioFragment.PrecompileFlag precompileFlag :
        List.of(
            PRC_ECRECOVER,
            PRC_SHA2_256,
            PRC_RIPEMD_160,
            PRC_IDENTITY,
            PRC_MODEXP,
            PRC_ECMUL,
            PRC_ECPAIRING,
            PRC_BLAKE2F,
            PRC_POINT_EVALUATION,
            PRC_BLS_G1_MSM,
            PRC_BLS_G2_MSM,
            PRC_BLS_PAIRING_CHECK,
            PRC_BLS_MAP_FP_TO_G1,
            PRC_BLS_MAP_FP2_TO_G2,
            PRC_P256_VERIFY)) {
      final int cds = getMeaningfulCallDataSize(precompileFlag);
      final long targetCalleeGas =
          precompileFlag == PRC_BLAKE2F
              ? getBLAKE2FCost(r)
              : precompileFlag == PRC_MODEXP
                  ? getMODEXPCost(bbs, mbs, exponentLog)
                  : getPrecompileCost(precompileFlag.getAddress(), cds);
      for (int cornerCase : List.of(0, -1)) {
        for (boolean transfersValue : List.of(true, false)) {
          for (boolean targetAddressExists : List.of(true, false)) {
            if (!transfersValue && targetAddressExists) {
              continue; // no need to test this case
            }
            final long gasLimit =
                getGasLimit(
                    targetCalleeGas + cornerCase,
                    transfersValue,
                    targetAddressExists,
                    preCallProgramGasMap.get(precompileFlag).get(targetAddressExists));
            arguments.add(
                Arguments.of(
                    precompileFlag,
                    gasLimit,
                    cornerCase == -1,
                    transfersValue,
                    targetAddressExists,
                    cds));
          }
        }
      }
    }
    return arguments.stream();
  }

  // Support methods
  static Bytes preCallProgram(
      PrecompileScenarioFragment.PrecompileFlag precompileFlag,
      boolean transfersValue,
      boolean summonTargetAddressIntoExistence,
      int cds) {
    return BytecodeCompiler.newProgram(chainConfig)
        .immediate(expandMemoryTo2048Words())
        // Filling memory for preliminary call to summon precompile into existence in the case of
        // PRC_POINT_EVALUATION
        .immediate(
            precompileFlag == PRC_POINT_EVALUATION
                ? writeInMemoryByteCodeOfCodeOwner(pointEvaluationInputAsByteCodeOwnerAddress, 0)
                : Bytes.EMPTY)
        .immediate(
            summonTargetAddressIntoExistence
                ? successfullySummonIntoExistence(precompileFlag)
                : Bytes.EMPTY)
        // Filling memory for actual call happening in the test
        .immediate(
            switch (precompileFlag) {
              case PRC_MODEXP ->
                  writeInMemoryByteCodeOfCodeOwner(modexpInputAsByteCodeOwnerAddress, 0);
              case PRC_BLAKE2F -> prepareBlake2F(rLeadingByte, 2);
              case PRC_POINT_EVALUATION ->
                  writeInMemoryByteCodeOfCodeOwner(pointEvaluationInputAsByteCodeOwnerAddress, 0);
              default -> Bytes.EMPTY;
            })
        // TODO: we should be able to re-use the input of the first call if we do not overwrite it
        .immediate(pushCallArguments(INFINITE_GAS, precompileFlag, cds, transfersValue))
        .compile();
  }

  static Bytes expandMemoryTo2048Words() {
    return expandMemoryTo(2048);
  }

  static Bytes expandMemoryTo(int words) {
    checkArgument(words >= 1);
    return BytecodeCompiler.newProgram(chainConfig)
        .push((words - 1) * WORD_SIZE)
        .op(MLOAD)
        .op(POP)
        .compile();
  }

  /**
   * {@link #successfullySummonIntoExistence} MUST be provided with a valid (in particular nonzero)
   * call data size for <b>POINT_EVALUATION</b> and <b>BLS_XXX</b>. Otherwise, since the call is
   * given {@link #INFINITE_GAS}, the failed <b>CALL</b> could consume 63/64-ths of the frame's gas.
   * When used to compute the gas cost, this may end up being ~ 2B * (63/64) = 2B - 32M, which is
   * incompatible with OSAKA max transaction gas limits.
   *
   * @param precompileFlag
   * @return
   */
  static Bytes successfullySummonIntoExistence(
      PrecompileScenarioFragment.PrecompileFlag precompileFlag) {
    return call(
        INFINITE_GAS,
        precompileFlag,
        getMeaningfulCallDataSize(precompileFlag),
        // precompileFlag == PRC_BLAKE2F
        //     ? PRECOMPILE_CALL_DATA_SIZE___BLAKE2F
        //     : isBlsPrecompile(precompileFlag.getAddress())
        //         ? getMeaningfulCallDataSize(precompileFlag)
        //         : 0, // For BLAKE2F and BLS precompiles we need a meaningful cds for the call to
        // succeed
        true);
  }

  static Bytes call(
      Bytes gas,
      PrecompileScenarioFragment.PrecompileFlag precompileFlag,
      int cds,
      boolean transfersValue) {
    return BytecodeCompiler.newProgram(chainConfig)
        .immediate(pushCallArguments(gas, precompileFlag, cds, transfersValue))
        .op(CALL)
        .compile();
  }

  static Bytes pushCallArguments(
      Bytes gas,
      PrecompileScenarioFragment.PrecompileFlag precompileFlag,
      int cds,
      boolean transfersValue) {
    return BytecodeCompiler.newProgram(chainConfig)
        .push(0) // returnAtCapacity
        .push(0) // returnAtOffset
        .push(cds) // callDataSize
        .push(0) // callDataOffset
        .push(transfersValue ? 1 : 0) // value
        .push(precompileFlag.getAddress()) // address
        .push(gas) // gas
        .compile();
  }

  /**
   * Calculates the gas limit for a transaction to a contract calling a callee (in this class, a
   * precompile), based on targetCallGas, transfersValue, targetAddressExists, and
   * preCallProgramGas.
   *
   * @param targetCalleeGas the gas given to the callee.
   * @param transfersValue flag indicating if the call to callee transfers value.
   * @param targetAddressExists flag indicating if the target address exists.
   * @param preCallProgramGas the gas cost of the preCallProgram.
   * @return the calculated gas limit for the transaction.
   */
  static long getGasLimit(
      long targetCalleeGas,
      boolean transfersValue,
      boolean targetAddressExists,
      long preCallProgramGas) {
    /* gasLimit = preCallProgramGasCost + gasPreCall
    /  let x = gasPreCall - gasUpFront = 64 * k + l
    /  with:
    /  x !≡ 63 % 64 => (x - 63) % 64 != 0
    /  k = floor(x / 64)
    /  l = x % 64
    /  63 * k + l + stipend = targetCalleeGas
    /  find gasLimit going backwards
    */
    final long stipend = transfersValue ? GAS_CONST_G_CALL_STIPEND : 0;
    checkArgument(targetCalleeGas >= stipend);
    final long l = (targetCalleeGas - stipend) % 63;
    final long k = (targetCalleeGas - stipend - l) / 63;
    checkArgument(targetCalleeGas == 63 * k + l + stipend);
    final long gasUpfront = getUpfrontGasCost(transfersValue, targetAddressExists);
    final long gasPreCall = 64 * k + l + gasUpfront;
    final long gasLimit = preCallProgramGas + gasPreCall;
    return gasLimit; // gasLimit
  }

  /**
   * Calculates the upfront gas cost of a CALL-type instruction based on whether it transfers value
   * and if the target address exists. It assumed target address is warm and there is no memory
   * expansion.
   *
   * @param transfersValue flag indicating if the call transfers value.
   * @param targetAddressExists flag indicating if the target address exists.
   * @return the upfront gas cost for the call.
   */
  static long getUpfrontGasCost(boolean transfersValue, boolean targetAddressExists) {
    // GAS_CONST_G_WARM_ACCESS = 100
    // GAS_CONST_G_COLD_ACCOUNT_ACCESS = 2600
    // GAS_CONST_G_CALL_VALUE = 9000
    // GAS_CONST_G_NEW_ACCOUNT = 25000
    return GAS_CONST_G_WARM_ACCESS
        + (transfersValue
            ? GAS_CONST_G_CALL_VALUE + (targetAddressExists ? 0 : GAS_CONST_G_NEW_ACCOUNT)
            : 0);
  }

  static int getMeaningfulCallDataSize(PrecompileScenarioFragment.PrecompileFlag precompileFlag) {
    return switch (precompileFlag) {
      case PRC_SHA2_256, PRC_RIPEMD_160, PRC_IDENTITY ->
          1024 * WORD_SIZE; // Ensures cost is greater than stipend
      case PRC_MODEXP ->
          96 + bbs + ebs + mbs; // Ensures cost is greater than stipend with non-zero non-trivial
      case PRC_BLAKE2F -> PRECOMPILE_CALL_DATA_SIZE___BLAKE2F; // Ensures cost is greater than
      // stipend with non-zero non-trivial
      // input
      case PRC_POINT_EVALUATION -> PRECOMPILE_CALL_DATA_SIZE___POINT_EVALUATION;
      case PRC_BLS_G1_ADD -> PRECOMPILE_CALL_DATA_SIZE___G1_ADD;
      case PRC_BLS_G1_MSM -> PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G1_MSM; // 1 unit only
      case PRC_BLS_G2_ADD -> PRECOMPILE_CALL_DATA_SIZE___G2_ADD;
      case PRC_BLS_G2_MSM -> PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G2_MSM; // 1 unit only
      case PRC_BLS_PAIRING_CHECK -> PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK; // 1 unit
      // only
      case PRC_BLS_MAP_FP_TO_G1 -> PRECOMPILE_CALL_DATA_SIZE___FP_TO_G1;
      case PRC_BLS_MAP_FP2_TO_G2 -> PRECOMPILE_CALL_DATA_SIZE___FP2_TO_G2;
      case PRC_P256_VERIFY -> PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY; // Any size works
      default -> 0; // Remaining precompiles such as ECRECOVER, ECADD, ECMUL, ECPAIRING can be
        // called with cds = 0
    };
  }
}
