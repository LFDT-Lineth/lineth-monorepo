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

package net.consensys.linea.zktracer.module.blsdata;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.Blsdata.BLS_PRIME_0;
import static net.consensys.linea.zktracer.Trace.Blsdata.BLS_PRIME_1;
import static net.consensys.linea.zktracer.Trace.Blsdata.BLS_PRIME_2;
import static net.consensys.linea.zktracer.Trace.Blsdata.BLS_PRIME_3;
import static net.consensys.linea.zktracer.Trace.Blsdata.CT_MAX_LARGE_POINT;
import static net.consensys.linea.zktracer.Trace.Blsdata.CT_MAX_MAP_FP2_TO_G2;
import static net.consensys.linea.zktracer.Trace.Blsdata.CT_MAX_MAP_FP_TO_G1;
import static net.consensys.linea.zktracer.Trace.Blsdata.CT_MAX_POINT_EVALUATION;
import static net.consensys.linea.zktracer.Trace.Blsdata.CT_MAX_SCALAR;
import static net.consensys.linea.zktracer.Trace.Blsdata.CT_MAX_SMALL_POINT;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_DATA_G1_ADD;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_DATA_G1_MSM_MIN;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_DATA_G2_ADD;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_DATA_G2_MSM_MIN;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_DATA_MAP_FP2_TO_G2;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_DATA_MAP_FP_TO_G1;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_DATA_PAIRING_CHECK_MIN;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_DATA_POINT_EVALUATION;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_RSLT_G1_ADD;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_RSLT_G1_MSM;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_RSLT_G2_ADD;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_RSLT_G2_MSM;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_RSLT_MAP_FP2_TO_G2;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_RSLT_MAP_FP_TO_G1;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_RSLT_PAIRING_CHECK;
import static net.consensys.linea.zktracer.Trace.Blsdata.INDEX_MAX_RSLT_POINT_EVALUATION;
import static net.consensys.linea.zktracer.Trace.Blsdata.POINT_EVALUATION_PRIME_HI;
import static net.consensys.linea.zktracer.Trace.Blsdata.POINT_EVALUATION_PRIME_LO;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G1_ADD;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G1_MSM;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G2_ADD;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_G2_MSM;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_MAP_FP2_TO_G2;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_MAP_FP_TO_G1;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_BLS_PAIRING_CHECK;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment.PrecompileFlag.PRC_POINT_EVALUATION;
import static net.consensys.linea.zktracer.types.Containers.repeat;
import static net.consensys.linea.zktracer.types.Conversions.ZERO;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;

import com.google.common.base.Preconditions;
import java.util.List;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.module.hub.fragment.scenario.PrecompileScenarioFragment;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.nativelib.gnark.LibGnarkEIP2537;

@Accessors(fluent = true)
public class BlsDataOperation extends ModuleOperation {
  static final EWord POINT_EVALUATION_PRIME =
      EWord.of(POINT_EVALUATION_PRIME_HI, POINT_EVALUATION_PRIME_LO);

  public static final int nBYTES_OF_DELTA_BYTES = 4;
  private static final int SIZE_SMALL_POINT = LLARGE * (CT_MAX_SMALL_POINT + 1);
  private static final int SIZE_LARGE_POINT = LLARGE * (CT_MAX_LARGE_POINT + 1);
  private static final int SIZE_SCALAR = LLARGE * (CT_MAX_SCALAR + 1);

  private final Wcp wcp;

  private final Bytes callData;
  private final Bytes returnData;

  @Getter private final PrecompileScenarioFragment.PrecompileFlag precompileFlag;
  private final int nRows;
  private final int nRowsData;
  private final int nRowsResult;

  @Getter private final long id;
  private final int totalSizeData;
  private final int totalSizeResult;
  @Getter private final boolean successBit;

  private final List<Boolean> mintBit;
  private final List<Boolean> mextBit;
  private final List<Boolean> isInfinity;
  private final List<Boolean> nontrivialPairOfPointsBit;

  @Getter private boolean malformedDataInternal;
  @Getter private boolean malformedDataExternal;
  @Getter private boolean wellformedDataTrivial;
  @Getter private boolean wellformedDataNonTrivial;
  @Getter private boolean firstPointNotInSubgroupIsSmall;
  @Getter private int nontrivialPopCounter;
  @Getter private int trivialPopDueToG2PointCounter; // Counting trivial pairs of the form (P,inf)
  @Getter private int trivialPopDueToG1PointCounter; // Counting trivial pairs of the form (inf,Q)

  // WCP interaction
  private final List<Boolean> wcpFlag;
  private final List<Bytes> wcpArg1Hi;
  private final List<Bytes> wcpArg1Lo;
  private final List<Bytes> wcpArg2Hi;
  private final List<Bytes> wcpArg2Lo;
  private final List<Boolean> wcpRes;
  private final List<OpCode> wcpInst;

  private BlsDataOperation(
      Wcp wcp,
      int id,
      final PrecompileScenarioFragment.PrecompileFlag precompileFlag,
      Bytes callData,
      Bytes returnData,
      boolean successBit) {
    checkArgument(
        precompileFlag.isBlsPrecompile(),
        "BlsDataOperation: precompile %s isn't of BLS type",
        precompileFlag);
    checkArgument(!callData.isEmpty(), "BlsDataOperation: callData is empty");

    this.precompileFlag = precompileFlag;
    this.callData = callData;
    totalSizeData = callData.size();
    totalSizeResult = returnData.size();

    nRowsData = getIndexMax(precompileFlag, true) + 1;
    nRowsResult = getIndexMax(precompileFlag, false) + 1;
    nRows = nRowsData + nRowsResult;
    this.id = id;

    mintBit = repeat(false, nRows);
    mextBit = repeat(false, nRows);
    isInfinity = repeat(false, nRows);
    nontrivialPairOfPointsBit = repeat(false, nRows);

    wcpFlag = repeat(false, nRows);
    wcpArg1Hi = repeat(Bytes.EMPTY, nRows);
    wcpArg1Lo = repeat(Bytes.EMPTY, nRows);
    wcpArg2Hi = repeat(Bytes.EMPTY, nRows);
    wcpArg2Lo = repeat(Bytes.EMPTY, nRows);
    wcpRes = repeat(false, nRows);
    wcpInst = repeat(OpCode.INVALID, nRows);

    this.wcp = wcp;

    // Set returnData
    this.returnData = returnData;

    // Set successBit
    this.successBit = successBit;
    final int returnDataSize = returnData.toArray().length;
    final int expectedReturnDataSize = (successBit ? expectedReturnDataSize(precompileFlag) : 0);
    Preconditions.checkArgument(
        returnDataSize == expectedReturnDataSize,
        "BLS precompile return data size %s does not agree with the expected value %s",
        returnDataSize,
        expectedReturnDataSize);
  }

  public static int expectedReturnDataSize(
      final PrecompileScenarioFragment.PrecompileFlag precompileFlag) {
    return switch (precompileFlag) {
          case PRC_POINT_EVALUATION -> INDEX_MAX_RSLT_POINT_EVALUATION + 1;
          case PRC_BLS_G1_ADD -> INDEX_MAX_RSLT_G1_ADD + 1;
          case PRC_BLS_G1_MSM -> INDEX_MAX_RSLT_G1_MSM + 1;
          case PRC_BLS_G2_ADD -> INDEX_MAX_RSLT_G2_ADD + 1;
          case PRC_BLS_G2_MSM -> INDEX_MAX_RSLT_G2_MSM + 1;
          case PRC_BLS_PAIRING_CHECK -> INDEX_MAX_RSLT_PAIRING_CHECK + 1;
          case PRC_BLS_MAP_FP_TO_G1 -> INDEX_MAX_RSLT_MAP_FP_TO_G1 + 1;
          case PRC_BLS_MAP_FP2_TO_G2 -> INDEX_MAX_RSLT_MAP_FP2_TO_G2 + 1;
          default -> throw new IllegalStateException("Unexpected value: " + precompileFlag);
        }
        * 16;
  }

  public static BlsDataOperation of(
      Wcp wcp,
      int id,
      final PrecompileScenarioFragment.PrecompileFlag precompileFlag,
      Bytes callData,
      Bytes returnData,
      boolean successBit) {
    BlsDataOperation blsDataOperation =
        new BlsDataOperation(wcp, id, precompileFlag, callData, returnData, successBit);
    switch (precompileFlag) {
      case PRC_POINT_EVALUATION -> blsDataOperation.handlePointEvaluation();
      case PRC_BLS_G1_ADD -> blsDataOperation.handleBlsG1Add();
      case PRC_BLS_G1_MSM -> blsDataOperation.handleBlsG1Msm();
      case PRC_BLS_G2_ADD -> blsDataOperation.handleBlsG2Add();
      case PRC_BLS_G2_MSM -> blsDataOperation.handleBlsG2Msm();
      case PRC_BLS_PAIRING_CHECK -> blsDataOperation.handleBlsPairingCheck();
      case PRC_BLS_MAP_FP_TO_G1 -> blsDataOperation.handleBlsMapFpToG1();
      case PRC_BLS_MAP_FP2_TO_G2 -> blsDataOperation.handleBlsMapFp2ToG2();
      default ->
          throw new IllegalArgumentException(
              "BlsOperation expects to be called on a bls precompile, not on "
                  + precompileFlag.name());
    }
    blsDataOperation.handleGlobalColumns();
    return blsDataOperation;
  }

  private void handleGlobalColumns() {
    malformedDataInternal = mintBit.stream().reduce(false, Boolean::logicalOr);
    malformedDataExternal = mextBit.stream().reduce(false, Boolean::logicalOr);
    final boolean nonTrivialPairOfPointsTot =
        nontrivialPairOfPointsBit.stream().reduce(false, Boolean::logicalOr);
    wellformedDataTrivial =
        !malformedDataInternal
            && !malformedDataExternal
            && precompileFlag == PRC_BLS_PAIRING_CHECK
            && !nonTrivialPairOfPointsTot;
    wellformedDataNonTrivial =
        !malformedDataInternal
            && !malformedDataExternal
            && (precompileFlag != PRC_BLS_PAIRING_CHECK || nonTrivialPairOfPointsTot);
  }

  private void handlePointEvaluation() {
    // Extract inputs
    final EWord z = EWord.of(callData.slice(WORD_SIZE, WORD_SIZE));
    final EWord y = EWord.of(callData.slice(2 * WORD_SIZE, WORD_SIZE));

    final boolean zIsInRange = wcpCallToLT(0, z, POINT_EVALUATION_PRIME);

    final boolean yIsInRange = wcpCallToLT(1, y, POINT_EVALUATION_PRIME);

    final boolean internalChecksPassed = zIsInRange && yIsInRange;

    final boolean mextBit = internalChecksPassed && !successBit;

    for (int j = 0; j <= CT_MAX_POINT_EVALUATION; j++) {
      this.mintBit.set(j, !internalChecksPassed);
      this.mextBit.set(j, mextBit);
    }
  }

  private void handleBlsG1Add() {
    boolean mextBitIsSet = false;

    for (int k = 0; k < 2; k++) {
      final int sizeOffset = k * SIZE_SMALL_POINT;
      final int indexOffset = k * (CT_MAX_SMALL_POINT + 1);

      final boolean wellFormedCoordinate =
          wellFormedFpCoordinate(indexOffset, callData.slice(sizeOffset, SIZE_SMALL_POINT));
      final boolean isSmallPointOnCurve =
          isSmallPointOnCurve(indexOffset, callData.slice(sizeOffset, SIZE_SMALL_POINT));
      final boolean mextBit = wellFormedCoordinate && !isSmallPointOnCurve;

      if (mextBit && !mextBitIsSet) {
        for (int j = 0; j <= CT_MAX_SMALL_POINT; j++) {
          this.mextBit.set(indexOffset + j, true);
        }
        mextBitIsSet = true;
      }
    }
  }

  private void handleBlsG1Msm() {
    boolean mextBitIsSet = false;

    final int numberOfInputs = callData.size() / (SIZE_SMALL_POINT + SIZE_SCALAR);
    for (int k = 0; k < numberOfInputs; k++) {
      final int sizeOffset = k * (SIZE_SMALL_POINT + SIZE_SCALAR);
      final int indexOffset = k * (CT_MAX_SMALL_POINT + 1 + CT_MAX_SCALAR + 1);

      final boolean wellFormedCoordinate =
          wellFormedFpCoordinate(indexOffset, callData.slice(sizeOffset, SIZE_SMALL_POINT));
      final boolean isSmallPointInSubgroup =
          isSmallPointInSubGroup(indexOffset, callData.slice(sizeOffset, SIZE_SMALL_POINT));
      final boolean mextBit = wellFormedCoordinate && !isSmallPointInSubgroup;

      if (mextBit && !mextBitIsSet) {
        for (int j = 0; j <= CT_MAX_SMALL_POINT; j++) {
          this.mextBit.set(indexOffset + j, true);
        }
        mextBitIsSet = true;
      }
    }
  }

  private void handleBlsG2Add() {
    boolean mextBitIsSet = false;

    for (int k = 0; k < 2; k++) {
      final int sizeOffset = k * SIZE_LARGE_POINT;
      final int indexOffset = k * (CT_MAX_LARGE_POINT + 1);

      // Extract inputs
      final boolean wellFormedCoordinate =
          wellFormedFp2Coordinate(indexOffset, callData.slice(sizeOffset, SIZE_LARGE_POINT));

      final boolean isLargePointOnCurve =
          isLargePointOnCurve(indexOffset, callData.slice(sizeOffset, SIZE_LARGE_POINT));
      final boolean mextBit = wellFormedCoordinate && !isLargePointOnCurve;

      if (mextBit && !mextBitIsSet) {
        for (int j = 0; j <= CT_MAX_LARGE_POINT; j++) {
          this.mextBit.set(indexOffset + j, true);
        }
        mextBitIsSet = true;
      }
    }
  }

  private void handleBlsG2Msm() {
    boolean mextBitIsSet = false;

    final int numberOfInputs = callData.size() / (SIZE_LARGE_POINT + SIZE_SCALAR);
    for (int k = 0; k < numberOfInputs; k++) {
      final int sizeOffset = k * (SIZE_LARGE_POINT + SIZE_SCALAR);
      final int indexOffset = k * (CT_MAX_LARGE_POINT + 1 + CT_MAX_SCALAR + 1);

      final boolean wellFormedCoordinate =
          wellFormedFp2Coordinate(indexOffset, callData.slice(sizeOffset, SIZE_LARGE_POINT));

      final boolean isLargePointInSubgroup =
          isLargePointInSubGroup(indexOffset, callData.slice(sizeOffset, SIZE_LARGE_POINT));
      final boolean mextBit = wellFormedCoordinate && !isLargePointInSubgroup;

      if (mextBit && !mextBitIsSet) {
        for (int j = 0; j <= CT_MAX_LARGE_POINT; j++) {
          this.mextBit.set(indexOffset + j, true);
        }
        mextBitIsSet = true;
      }
    }
  }

  private void handleBlsPairingCheck() {
    boolean mextBitIsSet = false;

    final int numberOfInputs = callData.size() / (SIZE_SMALL_POINT + SIZE_LARGE_POINT);
    for (int k = 0; k < numberOfInputs; k++) {
      final int sizeOffset = k * (SIZE_SMALL_POINT + SIZE_LARGE_POINT);
      final int indexOffset = k * (CT_MAX_SMALL_POINT + 1 + CT_MAX_LARGE_POINT + 1);

      final boolean wellFormedFpCoordinate =
          wellFormedFpCoordinate(indexOffset, callData.slice(sizeOffset, SIZE_SMALL_POINT));
      final boolean isSmallPointInSubgroup =
          isSmallPointInSubGroup(indexOffset, callData.slice(sizeOffset, SIZE_SMALL_POINT));
      final boolean mextBitSmall = wellFormedFpCoordinate && !isSmallPointInSubgroup;

      if (mextBitSmall && !mextBitIsSet) {
        for (int j = 0; j <= CT_MAX_SMALL_POINT; j++) {
          this.mextBit.set(indexOffset + j, true);
        }
        mextBitIsSet = true;
        firstPointNotInSubgroupIsSmall = true;
      }

      final boolean wellFormedFp2Coordinate =
          wellFormedFp2Coordinate(
              8 + indexOffset, callData.slice(8 * LLARGE + sizeOffset, SIZE_LARGE_POINT));

      final boolean isLargePointInSubgroup =
          isLargePointInSubGroup(
              8 + indexOffset, callData.slice(8 * LLARGE + sizeOffset, SIZE_LARGE_POINT));
      final boolean mextBitLarge = wellFormedFp2Coordinate && !isLargePointInSubgroup;

      if (mextBitLarge && !mextBitIsSet) {
        for (int j = 0; j <= CT_MAX_LARGE_POINT; j++) {
          this.mextBit.set(8 + indexOffset + j, true);
        }
        mextBitIsSet = true;
      }

      final boolean smallPointIsAtInfinity = isInfinity.get(indexOffset);
      final boolean largePointIsAtInfinity = isInfinity.get(8 + indexOffset);
      final boolean pairOfPointsNonTrivialBit = !smallPointIsAtInfinity && !largePointIsAtInfinity;
      if (pairOfPointsNonTrivialBit) {
        nontrivialPopCounter++;
      }
      if (!smallPointIsAtInfinity && largePointIsAtInfinity) {
        trivialPopDueToG2PointCounter++;
      }
      if (smallPointIsAtInfinity && !largePointIsAtInfinity) {
        trivialPopDueToG1PointCounter++;
      }
      for (int j = indexOffset; j < 24 + indexOffset; j++) {
        this.nontrivialPairOfPointsBit.set(j, pairOfPointsNonTrivialBit);
      }
    }
  }

  private void handleBlsMapFpToG1() {
    // Extract inputs
    final Bytes e = callData.slice(0, 4 * LLARGE);

    final boolean internalChecksPassed = callToLTBlsPrime(0, e); // eIsInRange;

    for (int j = 0; j <= CT_MAX_MAP_FP_TO_G1; j++) {
      this.mintBit.set(j, !internalChecksPassed);
    }
  }

  private void handleBlsMapFp2ToG2() {
    // Extract inputs
    final Bytes eRe = callData.slice(0, 4 * LLARGE);
    final Bytes eIm = callData.slice(4 * LLARGE, 4 * LLARGE);

    final boolean eReIsInRange = callToLTBlsPrime(0, eRe);

    final boolean eImIsInRange = callToLTBlsPrime(4, eIm);

    final boolean internalChecksPassed = eReIsInRange && eImIsInRange;

    for (int j = 0; j <= CT_MAX_MAP_FP2_TO_G2; j++) {
      this.mintBit.set(j, !internalChecksPassed);
    }
  }

  private boolean isSmallPointOnCurve(int i, Bytes smallPoint) {
    final boolean isInfinity = isInfinity(i, smallPoint);
    if (isInfinity) {
      return true;
    }

    byte[] input = smallPoint.toArray();
    byte[] error = new byte[256];
    return LibGnarkEIP2537.eip2537G1IsOnCurve(
        smallPoint.toArray(), error, input.length, error.length);
  }

  // Note: this checks also if the point is on curve
  private boolean isSmallPointInSubGroup(int i, Bytes smallPoint) {
    final boolean isOnCurve = isSmallPointOnCurve(i, smallPoint);
    if (!isOnCurve) {
      return false;
    }

    byte[] input = smallPoint.toArray();
    byte[] error = new byte[256];
    return LibGnarkEIP2537.eip2537G1IsInSubGroup(input, error, input.length, error.length);
  }

  private boolean isLargePointOnCurve(int i, Bytes largePoint) {
    final boolean isInfinity = isInfinity(i, largePoint);
    if (isInfinity) {
      return true;
    }

    byte[] input = largePoint.toArray();
    byte[] error = new byte[256];
    return LibGnarkEIP2537.eip2537G2IsOnCurve(input, error, input.length, error.length);
  }

  // Note: this checks also if the point is on curve
  private boolean isLargePointInSubGroup(int i, Bytes largePoint) {
    final boolean isOnCurve = isLargePointOnCurve(i, largePoint);
    if (!isOnCurve) {
      return false;
    }

    byte[] input = largePoint.toArray();
    byte[] error = new byte[256];
    return LibGnarkEIP2537.eip2537G2IsInSubGroup(input, error, input.length, error.length);
  }

  private int getIndexMax(
      PrecompileScenarioFragment.PrecompileFlag precompileFlag, boolean isData) {
    if (isData) {
      return switch (precompileFlag) {
        case PRC_POINT_EVALUATION -> INDEX_MAX_DATA_POINT_EVALUATION;
        case PRC_BLS_G1_ADD -> INDEX_MAX_DATA_G1_ADD;
        case PRC_BLS_G2_ADD -> INDEX_MAX_DATA_G2_ADD;
        case PRC_BLS_MAP_FP_TO_G1 -> INDEX_MAX_DATA_MAP_FP_TO_G1;
        case PRC_BLS_MAP_FP2_TO_G2 -> INDEX_MAX_DATA_MAP_FP2_TO_G2;
        case PRC_BLS_G1_MSM, PRC_BLS_G2_MSM, PRC_BLS_PAIRING_CHECK -> totalSizeData / 16 - 1;
        default -> throw new IllegalStateException("invalid BLS type");
      };
    } else {
      return switch (precompileFlag) {
        case PRC_POINT_EVALUATION -> INDEX_MAX_RSLT_POINT_EVALUATION;
        case PRC_BLS_G1_ADD -> INDEX_MAX_RSLT_G1_ADD;
        case PRC_BLS_G1_MSM -> INDEX_MAX_RSLT_G1_MSM;
        case PRC_BLS_G2_ADD -> INDEX_MAX_RSLT_G2_ADD;
        case PRC_BLS_G2_MSM -> INDEX_MAX_RSLT_G2_MSM;
        case PRC_BLS_PAIRING_CHECK -> INDEX_MAX_RSLT_PAIRING_CHECK;
        case PRC_BLS_MAP_FP_TO_G1 -> INDEX_MAX_RSLT_MAP_FP_TO_G1;
        case PRC_BLS_MAP_FP2_TO_G2 -> INDEX_MAX_RSLT_MAP_FP2_TO_G2;
        default -> throw new IllegalStateException("invalid BLS type");
      };
    }
  }

  // Utilities
  private boolean wcpCallTo(int i, OpCode wcpInst, EWord arg1, EWord arg2) {
    final boolean wcpRes =
        switch (wcpInst) {
          case LT -> wcp.callLT(arg1, arg2);
          case EQ -> wcp.callEQ(arg1, arg2);
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

  private boolean wcpCallToLT(int i, EWord arg1, EWord arg2) {
    return wcpCallTo(i, OpCode.LT, arg1, arg2);
  }

  private boolean wcpCallToEQ(int i, EWord arg1, EWord arg2) {
    return wcpCallTo(i, OpCode.EQ, arg1, arg2);
  }

  private boolean wcpGeneralizedCallToLT(
      int i,
      EWord firstArgumentHi,
      EWord firstArgumentLo,
      EWord secondArgumentHi,
      EWord secondArgumentLo) {
    wcpCallToLT(i + 1, firstArgumentHi, secondArgumentHi);
    wcpCallToEQ(i + 2, firstArgumentHi, secondArgumentHi);
    wcpCallToLT(i + 3, firstArgumentLo, secondArgumentLo);

    final boolean wcpRes =
        this.wcpRes.get(i + 1) || (this.wcpRes.get(i + 2) && this.wcpRes.get(i + 3));
    this.wcpRes.set(i, wcpRes); // TODO: do we want to set other WCP columns here?

    return wcpRes;
  }

  // This is defined here for convenience, but not appearing in the specs
  private boolean callToLTBlsPrime(int i, Bytes e) {
    return wcpGeneralizedCallToLT(
        i,
        EWord.of(e.slice(0, 2 * LLARGE)),
        EWord.of(e.slice(2 * LLARGE, 2 * LLARGE)),
        EWord.of(Bytes.ofUnsignedShort(BLS_PRIME_3), bigIntegerToBytes(BLS_PRIME_2)),
        EWord.of(bigIntegerToBytes(BLS_PRIME_1), bigIntegerToBytes(BLS_PRIME_0)));
  }

  private boolean wellFormedFpCoordinate(int i, Bytes smallPoint) {
    final Bytes pX = smallPoint.slice(0, 4 * LLARGE);
    final Bytes pY = smallPoint.slice(4 * LLARGE, 4 * LLARGE);
    final boolean pXIsInRange = callToLTBlsPrime(i, pX);
    final boolean pYIsInRange = callToLTBlsPrime(i + 4, pY);

    final boolean wellFormedCoordinate = pXIsInRange && pYIsInRange;

    for (int j = 0; j <= CT_MAX_SMALL_POINT; j++) {
      this.mintBit.set(i + j, !wellFormedCoordinate);
    }

    return wellFormedCoordinate;
  }

  private boolean wellFormedFp2Coordinate(int i, Bytes largePoint) {
    final Bytes pXRe = largePoint.slice(0, 4 * LLARGE);
    final Bytes pXIm = largePoint.slice(4 * LLARGE, 4 * LLARGE);
    final Bytes pYRe = largePoint.slice(8 * LLARGE, 4 * LLARGE);
    final Bytes pYIm = largePoint.slice(12 * LLARGE, 4 * LLARGE);
    final boolean pXReIsInRange = callToLTBlsPrime(i, pXRe);
    final boolean pXImIsInRange = callToLTBlsPrime(i + 4, pXIm);
    final boolean pYReIsInRange = callToLTBlsPrime(i + 8, pYRe);
    final boolean pYImIsInRange = callToLTBlsPrime(i + 12, pYIm);

    final boolean wellFormedCoordinate =
        pXReIsInRange && pXImIsInRange && pYReIsInRange && pYImIsInRange;

    for (int j = 0; j <= CT_MAX_LARGE_POINT; j++) {
      this.mintBit.set(i + j, !wellFormedCoordinate);
    }

    return wellFormedCoordinate;
  }

  // Note: in the specs isInfinity receives directly the sum of the coordinate
  private boolean isInfinity(int i, Bytes point) {
    final boolean isInfinity = point.isZero();

    // Set the isInfinity flag for all coordinate
    for (int j = 0; j < point.size() / LLARGE; j++) {
      this.isInfinity.set(i + j, isInfinity);
    }

    return isInfinity;
  }

  private int getCtMax(
      PrecompileScenarioFragment.PrecompileFlag precompileFlag,
      boolean isData,
      boolean isFirstInput) {
    if (isData) {
      return switch (precompileFlag) {
        case PRC_POINT_EVALUATION -> isFirstInput ? CT_MAX_POINT_EVALUATION : 0;
        case PRC_BLS_G1_ADD -> CT_MAX_SMALL_POINT;
        case PRC_BLS_G1_MSM -> isFirstInput ? CT_MAX_SMALL_POINT : CT_MAX_SCALAR;
        case PRC_BLS_G2_ADD -> CT_MAX_LARGE_POINT;
        case PRC_BLS_G2_MSM -> isFirstInput ? CT_MAX_LARGE_POINT : CT_MAX_SCALAR;
        case PRC_BLS_PAIRING_CHECK -> isFirstInput ? CT_MAX_SMALL_POINT : CT_MAX_LARGE_POINT;
        case PRC_BLS_MAP_FP_TO_G1 -> isFirstInput ? CT_MAX_MAP_FP_TO_G1 : 0;
        case PRC_BLS_MAP_FP2_TO_G2 -> isFirstInput ? CT_MAX_MAP_FP2_TO_G2 : 0;
        default -> throw new IllegalStateException("invalid BLS type");
      };
    } else {
      return getIndexMax(precompileFlag, false);
    }
  }

  void trace(Trace.Blsdata trace, final int stamp, final long previousId) {
    final Bytes limb = Bytes.concatenate(callData, returnData);
    final boolean returnDataIsNonEmpty = returnData.toArray().length > 0;

    final Bytes deltaByte =
        leftPadTo(Bytes.minimalBytes(id - previousId - 1), nBYTES_OF_DELTA_BYTES);

    int ct = 0;
    boolean isFirstInput = true;
    int accInputs = 0;
    boolean mintBitAcc = false;
    boolean mextBitAcc = false;
    boolean nontrivialPairOfPointsAcc = false;

    for (int i = 0; i < nRows; i++) {
      boolean isData = i < nRowsData;
      isFirstInput =
          switch (precompileFlag) {
            case PRC_POINT_EVALUATION -> i <= CT_MAX_POINT_EVALUATION;
            case PRC_BLS_G1_ADD -> i <= CT_MAX_SMALL_POINT;
            case PRC_BLS_G1_MSM ->
                (i % (CT_MAX_SMALL_POINT + CT_MAX_SCALAR + 2)) <= CT_MAX_SMALL_POINT;
            case PRC_BLS_G2_ADD -> i <= CT_MAX_LARGE_POINT;
            case PRC_BLS_G2_MSM ->
                (i % (CT_MAX_LARGE_POINT + CT_MAX_SCALAR + 2)) <= CT_MAX_LARGE_POINT;
            case PRC_BLS_PAIRING_CHECK ->
                (i % (CT_MAX_SMALL_POINT + CT_MAX_LARGE_POINT + 2)) <= CT_MAX_SMALL_POINT;
            case PRC_BLS_MAP_FP_TO_G1 -> i <= CT_MAX_MAP_FP_TO_G1;
            case PRC_BLS_MAP_FP2_TO_G2 -> i <= CT_MAX_MAP_FP2_TO_G2;
            default -> throw new IllegalStateException("invalid BLS type");
          };
      final int ctMax = getCtMax(precompileFlag, isData, isFirstInput);
      final int indexMax = getIndexMax(precompileFlag, isData);

      if (isData) {
        switch (precompileFlag) {
          case PRC_BLS_G1_MSM -> accInputs = i / (INDEX_MAX_DATA_G1_MSM_MIN + 1) + 1;
          case PRC_BLS_G2_MSM -> accInputs = i / (INDEX_MAX_DATA_G2_MSM_MIN + 1) + 1;
          case PRC_BLS_PAIRING_CHECK -> accInputs = i / (INDEX_MAX_DATA_PAIRING_CHECK_MIN + 1) + 1;
          default -> accInputs = 0;
        }
      }

      mintBitAcc = mintBitAcc || mintBit.get(i);
      mextBitAcc = mextBitAcc || mextBit.get(i);
      nontrivialPairOfPointsAcc = nontrivialPairOfPointsAcc || nontrivialPairOfPointsBit.get(i);

      trace
          .stamp(stamp)
          .id(id)
          .totalSize(isData ? totalSizeData : totalSizeResult)
          .index(isData ? i : i - nRowsData)
          .indexMax(indexMax)
          .phase(isData ? precompileFlag.dataPhase() : precompileFlag.resultPhase())
          .limb(isData || returnDataIsNonEmpty ? limb.slice(i * LLARGE, LLARGE) : ZERO)
          .successBit(successBit)
          .ct(ct)
          .ctMax(ctMax)
          .dataPointEvaluationFlag(precompileFlag == PRC_POINT_EVALUATION && isData)
          .dataBlsG1AddFlag(precompileFlag == PRC_BLS_G1_ADD && isData)
          .dataBlsG1MsmFlag(precompileFlag == PRC_BLS_G1_MSM && isData)
          .dataBlsG2AddFlag(precompileFlag == PRC_BLS_G2_ADD && isData)
          .dataBlsG2MsmFlag(precompileFlag == PRC_BLS_G2_MSM && isData)
          .dataBlsPairingCheckFlag(precompileFlag == PRC_BLS_PAIRING_CHECK && isData)
          .dataBlsMapFpToG1Flag(precompileFlag == PRC_BLS_MAP_FP_TO_G1 && isData)
          .dataBlsMapFp2ToG2Flag(precompileFlag == PRC_BLS_MAP_FP2_TO_G2 && isData)
          .rsltPointEvaluationFlag(precompileFlag == PRC_POINT_EVALUATION && !isData)
          .rsltBlsG1AddFlag(precompileFlag == PRC_BLS_G1_ADD && !isData)
          .rsltBlsG1MsmFlag(precompileFlag == PRC_BLS_G1_MSM && !isData)
          .rsltBlsG2AddFlag(precompileFlag == PRC_BLS_G2_ADD && !isData)
          .rsltBlsG2MsmFlag(precompileFlag == PRC_BLS_G2_MSM && !isData)
          .rsltBlsPairingCheckFlag(precompileFlag == PRC_BLS_PAIRING_CHECK && !isData)
          .rsltBlsMapFpToG1Flag(precompileFlag == PRC_BLS_MAP_FP_TO_G1 && !isData)
          .rsltBlsMapFp2ToG2Flag(precompileFlag == PRC_BLS_MAP_FP2_TO_G2 && !isData)
          .accInputs(isData ? accInputs : 0)
          .byteDelta(
              i < nBYTES_OF_DELTA_BYTES ? UnsignedByte.of(deltaByte.get(i)) : UnsignedByte.of(0))
          .malformedDataInternalBit(mintBit.get(i) && isData)
          .malformedDataInternalAcc(mintBitAcc && isData)
          .malformedDataInternalAccTot(malformedDataInternal)
          .malformedDataExternalBit(mextBit.get(i) && isData)
          .malformedDataExternalAcc(mextBitAcc && isData)
          .malformedDataExternalAccTot(malformedDataExternal)
          .wellformedDataTrivial(wellformedDataTrivial)
          .wellformedDataNontrivial(wellformedDataNonTrivial)
          .isFirstInput(isFirstInput && isData)
          .isSecondInput(!isFirstInput && isData)
          .isInfinity(isInfinity.get(i))
          .nontrivialPairOfPointsBit(nontrivialPairOfPointsBit.get(i))
          .nontrivialPairOfPointsAcc(nontrivialPairOfPointsAcc && isData)
          .wcpFlag(wcpFlag.get(i))
          .wcpArg1Hi(wcpArg1Hi.get(i))
          .wcpArg1Lo(wcpArg1Lo.get(i))
          .wcpArg2Hi(wcpArg2Hi.get(i))
          .wcpArg2Lo(wcpArg2Lo.get(i))
          .wcpRes(wcpRes.get(i))
          .wcpInst(wcpInst.get(i).unsignedByteValue())
          .validateRow();

      // Increment ct up to ctMax, then reset to 0
      if (ct < ctMax) {
        ct++;
      } else {
        ct = 0;
      }
    }
  }

  @Override
  protected int computeLineCount() {
    return nRowsData + nRowsResult;
  }
}
