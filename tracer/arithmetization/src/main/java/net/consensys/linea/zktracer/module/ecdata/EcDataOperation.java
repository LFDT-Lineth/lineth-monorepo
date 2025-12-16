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

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.Trace.Ecdata.CT_MAX_LARGE_POINT;
import static net.consensys.linea.zktracer.Trace.Ecdata.CT_MAX_SMALL_POINT;
import static net.consensys.linea.zktracer.Trace.Ecdata.INDEX_MAX_ECADD_DATA;
import static net.consensys.linea.zktracer.Trace.Ecdata.INDEX_MAX_ECADD_RESULT;
import static net.consensys.linea.zktracer.Trace.Ecdata.INDEX_MAX_ECMUL_DATA;
import static net.consensys.linea.zktracer.Trace.Ecdata.INDEX_MAX_ECMUL_RESULT;
import static net.consensys.linea.zktracer.Trace.Ecdata.INDEX_MAX_ECPAIRING_DATA_MIN;
import static net.consensys.linea.zktracer.Trace.Ecdata.INDEX_MAX_ECPAIRING_RESULT;
import static net.consensys.linea.zktracer.Trace.Ecdata.INDEX_MAX_ECRECOVER_DATA;
import static net.consensys.linea.zktracer.Trace.Ecdata.INDEX_MAX_ECRECOVER_RESULT;
import static net.consensys.linea.zktracer.Trace.Ecdata.P_BN_HI;
import static net.consensys.linea.zktracer.Trace.Ecdata.P_BN_LO;
import static net.consensys.linea.zktracer.Trace.Ecdata.SECP256K1N_HI;
import static net.consensys.linea.zktracer.Trace.Ecdata.SECP256K1N_LO;
import static net.consensys.linea.zktracer.Trace.Ecdata.TOTAL_SIZE_ECADD_DATA;
import static net.consensys.linea.zktracer.Trace.Ecdata.TOTAL_SIZE_ECADD_RESULT;
import static net.consensys.linea.zktracer.Trace.Ecdata.TOTAL_SIZE_ECMUL_DATA;
import static net.consensys.linea.zktracer.Trace.Ecdata.TOTAL_SIZE_ECMUL_RESULT;
import static net.consensys.linea.zktracer.Trace.Ecdata.TOTAL_SIZE_ECPAIRING_DATA_MIN;
import static net.consensys.linea.zktracer.Trace.Ecdata.TOTAL_SIZE_ECPAIRING_RESULT;
import static net.consensys.linea.zktracer.Trace.Ecdata.TOTAL_SIZE_ECRECOVER_DATA;
import static net.consensys.linea.zktracer.Trace.Ecdata.TOTAL_SIZE_ECRECOVER_RESULT;
import static net.consensys.linea.zktracer.Trace.PHASE_ECADD_DATA;
import static net.consensys.linea.zktracer.Trace.PHASE_ECADD_RESULT;
import static net.consensys.linea.zktracer.Trace.PHASE_ECMUL_DATA;
import static net.consensys.linea.zktracer.Trace.PHASE_ECMUL_RESULT;
import static net.consensys.linea.zktracer.Trace.PHASE_ECPAIRING_DATA;
import static net.consensys.linea.zktracer.Trace.PHASE_ECPAIRING_RESULT;
import static net.consensys.linea.zktracer.Trace.PHASE_ECRECOVER_DATA;
import static net.consensys.linea.zktracer.Trace.PHASE_ECRECOVER_RESULT;
import static net.consensys.linea.zktracer.Trace.PHASE_P256_VERIFY_DATA;
import static net.consensys.linea.zktracer.Trace.PHASE_P256_VERIFY_RESULT;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.A_COEFF_R1_HI;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.A_COEFF_R1_LO;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.B_COEFF_R1_HI;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.B_COEFF_R1_LO;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.INDEX_MAX_P256_VERIFY_DATA;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.INDEX_MAX_P256_VERIFY_RESULT;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.P_R1_HI;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.P_R1_LO;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.SECP256R1N_HI;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.SECP256R1N_LO;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.TOTAL_SIZE_P256_VERIFY_DATA;
import static net.consensys.linea.zktracer.TraceOsaka.Ecdata.TOTAL_SIZE_P256_VERIFY_RESULT;
import static net.consensys.linea.zktracer.module.Util.rightPaddedSlice;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.*;
import static net.consensys.linea.zktracer.types.Containers.repeat;
import static net.consensys.linea.zktracer.types.Conversions.booleanToBytes;
import static net.consensys.linea.zktracer.types.EWord.ZERO;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;

import java.util.List;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.ext.Ext;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.commons.lang3.tuple.Pair;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.altbn128.AltBn128Fq2Point;
import org.hyperledger.besu.crypto.altbn128.Fq2;

@Accessors(fluent = true)
public class EcDataOperation extends ModuleOperation {
  private static final EWord P_BN = EWord.of(P_BN_HI, P_BN_LO);
  public static final EWord SECP256K1N = EWord.of(SECP256K1N_HI, SECP256K1N_LO);
  public static final int nBYTES_OF_DELTA_BYTES = 4;

  public static final EWord P_R1 = EWord.of(P_R1_HI, P_R1_LO);
  public static final EWord SECP256R1N = EWord.of(SECP256R1N_HI, SECP256R1N_LO);

  public static final EWord A_COEFF_R1 = EWord.of(A_COEFF_R1_HI, A_COEFF_R1_LO);
  public static final EWord B_COEFF_R1 = EWord.of(B_COEFF_R1_HI, B_COEFF_R1_LO);

  @Getter private final Bytes returnData;

  private final Wcp wcp;
  private final Ext ext;

  @Getter private final long id;
  private final Bytes rightPaddedCallData;

  @Getter private final PrecompileScenarioFragment.PrecompileFlag precompileFlag;
  private final int nRows;
  private final int nRowsData;
  private final int nRowsResult;

  @Getter private final List<Bytes> limb;
  private final List<Boolean> hurdle;
  @Getter private boolean internalChecksPassed;

  // WCP interaction
  private final List<Boolean> wcpFlag;
  private final List<Bytes> wcpArg1Hi;
  private final List<Bytes> wcpArg1Lo;
  private final List<Bytes> wcpArg2Hi;
  private final List<Bytes> wcpArg2Lo;
  private final List<Boolean> wcpRes;
  private final List<OpCode> wcpInst;

  // EXT interaction
  private final List<Boolean> extFlag;
  private final List<Bytes> extArg1Hi;
  private final List<Bytes> extArg1Lo;
  private final List<Bytes> extArg2Hi;
  private final List<Bytes> extArg2Lo;
  private final List<Bytes> extArg3Hi;
  private final List<Bytes> extArg3Lo;
  private final List<Bytes> extResHi;
  private final List<Bytes> extResLo;
  private final List<OpCode> extInst;

  @Getter private boolean successBit;

  // pairing-specific
  @Getter private final int totalPairings;
  @Getter private int circuitSelectorEcPairingCounter = 0;
  @Getter private int circuitSelectorG2MembershipCounter = 0;

  private final List<Boolean> notOnG2; // counter-constant
  private final List<Boolean> notOnG2Acc; // counter-constant
  @Getter private boolean notOnG2AccMax; // index-constant
  private final List<Boolean> isInfinity; // counter-constant
  @Getter private final List<Boolean> overallTrivialPairing; // counter-constant

  private EcDataOperation(
      Wcp wcp,
      Ext ext,
      int id,
      final PrecompileScenarioFragment.PrecompileFlag precompileFlag,
      Bytes callData,
      Bytes returnData) {
    checkArgument(
        precompileFlag.isEcdataPrecompile(),
        "EcDataOperation: precompile %s isn't of EC_DATA type",
        precompileFlag);

    this.precompileFlag = precompileFlag;
    final int callDataSize = callData.size();
    checkArgument(
        callDataSize > 0, "EcDataOperation should only be called with nonempty call data");
    final int paddedCallDataLength =
        switch (precompileFlag) {
          case PRC_ECRECOVER -> TOTAL_SIZE_ECRECOVER_DATA;
          case PRC_ECADD -> TOTAL_SIZE_ECADD_DATA;
          case PRC_ECMUL -> TOTAL_SIZE_ECMUL_DATA;
          case PRC_ECPAIRING -> {
            checkArgument(
                callDataSize % TOTAL_SIZE_ECPAIRING_DATA_MIN == 0,
                "ECPAIRING: call data size expected to be multiple of %s, but remainder is %s ≠ 0",
                TOTAL_SIZE_ECPAIRING_DATA_MIN,
                callDataSize % TOTAL_SIZE_ECPAIRING_DATA_MIN);
            yield callDataSize;
          }
          case PRC_P256_VERIFY -> TOTAL_SIZE_P256_VERIFY_DATA;
          default ->
              throw new IllegalArgumentException(
                  "EcDataOperation expects to be called on an elliptic curve precompile, not on "
                      + precompileFlag.name());
        };

    rightPaddedCallData = rightPaddedSlice(callData, 0, paddedCallDataLength);

    if (precompileFlag == PRC_ECPAIRING) {
      totalPairings = callDataSize / TOTAL_SIZE_ECPAIRING_DATA_MIN;
    } else {
      totalPairings = 0;
    }

    nRowsData = getIndexMax(precompileFlag, true) + 1;
    nRowsResult = getIndexMax(precompileFlag, false) + 1;
    nRows = nRowsData + nRowsResult;
    this.id = id;

    limb = repeat(Bytes.EMPTY, nRows);
    hurdle = repeat(false, nRows);

    wcpFlag = repeat(false, nRows);
    wcpArg1Hi = repeat(Bytes.EMPTY, nRows);
    wcpArg1Lo = repeat(Bytes.EMPTY, nRows);
    wcpArg2Hi = repeat(Bytes.EMPTY, nRows);
    wcpArg2Lo = repeat(Bytes.EMPTY, nRows);
    wcpRes = repeat(false, nRows);
    wcpInst = repeat(OpCode.INVALID, nRows);

    extFlag = repeat(false, nRows);
    extArg1Hi = repeat(Bytes.EMPTY, nRows);
    extArg1Lo = repeat(Bytes.EMPTY, nRows);
    extArg2Hi = repeat(Bytes.EMPTY, nRows);
    extArg2Lo = repeat(Bytes.EMPTY, nRows);
    extArg3Hi = repeat(Bytes.EMPTY, nRows);
    extArg3Lo = repeat(Bytes.EMPTY, nRows);
    extResHi = repeat(Bytes.EMPTY, nRows);
    extResLo = repeat(Bytes.EMPTY, nRows);
    extInst = repeat(OpCode.INVALID, nRows);

    this.wcp = wcp;
    this.ext = ext;

    isInfinity = repeat(false, nRows);
    overallTrivialPairing = repeat(true, nRows);
    notOnG2 = repeat(false, nRows);
    notOnG2Acc = repeat(false, nRows);

    // Set returnData
    this.returnData = returnData;
  }

  public static EcDataOperation of(
      Wcp wcp,
      Ext ext,
      int id,
      final PrecompileScenarioFragment.PrecompileFlag precompileFlag,
      Bytes callData,
      Bytes returnData) {
    final EcDataOperation ecDataOperation =
        new EcDataOperation(wcp, ext, id, precompileFlag, callData, returnData);
    switch (precompileFlag) {
      case PRC_ECRECOVER -> ecDataOperation.handleRecover();
      case PRC_ECADD -> ecDataOperation.handleAdd();
      case PRC_ECMUL -> ecDataOperation.handleMul();
      case PRC_ECPAIRING -> ecDataOperation.handlePairing();
      case PRC_P256_VERIFY -> ecDataOperation.handleP256Verify();
    }
    return ecDataOperation;
  }

  private int getTotalSize(
      PrecompileScenarioFragment.PrecompileFlag precompileFlag, boolean isData) {
    if (isData) {
      return switch (precompileFlag) {
        case PRC_ECRECOVER -> TOTAL_SIZE_ECRECOVER_DATA;
        case PRC_ECADD -> TOTAL_SIZE_ECADD_DATA;
        case PRC_ECMUL -> TOTAL_SIZE_ECMUL_DATA;
        case PRC_ECPAIRING -> TOTAL_SIZE_ECPAIRING_DATA_MIN * totalPairings;
        case PRC_P256_VERIFY -> TOTAL_SIZE_P256_VERIFY_DATA;
        default -> throw new IllegalArgumentException("invalid EC type");
      };
    } else {
      return switch (precompileFlag) {
        case PRC_ECRECOVER -> successBit ? TOTAL_SIZE_ECRECOVER_RESULT : 0;
        case PRC_ECADD -> successBit ? TOTAL_SIZE_ECADD_RESULT : 0;
        case PRC_ECMUL -> successBit ? TOTAL_SIZE_ECMUL_RESULT : 0;
        case PRC_ECPAIRING -> successBit ? TOTAL_SIZE_ECPAIRING_RESULT : 0;
        case PRC_P256_VERIFY -> successBit ? TOTAL_SIZE_P256_VERIFY_RESULT : 0;
        default -> throw new IllegalArgumentException("invalid EC type");
      };
    }
  }

  private static short getPhase(
      PrecompileScenarioFragment.PrecompileFlag precompileFlag, boolean isData) {
    if (isData) {
      return switch (precompileFlag) {
        case PRC_ECRECOVER -> PHASE_ECRECOVER_DATA;
        case PRC_ECADD -> PHASE_ECADD_DATA;
        case PRC_ECMUL -> PHASE_ECMUL_DATA;
        case PRC_ECPAIRING -> PHASE_ECPAIRING_DATA;
        case PRC_P256_VERIFY -> PHASE_P256_VERIFY_DATA;
        default -> throw new IllegalArgumentException("invalid EC type");
      };
    } else {
      return switch (precompileFlag) {
        case PRC_ECRECOVER -> PHASE_ECRECOVER_RESULT;
        case PRC_ECADD -> PHASE_ECADD_RESULT;
        case PRC_ECMUL -> PHASE_ECMUL_RESULT;
        case PRC_ECPAIRING -> PHASE_ECPAIRING_RESULT;
        case PRC_P256_VERIFY -> PHASE_P256_VERIFY_RESULT;
        default -> throw new IllegalArgumentException("invalid EC type");
      };
    }
  }

  private int getIndexMax(
      PrecompileScenarioFragment.PrecompileFlag precompileFlag, boolean isData) {
    if (isData) {
      return switch (precompileFlag) {
        case PRC_ECRECOVER -> INDEX_MAX_ECRECOVER_DATA;
        case PRC_ECADD -> INDEX_MAX_ECADD_DATA;
        case PRC_ECMUL -> INDEX_MAX_ECMUL_DATA;
        case PRC_ECPAIRING -> (INDEX_MAX_ECPAIRING_DATA_MIN + 1) * totalPairings - 1;
        case PRC_P256_VERIFY -> INDEX_MAX_P256_VERIFY_DATA;
        default -> throw new IllegalArgumentException("invalid EC type");
      };
    } else {
      return switch (precompileFlag) {
        case PRC_ECRECOVER -> INDEX_MAX_ECRECOVER_RESULT;
        case PRC_ECADD -> INDEX_MAX_ECADD_RESULT;
        case PRC_ECMUL -> INDEX_MAX_ECMUL_RESULT;
        case PRC_ECPAIRING -> INDEX_MAX_ECPAIRING_RESULT;
        case PRC_P256_VERIFY -> INDEX_MAX_P256_VERIFY_RESULT;
        default -> throw new IllegalArgumentException("invalid EC type");
      };
    }
  }

  private boolean callWCPISZERO(int i, EWord arg) {
    return callWcp(i, OpCode.ISZERO, arg, ZERO);
  }

  private boolean callWcp(int i, OpCode wcpInst, EWord arg1, EWord arg2) {
    final boolean wcpRes =
        switch (wcpInst) {
          case LT -> wcp.callLT(arg1, arg2);
          case EQ -> wcp.callEQ(arg1, arg2);
          case ISZERO -> wcp.callISZERO(arg1);
          default -> throw new IllegalStateException("Unexpected value: " + wcpInst);
        };

    wcpFlag.set(i, true);
    wcpArg1Hi.set(i, arg1.hi());
    wcpArg1Lo.set(i, arg1.lo());
    wcpArg2Hi.set(i, arg2.hi());
    wcpArg2Lo.set(i, arg2.lo());
    this.wcpRes.set(i, wcpRes);
    this.wcpInst.set(i, wcpInst);
    return wcpRes;
  }

  private EWord callExt(int i, OpCode extInst, EWord arg1, EWord arg2, EWord arg3) {
    final EWord extRes = EWord.of(ext.call(extInst, arg1, arg2, arg3));

    extFlag.set(i, true);
    extArg1Hi.set(i, arg1.hi());
    extArg1Lo.set(i, arg1.lo());
    extArg2Hi.set(i, arg2.hi());
    extArg2Lo.set(i, arg2.lo());
    extArg3Hi.set(i, arg3.hi());
    extArg3Lo.set(i, arg3.lo());
    extResHi.set(i, extRes.hi());
    extResLo.set(i, extRes.lo());
    this.extInst.set(i, extInst);
    return extRes;
  }

  void handleRecover() {
    // Extract inputs
    final EWord h = EWord.of(rightPaddedCallData.slice(0, 32));
    final EWord v = EWord.of(rightPaddedCallData.slice(32, 32));
    final EWord r = EWord.of(rightPaddedCallData.slice(64, 32));
    final EWord s = EWord.of(rightPaddedCallData.slice(96, 32));

    // Set input limb
    limb.set(0, h.hi());
    limb.set(1, h.lo());
    limb.set(2, v.hi());
    limb.set(3, v.lo());
    limb.set(4, r.hi());
    limb.set(5, r.lo());
    limb.set(6, s.hi());
    limb.set(7, s.lo());

    // Compute internal checks
    // row i
    boolean rIsInRange = callWcp(0, OpCode.LT, r, SECP256K1N); // r < secp256k1N

    // row i + 1
    boolean rIsPositive = !callWCPISZERO(1, r); // 0 < r

    // row i + 2
    boolean sIsInRange = callWcp(2, OpCode.LT, s, SECP256K1N); // s < secp256k1N

    // row i + 3
    boolean sIsPositive = !callWCPISZERO(3, s); // 0 < s

    // row i+ 4
    boolean vIs27 = callWcp(4, OpCode.EQ, v, EWord.of(27)); // v == 27

    // row i + 5
    boolean vIs28 = callWcp(5, OpCode.EQ, v, EWord.of(28)); // v == 28

    // Set hurdle
    hurdle.set(0, rIsInRange && rIsPositive);
    hurdle.set(1, sIsInRange && sIsPositive);
    hurdle.set(2, hurdle.get(0) && hurdle.get(1));
    hurdle.set(INDEX_MAX_ECRECOVER_DATA, hurdle.get(2) && (vIs27 || vIs28));

    // Set internal checks passed
    internalChecksPassed = hurdle.get(INDEX_MAX_ECRECOVER_DATA);

    // Success bit is set in setReturnData

    // Set result rows
    EWord recoveredAddress = ZERO;

    // Extract output
    if (internalChecksPassed) {
      recoveredAddress = EWord.of(returnData);
    }

    // Set success bit and output limb
    successBit = !recoveredAddress.isZero();
    limb.set(8, recoveredAddress.hi());
    limb.set(9, recoveredAddress.lo());
  }

  void handleAdd() {
    // Extract inputs
    final EWord pX = EWord.of(rightPaddedCallData.slice(0, 32));
    final EWord pY = EWord.of(rightPaddedCallData.slice(32, 32));
    final EWord qX = EWord.of(rightPaddedCallData.slice(64, 32));
    final EWord qY = EWord.of(rightPaddedCallData.slice(96, 32));

    // Set limb
    limb.set(0, pX.hi());
    limb.set(1, pX.lo());
    limb.set(2, pY.hi());
    limb.set(3, pY.lo());
    limb.set(4, qX.hi());
    limb.set(5, qX.lo());
    limb.set(6, qY.hi());
    limb.set(7, qY.lo());

    // Compute internal checks
    // row i
    final boolean c1MembershipFirstPoint = callToC1Membership(0, pX, pY).getLeft();

    // row i + 4
    final boolean c1MembershipSecondPoint = callToC1Membership(4, qX, qY).getLeft();

    // Complete set hurdle
    hurdle.set(INDEX_MAX_ECADD_DATA, c1MembershipFirstPoint && c1MembershipSecondPoint);

    // Set intenral checks passed
    internalChecksPassed = hurdle.get(INDEX_MAX_ECADD_DATA);

    // Success bit is set in setReturnData

    // Set result rows
    EWord resX = ZERO;
    EWord resY = ZERO;

    // Extract output
    if (internalChecksPassed && returnData.toArray().length != 0) {
      checkArgument(
          returnData.toArray().length == 64,
          "ECADD: return data size %s should be 64",
          returnData.toArray().length);
      resX = EWord.of(returnData.slice(0, 32));
      resY = EWord.of(returnData.slice(32, 32));
    }

    // Set success bit and output limb
    successBit = internalChecksPassed;
    limb.set(8, resX.hi());
    limb.set(9, resX.lo());
    limb.set(10, resY.hi());
    limb.set(11, resY.lo());
  }

  void handleMul() {
    // Extract inputs
    final EWord pX = EWord.of(rightPaddedCallData.slice(0, 32));
    final EWord pY = EWord.of(rightPaddedCallData.slice(32, 32));
    final EWord n = EWord.of(rightPaddedCallData.slice(64, 32));

    // Set limb
    limb.set(0, pX.hi());
    limb.set(1, pX.lo());
    limb.set(2, pY.hi());
    limb.set(3, pY.lo());
    limb.set(4, n.hi());
    limb.set(5, n.lo());

    // Compute internal checks
    // row i
    boolean c1Membership = callToC1Membership(0, pX, pY).getLeft();

    // Complete set hurdle
    hurdle.set(INDEX_MAX_ECMUL_DATA, c1Membership);

    // Set intenral checks passed
    internalChecksPassed = hurdle.get(INDEX_MAX_ECMUL_DATA);

    // Success bit is set in setReturnData

    // Set result rows
    EWord resX = ZERO;
    EWord resY = ZERO;

    // Extract output
    if (internalChecksPassed && returnData.toArray().length != 0) {
      checkArgument(
          returnData.toArray().length == 64,
          "ECMUL: return data size %s should be 64",
          returnData.toArray().length);
      resX = EWord.of(returnData.slice(0, 32));
      resY = EWord.of(returnData.slice(32, 32));
    }

    // Set success bit and output limb
    successBit = internalChecksPassed;
    limb.set(6, resX.hi());
    limb.set(7, resX.lo());
    limb.set(8, resY.hi());
    limb.set(9, resY.lo());
  }

  void handlePairing() {
    boolean atLeastOneLargePointIsNotInfinity = false;
    boolean firstLargePointNotAtInfinity = false;
    boolean atLeastOneLargePointIsNotOnG2 = false;
    boolean firstLargePointNotOnG2 = false;

    for (int accPairings = 1; accPairings <= totalPairings; accPairings++) {
      // Extract inputs
      final int bytesOffset = (accPairings - 1) * TOTAL_SIZE_ECPAIRING_DATA_MIN;
      final EWord aX = EWord.of(rightPaddedCallData.slice(bytesOffset, 32));
      final EWord aY = EWord.of(rightPaddedCallData.slice(32 + bytesOffset, 32));
      final EWord bXIm = EWord.of(rightPaddedCallData.slice(64 + bytesOffset, 32));
      final EWord bXRe = EWord.of(rightPaddedCallData.slice(96 + bytesOffset, 32));
      final EWord bYIm = EWord.of(rightPaddedCallData.slice(128 + bytesOffset, 32));
      final EWord bYRe = EWord.of(rightPaddedCallData.slice(160 + bytesOffset, 32));

      // Set limb
      final int rowsOffset = (accPairings - 1) * (INDEX_MAX_ECPAIRING_DATA_MIN + 1); // 12
      limb.set(rowsOffset, aX.hi());
      limb.set(1 + rowsOffset, aX.lo());
      limb.set(2 + rowsOffset, aY.hi());
      limb.set(3 + rowsOffset, aY.lo());
      limb.set(4 + rowsOffset, bXIm.hi());
      limb.set(5 + rowsOffset, bXIm.lo());
      limb.set(6 + rowsOffset, bXRe.hi());
      limb.set(7 + rowsOffset, bXRe.lo());
      limb.set(8 + rowsOffset, bYIm.hi());
      limb.set(9 + rowsOffset, bYIm.lo());
      limb.set(10 + rowsOffset, bYRe.hi());
      limb.set(11 + rowsOffset, bYRe.lo());

      // Compute internal checks
      // row i
      final Pair<Boolean, Boolean> callToC1MembershipReturnedValues =
          callToC1Membership(rowsOffset, aX, aY);
      final boolean c1Membership = callToC1MembershipReturnedValues.getLeft();

      // row i + 4
      final Pair<Boolean, Boolean> callToWellFormedCoordinatesReturnedValues =
          callToWellFormedCoordinates(4 + rowsOffset, bXIm, bXRe, bYIm, bYRe);
      final boolean wellFormedCoordinates = callToWellFormedCoordinatesReturnedValues.getLeft();
      final boolean largePointIsAtInfinity = callToWellFormedCoordinatesReturnedValues.getRight();

      // Check if the large point is on G2
      final Fq2 bX = Fq2.create(bXRe.toUnsignedBigInteger(), bXIm.toUnsignedBigInteger());
      final Fq2 bY = Fq2.create(bYRe.toUnsignedBigInteger(), bYIm.toUnsignedBigInteger());
      final AltBn128Fq2Point b = new AltBn128Fq2Point(bX, bY);
      if (!atLeastOneLargePointIsNotOnG2 && (!b.isOnCurve() || !b.isInGroup())) {
        atLeastOneLargePointIsNotOnG2 = true;
        firstLargePointNotOnG2 = true;
        notOnG2AccMax = true;
      }

      // Set isInfinity, overallTrivialPairing, notOnG2, notOnG2Acc
      if (!largePointIsAtInfinity && !atLeastOneLargePointIsNotInfinity) {
        atLeastOneLargePointIsNotInfinity = true;
        firstLargePointNotAtInfinity = true;
      }

      for (int i = 0; i <= INDEX_MAX_ECPAIRING_DATA_MIN; i++) {
        if (firstLargePointNotAtInfinity) {
          if (i > CT_MAX_SMALL_POINT) {
            // Transition should happen at the beginning of large point
            overallTrivialPairing.set(i + rowsOffset, false);
            // Propagate overallTrivialPairing to result rows as soon as the pairing is identified
            // as non-trivial
            overallTrivialPairing.set(overallTrivialPairing.size() - 1, false);
            overallTrivialPairing.set(overallTrivialPairing.size() - 2, false);
          }
        } else {
          overallTrivialPairing.set(i + rowsOffset, !atLeastOneLargePointIsNotInfinity);
        }

        if (firstLargePointNotOnG2) {
          if (i > CT_MAX_SMALL_POINT) {
            // Transition should happen at the beginning of large point
            notOnG2.set(i + rowsOffset, true);
            notOnG2Acc.set(i + rowsOffset, true);
          }
        } else {
          notOnG2Acc.set(i + rowsOffset, atLeastOneLargePointIsNotOnG2);
        }
      }

      // Set firstLargePointNotAtInfinity back to false
      firstLargePointNotAtInfinity = false;

      // Set firstLargePointNotOnG2 back to false
      firstLargePointNotOnG2 = false;

      // Set hurdle and internal checks passed
      if (accPairings == 1) {
        internalChecksPassed = c1Membership && wellFormedCoordinates;

        hurdle.set(INDEX_MAX_ECPAIRING_DATA_MIN, internalChecksPassed);
      } else {
        final boolean prevInternalChecksPassed = internalChecksPassed;
        internalChecksPassed = c1Membership && wellFormedCoordinates && prevInternalChecksPassed;

        hurdle.set(
            INDEX_MAX_ECPAIRING_DATA_MIN - 1 + rowsOffset, c1Membership && wellFormedCoordinates);
        hurdle.set(INDEX_MAX_ECPAIRING_DATA_MIN + rowsOffset, internalChecksPassed);
      }
    }

    // This is after all pairings have been processed
    notOnG2AccMax = internalChecksPassed && notOnG2AccMax;

    // Set counters
    if (internalChecksPassed) {
      if (notOnG2AccMax) {
        circuitSelectorEcPairingCounter = 0;
        circuitSelectorG2MembershipCounter = 1;
      } else {
        if (!isOverallTrivialPairing()) {
          for (int accPairings = 1; accPairings <= totalPairings; accPairings++) {
            final int rowsOffset = (accPairings - 1) * (INDEX_MAX_ECPAIRING_DATA_MIN + 1);
            final boolean smallPointIsAtInfinity = isInfinity.get(rowsOffset);
            final boolean largePointIsAtInfinity =
                isInfinity.get(rowsOffset + CT_MAX_SMALL_POINT + 1);
            if (!largePointIsAtInfinity) {
              if (!smallPointIsAtInfinity) {
                circuitSelectorEcPairingCounter++;
              } else {
                circuitSelectorG2MembershipCounter++;
              }
            }
          }
        }
      }
    }

    // Set result rows
    EWord pairingResult = ZERO;

    // Extract output
    if (internalChecksPassed) {
      pairingResult = EWord.of(returnData);
    }

    // Set output limb
    limb.set(limb.size() - 2, pairingResult.hi());
    limb.set(limb.size() - 1, pairingResult.lo());

    // Set callSuccess
    if (!internalChecksPassed) {
      successBit = false;
    } else {
      successBit = !notOnG2AccMax;
    }

    // acceptablePairOfPointsForPairingCircuit, g2MembershipTestRequired are set in the trace method
  }

  private void handleP256Verify() {
    // Extract inputs
    final EWord h = EWord.of(rightPaddedCallData.slice(0, 32));
    final EWord r = EWord.of(rightPaddedCallData.slice(32, 32));
    final EWord s = EWord.of(rightPaddedCallData.slice(64, 32));
    final EWord qX = EWord.of(rightPaddedCallData.slice(96, 32));
    final EWord qY = EWord.of(rightPaddedCallData.slice(128, 32));

    // Set input limb
    limb.set(0, h.hi());
    limb.set(1, h.lo());
    limb.set(2, r.hi());
    limb.set(3, r.lo());
    limb.set(4, s.hi());
    limb.set(5, s.lo());
    limb.set(6, qX.hi());
    limb.set(7, qX.lo());
    limb.set(8, qY.hi());
    limb.set(9, qY.lo());

    // Compute internal checks
    // row i
    boolean rIsInRange = callWcp(0, OpCode.LT, r, SECP256R1N); // r < secp256r1N

    // row i + 1
    boolean rIsPositive = !callWCPISZERO(1, r); // 0 < r

    // row i + 2
    boolean sIsInRange = callWcp(2, OpCode.LT, s, SECP256R1N); // s < secp256r1N

    // row i + 3
    boolean sIsPositive = !callWCPISZERO(3, s); // 0 < s

    // row i+ 4
    boolean r1Membership = callToR1Membership(4, qX, qY).getLeft();

    // Set hurdle
    hurdle.set(0, rIsInRange && rIsPositive);
    hurdle.set(1, sIsInRange && sIsPositive);
    hurdle.set(2, hurdle.get(0) && hurdle.get(1));
    hurdle.set(INDEX_MAX_P256_VERIFY_DATA, r1Membership && hurdle.get(2));

    // Set internal checks passed
    internalChecksPassed = hurdle.get(INDEX_MAX_P256_VERIFY_DATA);

    // Set success bit and output limb
    successBit = returnData.toArray().length == TOTAL_SIZE_P256_VERIFY_RESULT;
    limb.set(10, Bytes.EMPTY);
    limb.set(11, booleanToBytes(successBit));
  }

  void trace(Trace.Ecdata trace, final int stamp, final long previousId) {
    final Bytes deltaByte =
        leftPadTo(Bytes.minimalBytes(id - previousId - 1), nBYTES_OF_DELTA_BYTES);

    // Part of the columns are computed here
    int ct = 0;
    boolean isSmallPoint = false;
    boolean isLargePoint = false;
    boolean smallPointIsAtInfinity = false;
    boolean largePointIsAtInfinity = false;

    for (int i = 0; i < nRows; i++) {
      boolean isData = i < nRowsData;
      // Turn isSmallPoint on if we are in the first row of a new pairing
      if (precompileFlag == PRC_ECPAIRING && isData && ct == 0 && !isSmallPoint && !isLargePoint) {
        isSmallPoint = true;
        smallPointIsAtInfinity = isInfinity.get(i);
        largePointIsAtInfinity = isInfinity.get(i + CT_MAX_SMALL_POINT + 1);
      }

      boolean notOnG2AccMax =
          precompileFlag == PRC_ECPAIRING
              && isData
              && this.notOnG2AccMax; // && conditions is necessary since we want IS_ECPAIRING_DATA
      // = 1
      // && conditions is necessary since we want IS_ECPAIRING_DATA
      // We care about G2 membership only if ICP = 1
      final boolean g2MembershipTestRequired =
          isData
              && (notOnG2AccMax
                  ? isLargePoint && !largePointIsAtInfinity && notOnG2.get(i)
                  : isLargePoint && !largePointIsAtInfinity && smallPointIsAtInfinity)
              && internalChecksPassed;
      final boolean acceptablePairOfPointsForPairingCircuit =
          precompileFlag == PRC_ECPAIRING
              && isData
              && successBit
              && !notOnG2AccMax
              && !largePointIsAtInfinity
              && !smallPointIsAtInfinity;

      if (precompileFlag != PRC_ECPAIRING || !isData) {
        checkArgument(
            ct == 0,
            "EcDataOperation's trace method: ct should be 0 except for ECPAIRING data rows, yet prc = %s, isData = %s, ct = %s",
            precompileFlag,
            isData,
            ct);
      }
      checkArgument(
          !(isSmallPoint && isLargePoint),
          "EcDataOperation's trace: ECPAIRING data simultaneously categorized as small and large point");

      trace
          .stamp(stamp)
          .id(id)
          .index(isData ? i : i - nRowsData)
          .limb(limb.get(i))
          .totalSize(getTotalSize(precompileFlag, isData))
          .phase(getPhase(precompileFlag, isData))
          .indexMax(getIndexMax(precompileFlag, isData))
          .successBit(successBit)
          .isEcrecoverData(precompileFlag == PRC_ECRECOVER && isData)
          .isEcrecoverResult(precompileFlag == PRC_ECRECOVER && !isData)
          .isEcaddData(precompileFlag == PRC_ECADD && isData)
          .isEcaddResult(precompileFlag == PRC_ECADD && !isData)
          .isEcmulData(precompileFlag == PRC_ECMUL && isData)
          .isEcmulResult(precompileFlag == PRC_ECMUL && !isData)
          .isEcpairingData(precompileFlag == PRC_ECPAIRING && isData)
          .isEcpairingResult(precompileFlag == PRC_ECPAIRING && !isData);
      if (precompileFlag == PRC_P256_VERIFY) {
        // this is safe: fillAndValidate row fills these automatically when the precompile
        // isn't P256_VERIFY
        trace.isP256VerifyData(isData).isP256VerifyResult(!isData);
      }
      trace
          .totalPairings(totalPairings)
          .accPairings(
              precompileFlag == PRC_ECPAIRING && isData
                  ? 1 + i / (INDEX_MAX_ECPAIRING_DATA_MIN + 1)
                  : 0)
          .internalChecksPassed(internalChecksPassed)
          .hurdle(hurdle.get(i))
          .byteDelta(
              i < nBYTES_OF_DELTA_BYTES ? UnsignedByte.of(deltaByte.get(i)) : UnsignedByte.of(0))
          .ct((short) ct)
          .ctMax(
              (short) (isSmallPoint ? CT_MAX_SMALL_POINT : (isLargePoint ? CT_MAX_LARGE_POINT : 0)))
          .isSmallPoint(isSmallPoint)
          .isLargePoint(isLargePoint)
          .notOnG2(
              notOnG2.get(i) && internalChecksPassed) // We care about G2 membership only if ICP = 1
          .notOnG2Acc(
              notOnG2Acc.get(i)
                  && internalChecksPassed) // We care about G2 membership only if ICP = 1
          .notOnG2AccMax(notOnG2AccMax)
          .isInfinity(isInfinity.get(i))
          .overallTrivialPairing(
              precompileFlag == PRC_ECPAIRING
                  && overallTrivialPairing.get(
                      i)) // && conditions necessary because default value is true
          .g2MembershipTestRequired(g2MembershipTestRequired)
          .acceptablePairOfPointsForPairingCircuit(acceptablePairOfPointsForPairingCircuit)
          .wcpFlag(wcpFlag.get(i))
          .wcpArg1Hi(wcpArg1Hi.get(i))
          .wcpArg1Lo(wcpArg1Lo.get(i))
          .wcpArg2Hi(wcpArg2Hi.get(i))
          .wcpArg2Lo(wcpArg2Lo.get(i))
          .wcpRes(wcpRes.get(i))
          .wcpInst(wcpInst.get(i).unsignedByteValue())
          .extFlag(extFlag.get(i))
          .extArg1Hi(extArg1Hi.get(i))
          .extArg1Lo(extArg1Lo.get(i))
          .extArg2Hi(extArg2Hi.get(i))
          .extArg2Lo(extArg2Lo.get(i))
          .extArg3Hi(extArg3Hi.get(i))
          .extArg3Lo(extArg3Lo.get(i))
          .extResHi(extResHi.get(i))
          .extResLo(extResLo.get(i))
          .extInst(extInst.get(i).unsignedByteValue())
          .fillAndValidateRow();

      // Update ct, isSmallPoint, isLargePoint
      if (precompileFlag == PRC_ECPAIRING && isData) {
        ct++;
        if (isSmallPoint && ct == CT_MAX_SMALL_POINT + 1) {
          isSmallPoint = false;
          isLargePoint = true;
          ct = 0;
        } else if (isLargePoint && ct == CT_MAX_LARGE_POINT + 1) {
          isLargePoint = false;
          ct = 0;
          smallPointIsAtInfinity = false;
          largePointIsAtInfinity = false;
        }
      }
    }
  }

  @Override
  protected int computeLineCount() {
    return nRowsData + nRowsResult;
  }

  private Pair<Boolean, Boolean> callToC1Membership(int k, EWord pX, EWord pY) {
    // EXT
    EWord pYSquare = callExt(k, OpCode.MULMOD, pY, pY, P_BN);
    EWord pXSquare = callExt(k + 1, OpCode.MULMOD, pX, pX, P_BN);
    EWord pXCube = callExt(k + 2, OpCode.MULMOD, pXSquare, pX, P_BN);
    EWord pXCubePlus3 = callExt(k + 3, OpCode.ADDMOD, pXCube, EWord.of(3), P_BN);

    // WCP
    boolean pXIsInRange = callWcp(k, OpCode.LT, pX, P_BN);
    boolean pYIsInRange = callWcp(k + 1, OpCode.LT, pY, P_BN);
    boolean pSatisfiesCubic = callWcp(k + 2, OpCode.EQ, pYSquare, pXCubePlus3);

    // Set hurdle
    boolean pIsRange = pXIsInRange && pYIsInRange;
    boolean pIsPointAtInfinity = pIsRange && pX.isZero() && pY.isZero();
    boolean c1Membership = pIsRange && (pIsPointAtInfinity || pSatisfiesCubic);
    hurdle.set(k + 1, pIsRange);
    hurdle.set(k, c1Membership);

    // Set isInfinity
    for (int i = 0; i <= CT_MAX_SMALL_POINT; i++) {
      isInfinity.set(i + k, pIsPointAtInfinity);
    }

    return Pair.of(c1Membership, pIsPointAtInfinity);
  }

  private Pair<Boolean, Boolean> callToR1Membership(int k, EWord pX, EWord pY) {
    // EXT
    EWord pYSquare = callExt(k, OpCode.MULMOD, pY, pY, P_R1);
    EWord pXSquare = callExt(k + 1, OpCode.MULMOD, pX, pX, P_R1);
    EWord pXCube = callExt(k + 2, OpCode.MULMOD, pXSquare, pX, P_R1);
    EWord aTimesPx = callExt(k + 3, OpCode.MULMOD, A_COEFF_R1, pX, P_R1);
    EWord pXCubePlusAPx = callExt(k + 4, OpCode.ADDMOD, pXCube, aTimesPx, P_R1);
    EWord pXCubePlusAPxPlusB = callExt(k + 5, OpCode.ADDMOD, pXCubePlusAPx, B_COEFF_R1, P_R1);

    // WCP
    boolean pXIsInRange = callWcp(k, OpCode.LT, pX, P_R1);
    boolean pYIsInRange = callWcp(k + 1, OpCode.LT, pY, P_R1);
    boolean pSatisfiesCubic = callWcp(k + 2, OpCode.EQ, pYSquare, pXCubePlusAPxPlusB);

    // Set hurdle
    boolean pIsRange = pXIsInRange && pYIsInRange;
    boolean pIsPointAtInfinity = pIsRange && pX.isZero() && pY.isZero();
    boolean r1Membership = pIsRange && !pIsPointAtInfinity && pSatisfiesCubic;
    hurdle.set(k + 1, pIsRange);
    hurdle.set(k, r1Membership);

    // Set isInfinity
    for (int i = 0; i <= CT_MAX_SMALL_POINT; i++) {
      isInfinity.set(i + k, pIsPointAtInfinity);
    }

    return Pair.of(r1Membership, pIsPointAtInfinity);
  }

  private Pair<Boolean, Boolean> callToWellFormedCoordinates(
      int k, EWord bXIm, EWord bXRe, EWord bYIm, EWord bYRe) {
    // WCP
    boolean bXImIsInRange = callWcp(k, OpCode.LT, bXIm, P_BN);
    boolean bXReIsInRange = callWcp(k + 1, OpCode.LT, bXRe, P_BN);
    boolean bYImIsInRange = callWcp(k + 2, OpCode.LT, bYIm, P_BN);
    boolean bYReIsInRange = callWcp(k + 3, OpCode.LT, bYRe, P_BN);

    // Set hurdle
    boolean bXIsRange = bXImIsInRange && bXReIsInRange;
    boolean bYIsRange = bYImIsInRange && bYReIsInRange;
    boolean wellFormedCoordinates = bXIsRange && bYIsRange;
    boolean bIsPointAtInfinity =
        wellFormedCoordinates && bXIm.isZero() && bXRe.isZero() && bYIm.isZero() && bYRe.isZero();
    hurdle.set(k + 2, bXIsRange);
    hurdle.set(k + 1, bYIsRange);
    hurdle.set(k, wellFormedCoordinates);

    // Set isInfinity
    for (int i = 0; i <= CT_MAX_LARGE_POINT; i++) {
      isInfinity.set(i + k, bIsPointAtInfinity);
    }

    return Pair.of(wellFormedCoordinates, bIsPointAtInfinity);
  }

  boolean isOverallTrivialPairing() {
    return overallTrivialPairing.getLast();
  }
}
