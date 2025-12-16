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
package net.consensys.linea.zktracer.module.hub.section.call.precompileSubsection;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.*;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileScenario.PRC_FAILURE_KNOWN_TO_HUB;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileScenario.PRC_FAILURE_KNOWN_TO_RAM;

import com.google.common.base.Preconditions;
import java.math.BigInteger;
import net.consensys.linea.zktracer.module.blsdata.BlsData;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.mmu.MmuCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.OobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.CommonPrecompileOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.EcPairingOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.ecAddMulRecover.EcAddOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.ecAddMulRecover.EcMulOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.ecAddMulRecover.EcRecoverOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.BlsPairingCheckOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.fixedSizeFixedGasCost.BlsG1AddOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.fixedSizeFixedGasCost.BlsG2AddOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.fixedSizeFixedGasCost.BlsMapFp2ToG2OobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.fixedSizeFixedGasCost.BlsMapFpToG1OobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.fixedSizeFixedGasCost.BlsPointEvaluationOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.fixedSizeFixedGasCost.P256VerifyOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.msm.BlsG1MsmOobCall;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.msm.BlsG2MsmOobCall;
import net.consensys.linea.zktracer.module.hub.section.call.CallSection;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import org.apache.tuweni.bytes.Bytes;

public class EllipticCurvePrecompileSubsection extends PrecompileSubsection {

  public static final short NB_ROWS_HUB_PRC_ELLIPTIC_CURVE = 4;

  final CommonPrecompileOobCall oobCall;

  public EllipticCurvePrecompileSubsection(Hub hub, CallSection callSection) {
    super(hub, callSection);

    final BigInteger calleeGas =
        BigInteger.valueOf(callSection.stpCall.effectiveChildContextGasAllowance());
    final OobCall call =
        switch (flag()) {
          case PRC_ECRECOVER -> new EcRecoverOobCall(calleeGas);
          case PRC_ECADD -> new EcAddOobCall(calleeGas);
          case PRC_ECMUL -> new EcMulOobCall(calleeGas);
          case PRC_ECPAIRING -> new EcPairingOobCall(calleeGas);
          case PRC_POINT_EVALUATION -> new BlsPointEvaluationOobCall(calleeGas);
          case PRC_BLS_G1_ADD -> new BlsG1AddOobCall(calleeGas);
          case PRC_BLS_G1_MSM -> new BlsG1MsmOobCall(calleeGas);
          case PRC_BLS_G2_ADD -> new BlsG2AddOobCall(calleeGas);
          case PRC_BLS_G2_MSM -> new BlsG2MsmOobCall(calleeGas);
          case PRC_BLS_PAIRING_CHECK -> new BlsPairingCheckOobCall(calleeGas);
          case PRC_BLS_MAP_FP_TO_G1 -> new BlsMapFpToG1OobCall(calleeGas);
          case PRC_BLS_MAP_FP2_TO_G2 -> new BlsMapFp2ToG2OobCall(calleeGas);
          case PRC_P256_VERIFY -> new P256VerifyOobCall(calleeGas);
          default ->
              throw new IllegalArgumentException(
                  String.format(
                      "Precompile address %s not supported by constructor",
                      this.flag().toString()));
        };

    oobCall = (CommonPrecompileOobCall) firstImcFragment.callOob(call);

    // Recall that the default scenario is PRC_SUCCESS_WONT_REVERT
    if (!oobCall.isHubSuccess()) {
      this.setScenario(PRC_FAILURE_KNOWN_TO_HUB);
    }
  }

  @Override
  public void resolveAtContextReEntry(Hub hub, CallFrame callFrame) {
    super.resolveAtContextReEntry(hub, callFrame);

    final Bytes returnData = extractReturnData();

    // sanity checks
    switch (flag()) {
      case PRC_ECRECOVER -> {
        checkArgument(oobCall.isHubSuccess() == callSuccess, "ECRECOVER hub success mismatch");
        checkArgument(
            callSuccess
                ? (returnData == Bytes.EMPTY || returnData.size() == WORD_SIZE)
                : returnData == Bytes.EMPTY,
            "ECRECOVER return data size mismatch");
      }
      case PRC_ECPAIRING ->
          checkArgument(
              returnDataRange.extract().size() == (callSuccess ? WORD_SIZE : 0),
              "ECPAIRING return data size mismatch");
      case PRC_ECADD, PRC_ECMUL ->
          checkArgument(
              returnDataRange.extract().size() == (callSuccess ? 2 * WORD_SIZE : 0),
              "%s return data size mismatch",
              flag());
      case PRC_POINT_EVALUATION,
          PRC_BLS_G1_ADD,
          PRC_BLS_G1_MSM,
          PRC_BLS_G2_ADD,
          PRC_BLS_G2_MSM,
          PRC_BLS_PAIRING_CHECK,
          PRC_BLS_MAP_FP_TO_G1,
          PRC_BLS_MAP_FP2_TO_G2 -> {
        // Note that BLS sanity checks are computed in BlsOperation
      }
      // For PRC_P256_VERIFY, the return data is either empty:
      // - callSuccess = false
      //    * Insufficient gas (the only scenario where the underlying call fails, callSuccess =
      // false)
      // - callSuccess = true
      //    * Invalid input length (not exactly 160 bytes)
      //    * Invalid field element encoding (≥ field modulus)
      //    * Invalid signature component bounds (r or s not in range (0, n))
      //    * Invalid public key (point at infinity or not on curve)
      //    * Signature verification failure
      // or non-empty, of size 32 bytes when none of the failure conditions above hold.
      case PRC_P256_VERIFY -> {
        checkArgument(
            returnData == Bytes.EMPTY || returnData.size() == WORD_SIZE,
            "P256_VERIFY return data size mismatch");
        // TODO: get the actual check using the MMU success bit
        checkArgument(
            returnData == Bytes.EMPTY || oobCall.getExtractCallData(),
            "Unless we extract call data, return date must be empty");
      }
      default -> throw new IllegalArgumentException("Not an elliptic curve precompile");
    }

    if (!oobCall.isHubSuccess()) {
      return;
    }

    // NOTE: from here on out
    //    hubSuccess ≡ true

    // ECRECOVER and P256_VERIFY can only be FAILURE_KNOWN_TO_HUB or some form of
    // SUCCESS_XXXX_REVERT
    // In particular, neither may be PRC_FAILURE_KNOWN_TO_RAM
    if (flag().isAnyOf(PRC_ECADD, PRC_ECMUL, PRC_ECPAIRING) || flag().isBlsPrecompile()) {
      if (oobCall.isHubSuccess() && !callSuccess) {
        precompileScenarioFragment.scenario(PRC_FAILURE_KNOWN_TO_RAM);
      }
    }

    final MmuCall firstMmuCall;
    final boolean triggerCallDataExtraction = oobCall.getExtractCallData();

    // P256_VERIFY call data extraction only happens if hubSuccess = true AND
    // callDataSize is correct, that is 160 bytes.
    // Note that hubSuccess is equivalent to having sufficient gas
    // and does not take into account call data size check.
    if (flag() == PRC_P256_VERIFY) {
      final boolean triggerCallDataExtractionP256Verify =
          callDataSize() == PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY;
      Preconditions.checkArgument(triggerCallDataExtractionP256Verify == triggerCallDataExtraction);
    }

    // BLS precompiles do not accept empty call data
    // This checks should be redundant with OOB checks
    if (flag().isBlsPrecompile()) {
      Preconditions.checkArgument(
          triggerCallDataExtraction, "BLS precompile %s called with empty call data", flag());
    }

    final boolean successBitMmuCall =
        switch (flag()) {
          case PRC_ECRECOVER, PRC_P256_VERIFY -> !returnData.isEmpty();
          default -> callSuccess;
        };

    if (triggerCallDataExtraction) {
      switch (flag()) {
        case PRC_ECRECOVER ->
            firstMmuCall = MmuCall.callDataExtractionForEcrecover(hub, this, successBitMmuCall);
        case PRC_ECADD ->
            firstMmuCall = MmuCall.callDataExtractionForEcadd(hub, this, successBitMmuCall);
        case PRC_ECMUL ->
            firstMmuCall = MmuCall.callDataExtractionForEcmul(hub, this, successBitMmuCall);
        case PRC_ECPAIRING ->
            firstMmuCall = MmuCall.callDataExtractionForEcpairing(hub, this, successBitMmuCall);
        case PRC_POINT_EVALUATION,
            PRC_BLS_G1_ADD,
            PRC_BLS_G1_MSM,
            PRC_BLS_G2_ADD,
            PRC_BLS_G2_MSM,
            PRC_BLS_PAIRING_CHECK,
            PRC_BLS_MAP_FP_TO_G1,
            PRC_BLS_MAP_FP2_TO_G2,
            PRC_P256_VERIFY ->
            firstMmuCall =
                MmuCall.callDataExtractionForPostCancunPrecompiles(hub, this, successBitMmuCall);
        default -> throw new IllegalArgumentException("Not an elliptic curve precompile");
      }
      firstImcFragment.callMmu(firstMmuCall);

      if (flag().isEcdataPrecompile()) {
        hub.ecData.callEcData(exoModuleOperationId(), flag(), extractCallData(), returnData);
      } else if (flag().isBlsPrecompile()) {
        ((BlsData) hub.blsData())
            .callBls(
                exoModuleOperationId(), flag(), extractCallData(), returnData, successBitMmuCall);
      }
    }

    if (!callSuccess) return;

    final ImcFragment secondImcFragment = ImcFragment.empty(hub);
    this.fragments().add(secondImcFragment);

    final ImcFragment thirdImcFragment = ImcFragment.empty(hub);
    this.fragments().add(thirdImcFragment);

    MmuCall secondMmuCall = null;
    MmuCall thirdMmuCall = null;
    final boolean producesNonemptyReturnData = !returnData.isEmpty();
    final boolean callerMayReceiveReturnData = !getReturnAtRange().isEmpty();

    if (producesNonemptyReturnData) {
      switch (flag()) {
        case PRC_ECRECOVER -> {
          secondMmuCall = MmuCall.fullReturnDataTransferForEcrecover(hub, this, successBitMmuCall);
          if (callerMayReceiveReturnData) {
            thirdMmuCall = MmuCall.partialReturnDataCopyForEcrecover(hub, this);
          }
        }
        case PRC_ECADD -> {
          if (triggerCallDataExtraction) {
            secondMmuCall = MmuCall.fullTransferOfReturnDataForEcadd(hub, this, successBitMmuCall);
          }
          if (callerMayReceiveReturnData) {
            thirdMmuCall = MmuCall.partialCopyOfReturnDataForEcadd(hub, this);
          }
        }
        case PRC_ECMUL -> {
          if (triggerCallDataExtraction) {
            secondMmuCall = MmuCall.fullReturnDataTransferForEcmul(hub, this, successBitMmuCall);
          }
          if (callerMayReceiveReturnData) {
            thirdMmuCall = MmuCall.partialCopyOfReturnDataForEcmul(hub, this);
          }
        }
        case PRC_ECPAIRING -> {
          secondMmuCall = MmuCall.fullReturnDataTransferForEcpairing(hub, this, successBitMmuCall);
          if (callerMayReceiveReturnData) {
            thirdMmuCall = MmuCall.partialCopyOfReturnDataForEcpairing(hub, this);
          }
        }
        case PRC_POINT_EVALUATION,
            PRC_BLS_G1_ADD,
            PRC_BLS_G1_MSM,
            PRC_BLS_G2_ADD,
            PRC_BLS_G2_MSM,
            PRC_BLS_PAIRING_CHECK,
            PRC_BLS_MAP_FP_TO_G1,
            PRC_BLS_MAP_FP2_TO_G2,
            PRC_P256_VERIFY -> {
          // Note that for BLS precompiles nonemptyCallData is always true at this point
          secondMmuCall =
              MmuCall.fullReturnDataTransferForPostCancunPrecompiles(hub, this, successBitMmuCall);
          if (callerMayReceiveReturnData) {
            thirdMmuCall = MmuCall.partialCopyOfReturnDataForPostCancunPrecompiles(hub, this);
          }
        }
        default -> throw new IllegalArgumentException("Not an elliptic curve precompile");
      }
      if (secondMmuCall != null) {
        secondImcFragment.callMmu(secondMmuCall);
      }
      if (thirdMmuCall != null) {
        thirdImcFragment.callMmu(thirdMmuCall);
      }
    }
  }

  // 4 = 1 + 3 (scenario row + 3 miscellaneous fragments)
  @Override
  protected short maxNumberOfLines() {
    return NB_ROWS_HUB_PRC_ELLIPTIC_CURVE;
    // Note: we don't have the successBit available at the moment
    // and can't provide the "real" value (2 in case of FKTH.)
  }
}
