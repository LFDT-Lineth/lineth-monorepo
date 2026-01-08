/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.module.hub.fragment.scenario;

import static java.util.Map.entry;
import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.Trace.Oob.*;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.*;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileScenario.*;
import static net.consensys.linea.zktracer.module.mod.ModOperation.NB_ROWS_MOD;

import java.util.Map;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection.PrecompileSubsection;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;

@Getter
@AllArgsConstructor
@Accessors(fluent = true)
public class PrecompileScenarioFragment implements TraceFragment {

  public enum PrecompileScenario {
    PRC_FAILURE_KNOWN_TO_HUB,
    PRC_FAILURE_KNOWN_TO_RAM,
    PRC_SUCCESS_WILL_REVERT,
    PRC_SUCCESS_WONT_REVERT;

    public boolean isFailure() {
      return this == PRC_FAILURE_KNOWN_TO_HUB || this == PRC_FAILURE_KNOWN_TO_RAM;
    }

    public boolean isSuccess() {
      return this == PRC_SUCCESS_WILL_REVERT || this == PRC_SUCCESS_WONT_REVERT;
    }
  }

  public enum PrecompileFlag {
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
    PRC_P256_VERIFY;

    public Address getAddress() {
      return ADDRESS_TO_FLAG_MAP.entrySet().stream()
          .filter(entry -> entry.getValue() == this)
          .map(Map.Entry::getKey)
          .findFirst()
          .orElseThrow(
              () -> new IllegalArgumentException("Precompile not included in ADDRESS_TO_FLAG_MAP"));
    }

    private static final Map<Address, PrecompileFlag> ADDRESS_TO_FLAG_MAP =
        Map.ofEntries(
            entry(Address.ECREC, PRC_ECRECOVER),
            entry(Address.SHA256, PRC_SHA2_256),
            entry(Address.RIPEMD160, PRC_RIPEMD_160),
            entry(Address.ID, PRC_IDENTITY),
            entry(Address.MODEXP, PRC_MODEXP),
            entry(Address.ALTBN128_ADD, PRC_ECADD),
            entry(Address.ALTBN128_MUL, PRC_ECMUL),
            entry(Address.ALTBN128_PAIRING, PRC_ECPAIRING),
            entry(Address.BLAKE2B_F_COMPRESSION, PRC_BLAKE2F),
            entry(Address.KZG_POINT_EVAL, PRC_POINT_EVALUATION),
            entry(Address.BLS12_G1ADD, PRC_BLS_G1_ADD),
            entry(Address.BLS12_G1MULTIEXP, PRC_BLS_G1_MSM),
            entry(Address.BLS12_G2ADD, PRC_BLS_G2_ADD),
            entry(Address.BLS12_G2MULTIEXP, PRC_BLS_G2_MSM),
            entry(Address.BLS12_PAIRING, PRC_BLS_PAIRING_CHECK),
            entry(Address.BLS12_MAP_FP_TO_G1, PRC_BLS_MAP_FP_TO_G1),
            entry(Address.BLS12_MAP_FP2_TO_G2, PRC_BLS_MAP_FP2_TO_G2),
            entry(Address.P256_VERIFY, PRC_P256_VERIFY));

    private static final Map<PrecompileFlag, Integer> DATA_PHASE_MAP =
        Map.ofEntries(
            Map.entry(PRC_ECRECOVER, Trace.PHASE_ECRECOVER_DATA),
            Map.entry(PRC_SHA2_256, Trace.PHASE_SHA2_DATA),
            Map.entry(PRC_RIPEMD_160, Trace.PHASE_RIPEMD_DATA),
            // IDENTITY not supported
            // MODEXP not supported
            Map.entry(PRC_ECADD, Trace.PHASE_ECADD_DATA),
            Map.entry(PRC_ECMUL, Trace.PHASE_ECMUL_DATA),
            Map.entry(PRC_ECPAIRING, Trace.PHASE_ECPAIRING_DATA),
            // BLAKE2f not supported
            Map.entry(PRC_POINT_EVALUATION, Trace.PHASE_POINT_EVALUATION_DATA),
            Map.entry(PRC_BLS_G1_ADD, Trace.PHASE_BLS_G1_ADD_DATA),
            Map.entry(PRC_BLS_G1_MSM, Trace.PHASE_BLS_G1_MSM_DATA),
            Map.entry(PRC_BLS_G2_ADD, Trace.PHASE_BLS_G2_ADD_DATA),
            Map.entry(PRC_BLS_G2_MSM, Trace.PHASE_BLS_G2_MSM_DATA),
            Map.entry(PRC_BLS_PAIRING_CHECK, Trace.PHASE_BLS_PAIRING_CHECK_DATA),
            Map.entry(PRC_BLS_MAP_FP_TO_G1, Trace.PHASE_BLS_MAP_FP_TO_G1_DATA),
            Map.entry(PRC_BLS_MAP_FP2_TO_G2, Trace.PHASE_BLS_MAP_FP2_TO_G2_DATA),
            Map.entry(PRC_P256_VERIFY, Trace.PHASE_P256_VERIFY_DATA));

    private static final Map<PrecompileFlag, Integer> RESULT_PHASE_MAP =
        Map.ofEntries(
            Map.entry(PRC_ECRECOVER, Trace.PHASE_ECRECOVER_RESULT),
            Map.entry(PRC_SHA2_256, Trace.PHASE_SHA2_RESULT),
            Map.entry(PRC_RIPEMD_160, Trace.PHASE_RIPEMD_RESULT),
            // IDENTITY not supported
            // MODEXP not supported
            Map.entry(PRC_ECADD, Trace.PHASE_ECADD_RESULT),
            Map.entry(PRC_ECMUL, Trace.PHASE_ECMUL_RESULT),
            Map.entry(PRC_ECPAIRING, Trace.PHASE_ECPAIRING_RESULT),
            // BLAKE2f not supported
            Map.entry(PRC_POINT_EVALUATION, Trace.PHASE_POINT_EVALUATION_RESULT),
            Map.entry(PRC_BLS_G1_ADD, Trace.PHASE_BLS_G1_ADD_RESULT),
            Map.entry(PRC_BLS_G1_MSM, Trace.PHASE_BLS_G1_MSM_RESULT),
            Map.entry(PRC_BLS_G2_ADD, Trace.PHASE_BLS_G2_ADD_RESULT),
            Map.entry(PRC_BLS_G2_MSM, Trace.PHASE_BLS_G2_MSM_RESULT),
            Map.entry(PRC_BLS_PAIRING_CHECK, Trace.PHASE_BLS_PAIRING_CHECK_RESULT),
            Map.entry(PRC_BLS_MAP_FP_TO_G1, Trace.PHASE_BLS_MAP_FP_TO_G1_RESULT),
            Map.entry(PRC_BLS_MAP_FP2_TO_G2, Trace.PHASE_BLS_MAP_FP2_TO_G2_RESULT),
            Map.entry(PRC_P256_VERIFY, Trace.PHASE_P256_VERIFY_RESULT));

    public static PrecompileFlag addressToPrecompileFlag(Address precompileAddress) {
      if (!ADDRESS_TO_FLAG_MAP.containsKey(precompileAddress)) {
        throw new IllegalArgumentException(
            "Not valid precompile address: " + precompileAddress.toString());
      }
      return ADDRESS_TO_FLAG_MAP.get(precompileAddress);
    }

    public int dataPhase() {
      if (!DATA_PHASE_MAP.containsKey(this)) {
        throw new IllegalArgumentException("Precompile not supported by the DATA_PHASE_MAP");
      }
      return DATA_PHASE_MAP.get(this);
    }

    public int resultPhase() {
      if (!RESULT_PHASE_MAP.containsKey(this)) {
        throw new IllegalArgumentException("Precompile not supported by the RESULT_PHASE_MAP");
      }
      return RESULT_PHASE_MAP.get(this);
    }

    public boolean isEcdataPrecompile() {
      return this.isAnyOf(PRC_ECRECOVER, PRC_ECADD, PRC_ECMUL, PRC_ECPAIRING, PRC_P256_VERIFY);
    }

    public boolean isBlsPrecompile() {
      return this.isAnyOf(
          PRC_POINT_EVALUATION,
          PRC_BLS_G1_ADD,
          PRC_BLS_G1_MSM,
          PRC_BLS_G2_ADD,
          PRC_BLS_G2_MSM,
          PRC_BLS_PAIRING_CHECK,
          PRC_BLS_MAP_FP_TO_G1,
          PRC_BLS_MAP_FP2_TO_G2);
    }

    public boolean isAnyOf(PrecompileFlag... flags) {
      for (PrecompileFlag flag : flags) {
        if (this == flag) {
          return true;
        }
      }
      return false;
    }

    public static boolean validCallDataSize(
        PrecompileScenarioFragment.PrecompileFlag prc, int cds) {
      return switch (prc) {
        case PRC_POINT_EVALUATION -> cds == PRECOMPILE_CALL_DATA_SIZE___POINT_EVALUATION;
        case PRC_BLS_G1_ADD -> cds == PRECOMPILE_CALL_DATA_SIZE___G1_ADD;
        case PRC_BLS_G1_MSM -> cds != 0 && cds % PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G1_MSM == 0;
        case PRC_BLS_G2_ADD -> cds == PRECOMPILE_CALL_DATA_SIZE___G2_ADD;
        case PRC_BLS_G2_MSM -> cds != 0 && cds % PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_G2_MSM == 0;
        case PRC_BLS_PAIRING_CHECK ->
            cds != 0 && cds % PRECOMPILE_CALL_DATA_UNIT_SIZE___BLS_PAIRING_CHECK == 0;
        case PRC_BLS_MAP_FP_TO_G1 -> cds == PRECOMPILE_CALL_DATA_SIZE___FP_TO_G1;
        case PRC_BLS_MAP_FP2_TO_G2 -> cds == PRECOMPILE_CALL_DATA_SIZE___FP2_TO_G2;
        case PRC_P256_VERIFY -> cds == PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY;
        default -> throw new IllegalArgumentException("not implemented for prc: " + prc);
      };
    }

    public static short modLinesComingFromOobCall(PrecompileScenarioFragment.PrecompileFlag prc) {
      return switch (prc) {
        case PRC_SHA2_256, PRC_RIPEMD_160, PRC_IDENTITY, PRC_BLS_PAIRING_CHECK, PRC_ECPAIRING ->
            NB_ROWS_MOD;
        case PRC_MODEXP, PRC_BLS_G1_MSM, PRC_BLS_G2_MSM -> (short) (NB_ROWS_MOD * 2);
        default -> 0;
      };
    }
  }

  final PrecompileSubsection precompileSubSection;
  @Setter public PrecompileScenario scenario;
  @Setter public PrecompileFlag flag;

  public PrecompileScenarioFragment(
      final PrecompileSubsection precompileSubsection,
      final PrecompileFlag flag,
      final PrecompileScenario scenario) {
    this.precompileSubSection = precompileSubsection;
    this.flag = flag;
    this.scenario = scenario;
  }

  public boolean isPrcFailure() {
    return scenario == PRC_FAILURE_KNOWN_TO_HUB || scenario == PRC_FAILURE_KNOWN_TO_RAM;
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    trace
        .peekAtScenario(true)
        .pScenarioPrcEcrecover(flag == PRC_ECRECOVER)
        .pScenarioPrcSha2256(flag == PRC_SHA2_256)
        .pScenarioPrcRipemd160(flag == PRC_RIPEMD_160)
        .pScenarioPrcIdentity(flag == PRC_IDENTITY)
        .pScenarioPrcModexp(flag == PRC_MODEXP)
        .pScenarioPrcEcadd(flag == PRC_ECADD)
        .pScenarioPrcEcmul(flag == PRC_ECMUL)
        .pScenarioPrcEcpairing(flag == PRC_ECPAIRING)
        .pScenarioPrcBlake2f(flag == PRC_BLAKE2F)
        .pScenarioPrcSuccessCallerWillRevert(scenario == PRC_SUCCESS_WILL_REVERT)
        .pScenarioPrcSuccessCallerWontRevert(scenario == PRC_SUCCESS_WONT_REVERT)
        .pScenarioPrcFailureKnownToHub(scenario == PRC_FAILURE_KNOWN_TO_HUB)
        .pScenarioPrcFailureKnownToRam(scenario == PRC_FAILURE_KNOWN_TO_RAM)
        .pScenarioPrcCallerGas(Bytes.ofUnsignedLong(precompileSubSection.callerGas()))
        .pScenarioPrcCalleeGas(Bytes.ofUnsignedLong(precompileSubSection.calleeGas()))
        .pScenarioPrcReturnGas(Bytes.ofUnsignedLong(precompileSubSection.returnGas()))
        .pScenarioPrcCdo(precompileSubSection.callDataOffset())
        .pScenarioPrcCds(precompileSubSection.callDataSize())
        .pScenarioPrcRao(precompileSubSection.returnAtOffset())
        .pScenarioPrcRac(precompileSubSection.returnAtCapacity())
        .pScenarioPrcPointEvaluation(flag == PRC_POINT_EVALUATION)
        .pScenarioPrcBlsG1Add(flag == PRC_BLS_G1_ADD)
        .pScenarioPrcBlsG1Msm(flag == PRC_BLS_G1_MSM)
        .pScenarioPrcBlsG2Add(flag == PRC_BLS_G2_ADD)
        .pScenarioPrcBlsG2Msm(flag == PRC_BLS_G2_MSM)
        .pScenarioPrcBlsPairingCheck(flag == PRC_BLS_PAIRING_CHECK)
        .pScenarioPrcBlsMapFpToG1(flag == PRC_BLS_MAP_FP_TO_G1)
        .pScenarioPrcBlsMapFp2ToG2(flag == PRC_BLS_MAP_FP2_TO_G2)
        .pScenarioPrcP256Verify(flag == PRC_P256_VERIFY);
    return trace;
  }
}
