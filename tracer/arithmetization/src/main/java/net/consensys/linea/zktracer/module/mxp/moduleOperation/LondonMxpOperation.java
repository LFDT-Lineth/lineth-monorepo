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

package net.consensys.linea.zktracer.module.mxp.moduleOperation;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE_MO;
import static net.consensys.linea.zktracer.TraceLondon.Mxp.*;
import static net.consensys.linea.zktracer.module.Util.max;
import static net.consensys.linea.zktracer.module.mxp.MxpUtils.memoryCost;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import java.math.BigInteger;
import java.util.Arrays;

import lombok.Getter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;
import net.consensys.linea.zktracer.module.mxp.moduleCall.LondonMxpCall;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.opcode.gas.BillingRate;
import net.consensys.linea.zktracer.opcode.gas.MxpType;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.evm.frame.MessageFrame;
import org.hyperledger.besu.evm.internal.Words;

@Getter
public class LondonMxpOperation extends MxpOperation {
  public static final BigInteger TWO_POW_128 = BigInteger.ONE.shiftLeft(128);
  public static final BigInteger TWO_POW_32 = BigInteger.ONE.shiftLeft(32);

  private final int contextNumber;
  private final LondonMxpCall londonMxpCall;

  private BigInteger maxOffset1 = BigInteger.ZERO;
  private BigInteger maxOffset2 = BigInteger.ZERO;
  private BigInteger maxOffset = BigInteger.ZERO;

  @Getter private boolean roob;
  @Getter private boolean noOperation;
  private boolean comp;

  private BigInteger acc1 = BigInteger.ZERO;
  private BigInteger acc2 = BigInteger.ZERO;
  private BigInteger acc3 = BigInteger.ZERO;
  private BigInteger acc4 = BigInteger.ZERO;
  private BigInteger accA = BigInteger.ZERO;
  private BigInteger accW = BigInteger.ZERO;
  private BigInteger accQ = BigInteger.ZERO;

  private UnsignedByte[] byte1;
  private UnsignedByte[] byte2;
  private UnsignedByte[] byte3;
  private UnsignedByte[] byte4;
  private UnsignedByte[] byteA;
  private UnsignedByte[] byteW;
  private UnsignedByte[] byteQ;
  private UnsignedByte[] byteQQ;
  private UnsignedByte[] byteR;

  private boolean expands;
  private final MxpType typeMxp;
  private long wordsNew;
  private final long cMem;
  private long cMemNew;
  private long quadCost = 0;
  private long linCost = 0;

  public LondonMxpOperation(final MxpCall mxpCall) {
    super(mxpCall);
    // We cast the mxpCall to a LondonMxpCall, to keep the same behavior as in the following forks
    // MxpCall and LondonMxpCall are strictly the same
    this.londonMxpCall = (LondonMxpCall) mxpCall;
    final Hub hub = this.londonMxpCall.hub;
    final MessageFrame frame = hub.messageFrame();
    this.wordsNew = frame.memoryWordSize(); // will (may) be updated later
    this.cMem = memoryCost(frame.memoryWordSize());
    this.cMemNew = memoryCost(frame.memoryWordSize()); // will (may) be updated later
    this.contextNumber = hub.currentFrame().contextNumber();
    this.typeMxp = this.londonMxpCall.getOpCodeData().billing().type();

    setRoob();
    setNoOperation();
    setMaxOffset1and2();
    setMaxOffsetAndMxpx();
    setInitializeByteArrays();
    setAccAAndFirstTwoBytesOfByteR();
    setExpands();
    setWordsNew(frame);
    setCMemNew();
    setCosts();
    setMtntop();

    // "tracing" the remaining fields of the MxpCall
    londonMxpCall.setGasMxp(getGasMxp());
  }

  @Override
  protected int computeLineCount() {
    return this.nRows();
  }

  void compute() {
    setComp();
    setAcc1and2();
    setAcc3();
    setAcc4();
    setAccWAndLastTwoBytesOfByteR();
    setAccQAndByteQQ();
    setBytes();
  }

  private void setInitializeByteArrays() {
    int nRows = nRows();
    byte1 = new UnsignedByte[nRows];
    byte2 = new UnsignedByte[nRows];
    byte3 = new UnsignedByte[nRows];
    byte4 = new UnsignedByte[nRows];
    byteA = new UnsignedByte[nRows];
    byteW = new UnsignedByte[nRows];
    byteQ = new UnsignedByte[nRows];
    byteQQ = new UnsignedByte[nRows];
    byteR = new UnsignedByte[nRows];
    Arrays.fill(byte1, UnsignedByte.of(0));
    Arrays.fill(byte2, UnsignedByte.of(0));
    Arrays.fill(byte3, UnsignedByte.of(0));
    Arrays.fill(byte4, UnsignedByte.of(0));
    Arrays.fill(byteA, UnsignedByte.of(0));
    Arrays.fill(byteW, UnsignedByte.of(0));
    Arrays.fill(byteQ, UnsignedByte.of(0));
    Arrays.fill(byteQQ, UnsignedByte.of(0));
    Arrays.fill(byteR, UnsignedByte.of(0));
  }

  /** get ridiculously out of bounds. */
  protected void setRoob() {
    roob =
        switch (typeMxp) {
          case TYPE_2, TYPE_3 -> londonMxpCall
                  .getOffset1()
                  .toUnsignedBigInteger()
                  .compareTo(TWO_POW_128)
              >= 0;
          case TYPE_4 -> londonMxpCall.getSize1().toUnsignedBigInteger().compareTo(TWO_POW_128) >= 0
              || (londonMxpCall.getOffset1().toUnsignedBigInteger().compareTo(TWO_POW_128) >= 0
                  && !londonMxpCall.getSize1().toUnsignedBigInteger().equals(BigInteger.ZERO));
          case TYPE_5 -> londonMxpCall.getSize1().toUnsignedBigInteger().compareTo(TWO_POW_128) >= 0
              || (londonMxpCall.getOffset1().toUnsignedBigInteger().compareTo(TWO_POW_128) >= 0
                  && !londonMxpCall.getSize1().toUnsignedBigInteger().equals(BigInteger.ZERO))
              || (londonMxpCall.getSize2().toUnsignedBigInteger().compareTo(TWO_POW_128) >= 0
                  || (londonMxpCall.getOffset2().toUnsignedBigInteger().compareTo(TWO_POW_128) >= 0
                      && !londonMxpCall.getSize2().toUnsignedBigInteger().equals(BigInteger.ZERO)));
          default -> false;
        };
  }

  /** get no op. */
  protected void setNoOperation() {
    if (roob) {
      noOperation = false;
    }
    noOperation =
        switch (typeMxp) {
          case TYPE_1 -> true;
          case TYPE_4 -> londonMxpCall.getSize1().isZero();
          case TYPE_5 -> londonMxpCall.getSize1().isZero() && londonMxpCall.getSize2().isZero();
          default -> false;
        };
  }

  private void setMtntop() {
    londonMxpCall.setMayTriggerNontrivialMmuOperation();
  }

  /** set max offsets 1 and 2. */
  protected void setMaxOffset1and2() {
    if (getMxpExecutionPath() != mxpExecutionPath.TRIVIAL) {
      switch (typeMxp) {
        case TYPE_2 -> maxOffset1 =
            londonMxpCall.getOffset1().toUnsignedBigInteger().add(BigInteger.valueOf(31));
        case TYPE_3 -> maxOffset1 = londonMxpCall.getOffset1().toUnsignedBigInteger();
        case TYPE_4 -> {
          if (!londonMxpCall.getSize1().toUnsignedBigInteger().equals(BigInteger.ZERO)) {
            maxOffset1 =
                londonMxpCall
                    .getOffset1()
                    .toUnsignedBigInteger()
                    .add(londonMxpCall.getSize1().toUnsignedBigInteger())
                    .subtract(BigInteger.ONE);
          }
        }
        case TYPE_5 -> {
          if (!londonMxpCall.getSize1().toUnsignedBigInteger().equals(BigInteger.ZERO)) {
            maxOffset1 =
                londonMxpCall
                    .getOffset1()
                    .toUnsignedBigInteger()
                    .add(londonMxpCall.getSize1().toUnsignedBigInteger())
                    .subtract(BigInteger.ONE);
          }
          if (!londonMxpCall.getSize2().toUnsignedBigInteger().equals(BigInteger.ZERO)) {
            maxOffset2 =
                londonMxpCall
                    .getOffset2()
                    .toUnsignedBigInteger()
                    .add(londonMxpCall.getSize2().toUnsignedBigInteger())
                    .subtract(BigInteger.ONE);
          }
        }
      }
    }
  }

  /** set max offset and mxpx. */
  protected void setMaxOffsetAndMxpx() {
    if (roob || noOperation) {
      londonMxpCall.setMxpx(roob);
    } else {
      // choose the max value
      maxOffset = max(maxOffset1, maxOffset2);
      londonMxpCall.setMxpx(maxOffset.compareTo(TWO_POW_32) >= 0);
    }
  }

  public void setExpands() {
    if (!roob && !noOperation && !londonMxpCall.isMxpx()) {
      expands = accA.compareTo(BigInteger.valueOf(londonMxpCall.getMemorySizeInWords())) > 0;
    }
  }

  private long getLinCost(OpCodeData opCodeData, long sizeInBytes) {
    if (getMxpExecutionPath() == mxpExecutionPath.NON_TRIVIAL) {
      if (opCodeData.billing().billingRate() == BillingRate.BY_BYTE) {
        return opCodeData.billing().perUnit().cost() * sizeInBytes;
      }
      if (opCodeData.billing().billingRate() == BillingRate.BY_WORD) {
        long sizeInWords = (sizeInBytes + WORD_SIZE_MO) / WORD_SIZE;
        return opCodeData.billing().perUnit().cost() * sizeInWords;
      }
    }
    return 0;
  }

  protected void setComp() {
    comp = maxOffset1.compareTo(maxOffset2) >= 0;
  }

  // This should translate into code page 12, point 5?
  protected void setAccAAndFirstTwoBytesOfByteR() {
    if (this.getMxpExecutionPath() == mxpExecutionPath.NON_TRIVIAL) {
      BigInteger maxOffsetPlusOne = maxOffset.add(BigInteger.ONE);
      accA = divRoundedUpBigInteger(maxOffsetPlusOne, BigInteger.valueOf(32));
      BigInteger diff = accA.multiply(BigInteger.valueOf(32)).subtract(maxOffsetPlusOne);
      Bytes32 diffBytes = Bytes32.leftPad(UInt256.valueOf(diff));

      UnsignedByte r = UnsignedByte.of(diffBytes.get(31));
      int rAsInt = r.toInteger();
      byteR[0] = UnsignedByte.of(rAsInt + 224);
      byteR[1] = UnsignedByte.of(rAsInt);
    }
  }

  public BigInteger divRoundedUpBigInteger(BigInteger a, BigInteger b) {
    return a.add(b.subtract(BigInteger.ONE)).divide(b);
  }

  protected void setAcc1and2() {
    if (roob) {
      return;
    }
    if (londonMxpCall.isMxpx()) {
      if (maxOffset1.compareTo(TWO_POW_32) >= 0) {
        acc1 = maxOffset1.subtract(TWO_POW_32);
      } else {
        checkArgument(
            maxOffset2.compareTo(TWO_POW_32) >= 0,
            "London MXP error: maxOffset2 = %s < 2^32",
            maxOffset2);
        acc2 = maxOffset2.subtract(TWO_POW_32);
      }
    } else {
      acc1 = maxOffset1;
      acc2 = maxOffset2;
    }
  }

  protected void setAcc3() {
    if (comp) {
      acc3 = maxOffset1.subtract(maxOffset2);
    } else {
      acc3 = maxOffset2.subtract(maxOffset1);
      acc3 = acc3.subtract(BigInteger.ONE);
    }
  }

  protected void setAcc4() {
    if (this.getMxpExecutionPath() == mxpExecutionPath.NON_TRIVIAL) {
      if (expands) {
        acc4 = accA.subtract(BigInteger.valueOf(londonMxpCall.getMemorySizeInWords() + 1));
      } else {
        acc4 = BigInteger.valueOf(londonMxpCall.getMemorySizeInWords()).subtract(accA);
      }
    }
  }

  protected void setAccWAndLastTwoBytesOfByteR() {
    if (this.getMxpExecutionPath() == mxpExecutionPath.NON_TRIVIAL) {
      if (typeMxp != MxpType.TYPE_4) {
        return;
      }

      accW =
          londonMxpCall
              .getSize1()
              .toUnsignedBigInteger()
              .add(BigInteger.valueOf(31))
              .divide(BigInteger.valueOf(32));

      final BigInteger r =
          accW.multiply(BigInteger.valueOf(32))
              .subtract(londonMxpCall.getSize1().toUnsignedBigInteger());

      // r in [0,31]
      UnsignedByte rByte = UnsignedByte.of(r.toByteArray()[r.toByteArray().length - 1]);
      int rByteAsInt = rByte.toInteger();
      byteR[2] = UnsignedByte.of(rByteAsInt + 224);
      byteR[3] = rByte;
    }
  }

  protected enum mxpExecutionPath {
    TRIVIAL,
    NON_TRIVIAL_BUT_MXPX,
    NON_TRIVIAL
  }

  private mxpExecutionPath getMxpExecutionPath() {
    if (this.isRoob() || this.isNoOperation()) {
      return mxpExecutionPath.TRIVIAL;
    }
    if (londonMxpCall.mxpx) {
      return mxpExecutionPath.NON_TRIVIAL_BUT_MXPX;
    }
    return mxpExecutionPath.NON_TRIVIAL;
  }

  public int ctMax() {
    return switch (this.getMxpExecutionPath()) {
      case TRIVIAL -> CT_MAX_TRIVIAL;
      case NON_TRIVIAL_BUT_MXPX -> CT_MAX_NON_TRIVIAL_BUT_MXPX;
      case NON_TRIVIAL -> CT_MAX_NON_TRIVIAL;
    };
  }

  public int nRows() {
    return ctMax() + 1;
  }

  protected void setAccQAndByteQQ() {
    // accQ and byteQQ all equal to 0 by default
    if (this.getMxpExecutionPath() == mxpExecutionPath.NON_TRIVIAL) {
      setAccQAndByteQQNonTrivialCase();
    }
  }

  protected void setAccQAndByteQQNonTrivialCase() {
    long square = wordsNew * wordsNew; // ACC_A

    long quotient = square / 512;
    long remainder = square % 512;

    accQ = BigInteger.valueOf(quotient % (1L << 32));

    Bytes32 quotientBytes = UInt256.valueOf(quotient); // q'
    Bytes32 remainderBytes = UInt256.valueOf(remainder); // r'

    byteQQ[0] = UnsignedByte.of(quotientBytes.get(quotientBytes.size() - 6));
    byteQQ[1] = UnsignedByte.of(quotientBytes.get(quotientBytes.size() - 5));
    byteQQ[2] = UnsignedByte.of(remainderBytes.get(quotientBytes.size() - 2));
    byteQQ[3] = UnsignedByte.of(remainderBytes.get(quotientBytes.size() - 1));
  }

  protected void setBytes() {
    final int nRows = nRows();
    Bytes32 b1 = UInt256.valueOf(acc1);
    Bytes32 b2 = UInt256.valueOf(acc2);
    Bytes32 b3 = UInt256.valueOf(acc3);
    Bytes32 b4 = UInt256.valueOf(acc4);
    Bytes32 bA = UInt256.valueOf(accA);
    Bytes32 bW = UInt256.valueOf(accW);
    Bytes32 bQ = UInt256.valueOf(accQ);
    for (int i = 0; i < nRows; i++) {
      byte1[i] = UnsignedByte.of(b1.get(b1.size() - 1 - nRows + i));
      byte2[i] = UnsignedByte.of(b2.get(b2.size() - 1 - nRows + i));
      byte3[i] = UnsignedByte.of(b3.get(b3.size() - 1 - nRows + i));
      byte4[i] = UnsignedByte.of(b4.get(b4.size() - 1 - nRows + i));
      byteA[i] = UnsignedByte.of(bA.get(bA.size() - 1 - nRows + i));
      byteW[i] = UnsignedByte.of(bW.get(bW.size() - 1 - nRows + i));
      byteQ[i] = UnsignedByte.of(bQ.get(bQ.size() - 1 - nRows + i));
    }
  }

  private void setWordsNew(final MessageFrame frame) {
    if (getMxpExecutionPath() == LondonMxpOperation.mxpExecutionPath.NON_TRIVIAL && expands) {
      switch (getTypeMxp()) {
        case TYPE_1 -> wordsNew =
            frame.calculateMemoryExpansion(Words.clampedToLong(londonMxpCall.getOffset1()), 0);
        case TYPE_2 -> wordsNew =
            frame.calculateMemoryExpansion(Words.clampedToLong(londonMxpCall.getOffset1()), 32);
        case TYPE_3 -> wordsNew =
            frame.calculateMemoryExpansion(Words.clampedToLong(londonMxpCall.getOffset1()), 1);
        case TYPE_4 -> wordsNew =
            frame.calculateMemoryExpansion(
                Words.clampedToLong(londonMxpCall.getOffset1()),
                Words.clampedToLong(londonMxpCall.getSize1()));
        case TYPE_5 -> {
          long wordsNew1 =
              frame.calculateMemoryExpansion(
                  Words.clampedToLong(londonMxpCall.getOffset1()),
                  Words.clampedToLong(londonMxpCall.getSize1()));
          long wordsNew2 =
              frame.calculateMemoryExpansion(
                  Words.clampedToLong(londonMxpCall.getOffset2()),
                  Words.clampedToLong(londonMxpCall.getSize2()));
          wordsNew = Math.max(wordsNew1, wordsNew2);
        }
      }
    }
  }

  private void setCMemNew() {
    if (getMxpExecutionPath() == LondonMxpOperation.mxpExecutionPath.NON_TRIVIAL && expands) {
      cMemNew = memoryCost(wordsNew);
    }
  }

  private void setCosts() {
    if (getMxpExecutionPath() == mxpExecutionPath.NON_TRIVIAL) {
      quadCost = cMemNew - cMem;
      linCost =
          getLinCost(londonMxpCall.getOpCodeData(), Words.clampedToLong(londonMxpCall.getSize1()));
    }
  }

  long getEffectiveLinCost() {
    if (londonMxpCall.getOpCodeData().mnemonic() != OpCode.RETURN) {
      return getLinCost(
          londonMxpCall.getOpCodeData(), Words.clampedToLong(londonMxpCall.getSize1()));
    } else {
      if (londonMxpCall.isDeploys()) {
        return getLinCost(
            londonMxpCall.getOpCodeData(), Words.clampedToLong(londonMxpCall.getSize1()));
      } else {
        return 0;
      }
    }
  }

  long getGasMxp() {
    return getQuadCost() + getEffectiveLinCost();
  }

  @Override
  public void trace(int stamp, Trace.Mxp trace) {
    this.compute();

    Bytes32 acc1Bytes32 = Bytes32.leftPad(bigIntegerToBytes(this.getAcc1()));
    Bytes32 acc2Bytes32 = Bytes32.leftPad(bigIntegerToBytes(this.getAcc2()));
    Bytes32 acc3Bytes32 = Bytes32.leftPad(bigIntegerToBytes(this.getAcc3()));
    Bytes32 acc4Bytes32 = Bytes32.leftPad(bigIntegerToBytes(this.getAcc4()));
    Bytes32 accABytes32 = Bytes32.leftPad(bigIntegerToBytes(this.getAccA()));
    Bytes32 accWBytes32 = Bytes32.leftPad(bigIntegerToBytes(this.getAccW()));
    Bytes32 accQBytes32 = Bytes32.leftPad(bigIntegerToBytes(this.getAccQ()));
    final EWord eOffset1 = EWord.of(this.londonMxpCall.getOffset1());
    final EWord eOffset2 = EWord.of(this.londonMxpCall.getOffset2());
    final EWord eSize1 = EWord.of(this.londonMxpCall.getSize1());
    final EWord eSize2 = EWord.of(this.londonMxpCall.getSize2());

    final int nRows = this.nRows();
    final int nRowsComplement = 32 - nRows;

    for (int i = 0; i < nRows; i++) {
      trace
          .stamp(stamp)
          .cn(Bytes.ofUnsignedLong(this.getContextNumber()))
          .ct((short) i)
          .roob(this.isRoob())
          .noop(this.isNoOperation())
          .mxpx(this.londonMxpCall.isMxpx())
          .inst(UnsignedByte.of(this.londonMxpCall.getOpCodeData().value()))
          .mxpType1(this.londonMxpCall.getOpCodeData().billing().type() == MxpType.TYPE_1)
          .mxpType2(this.londonMxpCall.getOpCodeData().billing().type() == MxpType.TYPE_2)
          .mxpType3(this.londonMxpCall.getOpCodeData().billing().type() == MxpType.TYPE_3)
          .mxpType4(this.londonMxpCall.getOpCodeData().billing().type() == MxpType.TYPE_4)
          .mxpType5(this.londonMxpCall.getOpCodeData().billing().type() == MxpType.TYPE_5)
          .gword(
              Bytes.ofUnsignedLong(
                  this.londonMxpCall.getOpCodeData().billing().billingRate() == BillingRate.BY_WORD
                      ? this.londonMxpCall.getOpCodeData().billing().perUnit().cost()
                      : 0))
          .gbyte(
              Bytes.ofUnsignedLong(
                  this.londonMxpCall.getOpCodeData().billing().billingRate() == BillingRate.BY_BYTE
                      ? this.londonMxpCall.getOpCodeData().billing().perUnit().cost()
                      : 0))
          .deploys(londonMxpCall.isDeploys())
          .offset1Hi(eOffset1.hi())
          .offset1Lo(eOffset1.lo())
          .offset2Hi(eOffset2.hi())
          .offset2Lo(eOffset2.lo())
          .size1Hi(eSize1.hi())
          .size1Lo(eSize1.lo())
          .size2Hi(eSize2.hi())
          .size2Lo(eSize2.lo())
          .maxOffset1(bigIntegerToBytes(this.getMaxOffset1()))
          .maxOffset2(bigIntegerToBytes(this.getMaxOffset2()))
          .maxOffset(bigIntegerToBytes(this.getMaxOffset()))
          .comp(this.isComp())
          .acc1(acc1Bytes32.slice(nRowsComplement, 1 + i))
          .acc2(acc2Bytes32.slice(nRowsComplement, 1 + i))
          .acc3(acc3Bytes32.slice(nRowsComplement, 1 + i))
          .acc4(acc4Bytes32.slice(nRowsComplement, 1 + i))
          .accA(accABytes32.slice(nRowsComplement, 1 + i))
          .accW(accWBytes32.slice(nRowsComplement, 1 + i))
          .accQ(accQBytes32.slice(nRowsComplement, 1 + i))
          .byte1(UnsignedByte.of(acc1Bytes32.get(nRowsComplement + i)))
          .byte2(UnsignedByte.of(acc2Bytes32.get(nRowsComplement + i)))
          .byte3(UnsignedByte.of(acc3Bytes32.get(nRowsComplement + i)))
          .byte4(UnsignedByte.of(acc4Bytes32.get(nRowsComplement + i)))
          .byteA(UnsignedByte.of(accABytes32.get(nRowsComplement + i)))
          .byteW(UnsignedByte.of(accWBytes32.get(nRowsComplement + i)))
          .byteQ(UnsignedByte.of(accQBytes32.get(nRowsComplement + i)))
          .byteQq(UnsignedByte.of(this.getByteQQ()[i].toInteger()))
          .byteR(UnsignedByte.of(this.getByteR()[i].toInteger()))
          .words(Bytes.ofUnsignedLong(this.londonMxpCall.getMemorySizeInWords()))
          .wordsNew(Bytes.ofUnsignedLong(this.getWordsNew()))
          .cMem(Bytes.ofUnsignedLong(this.getCMem())) // Returns current memory size in EVM words
          .cMemNew(Bytes.ofUnsignedLong(this.getCMemNew()))
          .quadCost(Bytes.ofUnsignedLong(this.getQuadCost()))
          .linCost(Bytes.ofUnsignedLong(this.getLinCost()))
          .gasMxp(Bytes.ofUnsignedLong(this.londonMxpCall.getGasMxp()))
          .expands(this.isExpands())
          .mtntop(this.londonMxpCall.mayTriggerNontrivialMmuOperation)
          .size1NonzeroNoMxpx(this.londonMxpCall.getSize1NonZeroNoMxpx())
          .size2NonzeroNoMxpx(this.londonMxpCall.getSize2NonZeroNoMxpx())
          .validateRow();
    }
  }
}
