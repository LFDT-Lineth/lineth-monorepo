/*
 * Copyright ConsenSys AG.
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

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.Utilities.populateMemory;

import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag;
import net.consensys.linea.zktracer.module.tables.BlsRt;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;

public class PrecompileUtils extends TracerTestBase {
  private static final short G_QUADDIVISOR = 3;

  /**
   * Computes the precompile cost based on the precompile address, arguments size, and r value in
   * case of BLAKE2F.
   *
   * @param precompileAddress the address of the precompile contract.
   * @param cds the call data size for SHA256, RIPEMD160, ID, and MODEXP. For other precompile
   *     contracts this value is ignored.
   * @param bbs the base byte size for MODEXP. For other precompile contracts, this value is
   *     ignored.
   * @param mbs the modulus byte size for MODEXP. For other precompile contracts, this value is
   *     ignored.
   * @param exponentLog the logarithm of the exponent for MODEXP. For other precompile contracts,
   *     this value is ignored.
   * @param r the number of rounds for BLAKE2F. For other precompile contracts, this value is
   *     ignored.
   * @return the computed precompile cost.
   */
  public static int getPrecompileCost(
      Address precompileAddress, int cds, int bbs, int mbs, int exponentLog, int r) {

    final PrecompileFlag flag = PrecompileFlag.addressToPrecompileFlag(precompileAddress);

    return switch (flag) {
      case PRC_ECRECOVER -> getECRECCost();
      case PRC_SHA2_256 -> getSHA256Cost(cds);
      case PRC_RIPEMD_160 -> getRIPEMD160Cost(cds);
      case PRC_IDENTITY -> getIDCost(cds);
      case PRC_MODEXP -> getMODEXPCost(bbs, mbs, exponentLog);
      case PRC_ECADD -> getECADDCost();
      case PRC_ECMUL -> getECMULCost();
      case PRC_ECPAIRING -> getECPAIRINGCost(cds);
      case PRC_BLAKE2F -> getBLAKE2FCost(r);
      case PRC_POINT_EVALUATION -> getPointEvaluationCost();
      case PRC_BLS_G1_ADD -> getBlsG1AddCost();
      case PRC_BLS_G1_MSM -> getBlsG1MsmCost(cds);
      case PRC_BLS_G2_ADD -> getBlsG2AddCost();
      case PRC_BLS_G2_MSM -> getBlsG2MsmCost(cds);
      case PRC_BLS_PAIRING_CHECK -> getBlsPairingCheckCost(cds);
      case PRC_BLS_MAP_FP_TO_G1 -> getBlsMapFpToG1Cost();
      case PRC_BLS_MAP_FP2_TO_G2 -> getBlsMapFp2ToG2Cost();
      case PRC_P256_VERIFY -> getP256VerifyCost();
    };
  }

  public static int getPrecompileCost(Address precompileAddress, int cds) {
    return getPrecompileCost(precompileAddress, cds, 0, 0, 0, 0);
  }

  private static int words(int sizeInBytes) {
    return (sizeInBytes + WORD_SIZE_MO) / WORD_SIZE;
  }

  public static int getECRECCost() {
    return GAS_CONST_ECRECOVER;
  }

  public static int getSHA256Cost(int cds) {
    return GAS_CONST_SHA2 + words(cds) * GAS_CONST_SHA2_WORD;
  }

  public static int getRIPEMD160Cost(int cds) {
    return GAS_CONST_RIPEMD + words(cds) * GAS_CONST_RIPEMD_WORD;
  }

  public static int getIDCost(int cds) {
    return GAS_CONST_IDENTITY + words(cds) * GAS_CONST_IDENTITY_WORD;
  }

  public static int getMODEXPCost(int bbs, int mbs, int exponentLog) {
    final int fOfMax = ((Math.max(bbs, mbs) + 7) / 8) * ((Math.max(bbs, mbs) + 7) / 8);
    final int bigNumerator = fOfMax * Math.max(exponentLog, 1);
    final int bigQuotient = bigNumerator / G_QUADDIVISOR;
    return Math.max(GAS_CONST_MODEXP, bigQuotient);
  }

  public static int getECADDCost() {
    return GAS_CONST_ECADD;
  }

  public static int getECMULCost() {
    return GAS_CONST_ECMUL;
  }

  public static int getECPAIRINGCost(int cds) {
    return GAS_CONST_ECPAIRING
        + GAS_CONST_ECPAIRING_PAIR * (cds / PRECOMPILE_CALL_DATA_UNIT_SIZE___ECPAIRING);
  }

  public static int getBLAKE2FCost(int r) {
    return GAS_CONST_BLAKE2_PER_ROUND * r;
  }

  public static int getPointEvaluationCost() {
    return GAS_CONST_POINT_EVALUATION;
  }

  public static int getBlsG1AddCost() {
    return GAS_CONST_BLS_G1_ADD;
  }

  public static int getBlsG1MsmCost(int cds) {
    final int numInputs = cds / PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G1_MSM;
    final int discount = BlsRt.getMsmDiscount(OOB_INST_BLS_G1_MSM, numInputs);
    return (numInputs * PRC_BLS_G1_MSM_MULTIPLICATION_COST * discount)
        / PRC_BLS_MULTIPLICATION_MULTIPLIER;
  }

  public static int getBlsG2AddCost() {
    return GAS_CONST_BLS_G2_ADD;
  }

  public static int getBlsG2MsmCost(int cds) {
    final int numInputs = cds / PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G2_MSM;
    final int discount = BlsRt.getMsmDiscount(OOB_INST_BLS_G2_MSM, numInputs);
    return (numInputs * PRC_BLS_G2_MSM_MULTIPLICATION_COST * discount)
        / PRC_BLS_MULTIPLICATION_MULTIPLIER;
  }

  public static int getBlsPairingCheckCost(int cds) {
    return GAS_CONST_BLS_PAIRING_CHECK
        + GAS_CONST_BLS_PAIRING_CHECK_PAIR
            * (cds / PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK);
  }

  public static int getBlsMapFpToG1Cost() {
    return GAS_CONST_BLS_MAP_FP_TO_G1;
  }

  public static int getBlsMapFp2ToG2Cost() {
    return GAS_CONST_BLS_MAP_FP2_TO_G2;
  }

  public static int getP256VerifyCost() {
    return GAS_CONST_P256_VERIFY;
  }

  // Methods to prepare inputs for certain precompiles

  // BLAKE2F
  static void prepareBlake2F(BytecodeCompiler program, int rLeadingByte, int offset) {
    program
        .push(rLeadingByte) // For simplicity, we only set the first byte of r
        .push(offset) // offset
        .op(OpCode.MSTORE8);
  }

  public static Bytes prepareBlake2F(int rLeadingByte, int offset) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    prepareBlake2F(program, rLeadingByte, offset);
    return program.compile();
  }

  // SHA256, RIPEMD160, ID
  static void prepareSha256Ripemd160Id(BytecodeCompiler program, int nWords, int offset) {
    populateMemory(program, nWords, offset);
  }

  static Bytes prepareSha256Ripemd160Id(int nWords, int offset) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    prepareSha256Ripemd160Id(program, nWords, offset);
    return program.compile();
  }

  // MODEXP
  public static Bytes generateModexpInput(int bbs, int mbs, int ebs) {
    final Bytes32 bbsPadded = Bytes32.leftPad(Bytes.of(bbs));
    final Bytes32 ebsPadded = Bytes32.leftPad(Bytes.of(ebs));
    final Bytes32 mbsPadded = Bytes32.leftPad(Bytes.of(mbs));
    final Bytes bem =
        Bytes.fromHexString("0x" + "aa".repeat(bbs) + "ff".repeat(ebs) + "bb".repeat(mbs));
    return Bytes.concatenate(bbsPadded, ebsPadded, mbsPadded, bem);
  }

  static void writeInMemoryByteCodeOfCodeOwner(
      Address codeOwnerAddress, int targetOffset, BytecodeCompiler program) {
    // Copy to targetOffset the code of codeOwnerAccount
    program
        .push(codeOwnerAddress)
        .op(OpCode.EXTCODESIZE) // size
        .push(0) // offset
        .push(targetOffset) // targetOffset
        .push(codeOwnerAddress) // address
        .op(OpCode.EXTCODECOPY);
  }

  // TODO: this is mostly a duplicate of Tests.fullCodeCopyOf, modulo the target offset. Decide
  // which one to keep.
  public static Bytes writeInMemoryByteCodeOfCodeOwner(Address codeOwnerAddress, int targetOffset) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    writeInMemoryByteCodeOfCodeOwner(codeOwnerAddress, targetOffset, program);
    return program.compile();
  }

  /**
   * Computes the expected returnAtCapacity based on the precompile address, and callDataSize in the
   * case of ID.
   *
   * @param precompileAddress the address of the precompile contract.
   * @param callDataSize the call data size. Beyond the case of ID, this value is ignored.
   * @param mbs the modulo byte size. Beyond the case of MODEXP, this value is ignored.
   * @return the computed return at capacity.
   */
  static int getExpectedReturnAtCapacity(Address precompileAddress, int callDataSize, int mbs) {

    final PrecompileFlag flag = PrecompileFlag.addressToPrecompileFlag(precompileAddress);

    return switch (flag) {
      case PRC_ECRECOVER, PRC_SHA2_256, PRC_RIPEMD_160, PRC_ECPAIRING -> WORD_SIZE;
      case PRC_ECADD, PRC_ECMUL, PRC_BLAKE2F -> 2 * WORD_SIZE;
      case PRC_MODEXP -> mbs;
      case PRC_IDENTITY -> callDataSize;
      case PRC_POINT_EVALUATION -> PRECOMPILE_RETURN_DATA_SIZE___POINT_EVALUATION;
      case PRC_BLS_G1_ADD -> PRECOMPILE_RETURN_DATA_SIZE___BLS_G1_ADD;
      case PRC_BLS_G1_MSM -> PRECOMPILE_RETURN_DATA_SIZE___BLS_G1_MSM;
      case PRC_BLS_G2_ADD -> PRECOMPILE_RETURN_DATA_SIZE___BLS_G2_ADD;
      case PRC_BLS_G2_MSM -> PRECOMPILE_RETURN_DATA_SIZE___BLS_G2_MSM;
      case PRC_BLS_PAIRING_CHECK -> PRECOMPILE_RETURN_DATA_SIZE___BLS_PAIRING_CHECK;
      case PRC_BLS_MAP_FP_TO_G1 -> PRECOMPILE_RETURN_DATA_SIZE___BLS_MAP_FP_TO_G1;
      case PRC_BLS_MAP_FP2_TO_G2 -> PRECOMPILE_RETURN_DATA_SIZE___BLS_MAP_FP2_TO_G2;
      case PRC_P256_VERIFY -> PRECOMPILE_RETURN_DATA_SIZE___P256_VERIFY;
    };
  }
}
