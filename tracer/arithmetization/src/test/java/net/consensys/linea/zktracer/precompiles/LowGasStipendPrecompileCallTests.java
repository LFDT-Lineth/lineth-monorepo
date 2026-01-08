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

package net.consensys.linea.zktracer.precompiles;

import static net.consensys.linea.testing.BytecodeRunner.MAX_GAS_LIMIT;
import static net.consensys.linea.zktracer.Fork.forkPredatesOsaka;
import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpPricingOobCall.computeExponentLog;
import static net.consensys.linea.zktracer.opcode.OpCode.CALL;
import static net.consensys.linea.zktracer.opcode.OpCode.JUMPDEST;
import static net.consensys.linea.zktracer.precompiles.LowGasStipendPrecompileCallTests.GasCase.COST;
import static net.consensys.linea.zktracer.precompiles.LowGasStipendPrecompileCallTests.GasCase.COST_MINUS_ONE;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.generateModexpInput;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.getExpectedReturnAtCapacity;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.getPrecompileCost;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.prepareBlake2F;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.prepareSha256Ripemd160Id;
import static net.consensys.linea.zktracer.precompiles.PrecompileUtils.writeInMemoryByteCodeOfCodeOwner;
import static org.hyperledger.besu.datatypes.Address.*;

import com.google.common.base.Preconditions;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

public class LowGasStipendPrecompileCallTests extends TracerTestBase {

  // Enums for the different testing scenarios
  enum ValueCase {
    ZERO,
    NON_ZERO;

    boolean isZeroCase() {
      return this == ZERO;
    }

    boolean isNonZeroCase() {
      return this == NON_ZERO;
    }
  }

  enum GasCase {
    ZERO,
    ONE,
    COST_MINUS_ONE,
    COST,
    COST_PLUS_ONE;
  }

  // BLAKE2F specific parameters
  int rLeadingByte = 0;
  int r = 0;

  // MODEXP specific parameters
  int bbs; // Defined within the test case
  int ebs; // Defined within the test case
  int mbs; // Defined within the test case
  int exponentLog; // Computed within the test case
  final Address codeOwnerAddress = Address.fromHexString("0xC0DE");
  List<ToyAccount> additionalAccounts = new ArrayList<>(); // Re-assigned within the test case

  /**
   * Parameterized test for low gas stipend precompile call. In this family of tests we call every
   * precompile with hand-crafted gas parameters in the CALL. The frame itself is provided with
   * ample gas, but we choose the gas parameter to be equal to the execution cost of the precompile
   * contract (up to -1, 0, +1). This allows us to deliberately trigger failures in the precompiles
   * due to insufficient gas and test that, on the contrary, calls to precompiles with *exactly* the
   * right amount of gas are successful (given that other conditions are met in terms of the
   * contents of the call data).
   *
   * @param precompileAddress the address of the precompile contract.
   * @param valueCase the value case (zero or non-zero).
   * @param gasCase the gas case (zero, one, cost minus one, cost, cost plus one).
   * @param callDataSize the size of the call data. It is ignored for MODEXP as it is defined
   *     internally.
   * @param modexpCostGT200OrBlake2fRoundsGT0 flag indicating if the MODEXP cost is greater than 200
   *     or if the BLAKE2F rounds are greater than 0. It is ignored for other precompile contracts.
   */
  @Tag("nightly")
  @ParameterizedTest
  @MethodSource("lowGasStipendPrecompileCallTestSource")
  @MethodSource("lowGasStipendPrecompileCallP256TestSource")
  void lowGasStipendPrecompileCallTest(
      Address precompileAddress,
      ValueCase valueCase,
      GasCase gasCase,
      Integer callDataSize,
      boolean modexpCostGT200OrBlake2fRoundsGT0,
      TestInfo testInfo) {
    final BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    // In order to actually trigger the insufficient gas we need to:
    // - Set a specific callDataSize for BLAKE2F and EC_PAIRING and BLS precompiles (already passed
    // as a parameter)
    // - Set the r value of BLAKE2F to have precompileCost > callStipend
    // - Populate the memory with a large enough number of words for SHA256, RIPEMD160, and ID
    //   to have precompileCost > callStipend.
    final int value = valueCase.isZeroCase() ? 0 : 1;
    // final int callDataSize; // depends on the called precompile
    final int callDataOffset = 0;

    // returnAtCapacity is defined below
    final int returnAtOffset = 13;

    // Prepare the arguments for the different precompile calls
    if (precompileAddress == BLAKE2B_F_COMPRESSION) {
      rLeadingByte = modexpCostGT200OrBlake2fRoundsGT0 ? 0x12 : 0;
      r = rLeadingByte << 8;
      prepareBlake2F(program, rLeadingByte, callDataOffset + 2);
    } else if ((precompileAddress == SHA256
        || precompileAddress == RIPEMD160
        || precompileAddress == ID)) {
      prepareSha256Ripemd160Id(program, callDataSize / WORD_SIZE, callDataOffset);
    } else if (precompileAddress == MODEXP) {
      bbs = modexpCostGT200OrBlake2fRoundsGT0 ? 1 : 2;
      ebs = modexpCostGT200OrBlake2fRoundsGT0 ? 6 : 3;
      mbs = modexpCostGT200OrBlake2fRoundsGT0 ? 25 : 4;
      Preconditions.checkArgument(callDataSize == null);
      callDataSize = 96 + bbs + ebs + mbs;

      final Bytes modexpInput = generateModexpInput(bbs, mbs, ebs);
      final int multiplier = (forkPredatesOsaka(fork)) ? 8 : 16;
      exponentLog = computeExponentLog(modexpInput, multiplier, callDataSize, bbs, ebs);
      // codeOwnerAccount owns the bytecode that will be given as input to MODEXP through
      // EXTCODECOPY
      final ToyAccount codeOwnerAccount =
          ToyAccount.builder()
              .balance(Wei.of(0))
              .nonce(1)
              .address(codeOwnerAddress)
              .code(modexpInput)
              .build();
      additionalAccounts = List.of(codeOwnerAccount);

      writeInMemoryByteCodeOfCodeOwner(codeOwnerAddress, callDataOffset, program);
    }

    // Set returnAtCapacity equal to the expected return size of the precompile call
    final int returnAtCapacity = getExpectedReturnAtCapacity(precompileAddress, callDataSize, mbs);

    // Compute the precompile cost
    final int precompileCost =
        getPrecompileCost(precompileAddress, callDataSize, bbs, mbs, exponentLog, r);

    // Compute the gas parameter of the CALL in the different testing scenarios
    int gas = getGas(gasCase, precompileCost);

    // In case funds are sent to the precompile contract (valueCase == NON_ZERO)
    // a gas stipend of 2300 is added to the precompile's frame initial gas.
    // We now deduce that gas stipend from the gas given to the transaction to trigger
    // insufficient gas for the precompile call in the non-trivial cases (COST_MINUS_ONE, COST,
    // COST_PLUS_ONE).
    // Note that we exclude the case of MODEXP as it is treated in a separate test,
    // the case of ECADD as it has a fixed gas cost of 150 < 2300,
    // the case of BLS12_G1ADD as it has a fixed gas cost of 375 < 2300
    // and the case of BLS12_G2ADD as it has a fixed gas cost of 600 < 2300.
    // TODO: are there other cases to exclude?
    if (valueCase.isNonZeroCase()
        && (gasCase == COST_MINUS_ONE || gasCase == COST || gasCase == GasCase.COST_PLUS_ONE)
        && !precompileAddress.equals(ALTBN128_ADD)
        && !precompileAddress.equals(BLS12_G1ADD)
        && !precompileAddress.equals(BLS12_G2ADD)
        && !precompileAddress.equals(MODEXP)) {
      gas -= GAS_CONST_G_CALL_STIPEND;
    }

    // Common program for all precompile calls
    program
        .push(returnAtCapacity) // returnAtCapacity
        .push(returnAtOffset) // returnAtOffset
        .push(callDataSize) // callDataSize
        .push(callDataOffset) // callDataOffset
        .push(value) // value
        .push(precompileAddress) // address
        .push(gas) // gas
        .op(CALL);

    for (int i = 0; i < 32; i++) {
      program.op(JUMPDEST);
    }

    final BytecodeRunner bytecodeRunner = BytecodeRunner.of(program);
    final long forkAppropriateGasLimit = forkPredatesOsaka(fork) ? 61_000_000L : MAX_GAS_LIMIT;
    bytecodeRunner.run(
        forkAppropriateGasLimit,
        precompileAddress == MODEXP ? additionalAccounts : List.of(),
        chainConfig,
        testInfo);
  }

  static Stream<Arguments> lowGasStipendPrecompileCallP256TestSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (GasCase gasCase : GasCase.values()) {
      for (ValueCase valueCase : ValueCase.values()) {
        arguments.add(Arguments.of(P256_VERIFY, valueCase, gasCase, 0, false));
        arguments.add(Arguments.of(P256_VERIFY, valueCase, gasCase, 1, false));
        arguments.add(
            Arguments.of(
                P256_VERIFY,
                valueCase,
                gasCase,
                PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY - 1,
                false));
        arguments.add(
            Arguments.of(
                P256_VERIFY, valueCase, gasCase, PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY, false));
        arguments.add(
            Arguments.of(
                P256_VERIFY,
                valueCase,
                gasCase,
                PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY + 1,
                false));
        arguments.add(Arguments.of(P256_VERIFY, valueCase, gasCase, Integer.MAX_VALUE, false));
      }
    }
    arguments.clear();
    arguments.add(Arguments.of(P256_VERIFY, ValueCase.ZERO, COST_MINUS_ONE, 160, false));
    arguments.add(Arguments.of(BLS12_G1ADD, ValueCase.ZERO, COST, 13, false));
    return arguments.stream();
  }

  static Stream<Arguments> lowGasStipendPrecompileCallTestSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (GasCase gasCase : GasCase.values()) {
      for (ValueCase valueCase : ValueCase.values()) {
        // 1 is an arbitrary callDataSize for ECREC, ALTBN128_ADD, ALTBN128_MUL
        arguments.add(Arguments.of(ECREC, valueCase, gasCase, 1, false));
        // 1024 * WORD_SIZE guarantees precompileCost > callStipend for SHA256, RIPEMD160 and ID
        arguments.add(Arguments.of(SHA256, valueCase, gasCase, 1024 * WORD_SIZE, false));
        arguments.add(Arguments.of(RIPEMD160, valueCase, gasCase, 1024 * WORD_SIZE, false));
        arguments.add(Arguments.of(ID, valueCase, gasCase, 1024 * WORD_SIZE, false));
        arguments.add(Arguments.of(ALTBN128_ADD, valueCase, gasCase, 1, false));
        arguments.add(Arguments.of(ALTBN128_MUL, valueCase, gasCase, 1, false));
        arguments.add(
            Arguments.of(
                ALTBN128_PAIRING,
                valueCase,
                gasCase,
                PRECOMPILE_CALL_DATA_UNIT_SIZE___ECPAIRING,
                false));
        arguments.add(
            Arguments.of(
                BLAKE2B_F_COMPRESSION,
                valueCase,
                gasCase,
                PRECOMPILE_CALL_DATA_SIZE___BLAKE2F,
                false));
        arguments.add(
            Arguments.of(
                BLAKE2B_F_COMPRESSION,
                valueCase,
                gasCase,
                PRECOMPILE_CALL_DATA_SIZE___BLAKE2F,
                true));
        // BLS precompiles
        arguments.add(
            Arguments.of(
                KZG_POINT_EVAL,
                valueCase,
                gasCase,
                PRECOMPILE_CALL_DATA_SIZE___POINT_EVALUATION,
                false));
        arguments.add(
            Arguments.of(
                BLS12_G1ADD, valueCase, gasCase, PRECOMPILE_CALL_DATA_SIZE___G1_ADD, false));
        arguments.add(
            Arguments.of(
                BLS12_G2ADD, valueCase, gasCase, PRECOMPILE_CALL_DATA_SIZE___G2_ADD, false));
        arguments.add(
            Arguments.of(
                BLS12_PAIRING,
                valueCase,
                gasCase,
                PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK,
                false));
        arguments.add(
            Arguments.of(
                BLS12_MAP_FP_TO_G1,
                valueCase,
                gasCase,
                PRECOMPILE_CALL_DATA_SIZE___FP_TO_G1,
                false));
        arguments.add(
            Arguments.of(
                BLS12_MAP_FP2_TO_G2,
                valueCase,
                gasCase,
                PRECOMPILE_CALL_DATA_SIZE___FP2_TO_G2,
                false));
        for (int numberOfUnits = 1; numberOfUnits <= 128 + 1; numberOfUnits++) {
          arguments.add(
              Arguments.of(
                  BLS12_G1MULTIEXP,
                  valueCase,
                  gasCase,
                  numberOfUnits * PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G1_MSM,
                  false));
          arguments.add(
              Arguments.of(
                  BLS12_G2MULTIEXP,
                  valueCase,
                  gasCase,
                  numberOfUnits * PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G2_MSM,
                  false));
        }
        arguments.add(Arguments.of(P256_VERIFY, valueCase, gasCase, 0, false));
        arguments.add(Arguments.of(P256_VERIFY, valueCase, gasCase, 1, false));
        arguments.add(
            Arguments.of(
                P256_VERIFY,
                valueCase,
                gasCase,
                PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY - 1,
                false));
        arguments.add(
            Arguments.of(
                P256_VERIFY, valueCase, gasCase, PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY, false));
        arguments.add(
            Arguments.of(
                P256_VERIFY,
                valueCase,
                gasCase,
                PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY + 1,
                false));
        arguments.add(Arguments.of(P256_VERIFY, valueCase, gasCase, Integer.MAX_VALUE, false));
      }
      // The NON_ZERO for MODEXP case will be treated in a separate test
      // callDataSize is defined internally
      // TODO
      arguments.add(Arguments.of(MODEXP, ValueCase.ZERO, gasCase, null, false));
      arguments.add(Arguments.of(MODEXP, ValueCase.ZERO, gasCase, null, true));
    }

    return arguments.stream();
  }

  // Support methods
  /**
   * Computes the gas based on the gas parameter and precompile cost.
   *
   * @param gasCase the gas case.
   * @param precompileCost the precompile cost.
   * @return the computed gas.
   */
  private static int getGas(GasCase gasCase, int precompileCost) {
    return switch (gasCase) {
      case ZERO -> 0;
      case ONE -> 1;
      case COST_MINUS_ONE -> precompileCost - 1;
      case COST -> precompileCost;
      case COST_PLUS_ONE -> precompileCost + 1;
    };
  }
}
