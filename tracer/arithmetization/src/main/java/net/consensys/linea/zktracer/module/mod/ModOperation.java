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

package net.consensys.linea.zktracer.module.mod;

import static com.google.common.base.Preconditions.checkElementIndex;
import static net.consensys.linea.zktracer.Trace.MMEDIUM;
import static net.consensys.linea.zktracer.module.Util.byteBits;

import java.math.BigInteger;
import java.util.Arrays;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.bytestheta.BaseBytes;
import net.consensys.linea.zktracer.bytestheta.BaseTheta;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;
import org.apache.tuweni.units.bigints.UInt64;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class ModOperation extends ModuleOperation {

  public static final short NB_ROWS_MOD = MMEDIUM;

  @EqualsAndHashCode.Include @Getter private final OpCode opCode;
  @EqualsAndHashCode.Include @Getter private final Bytes32 rawArg1;
  @EqualsAndHashCode.Include @Getter private final Bytes32 rawArg2;
  private final boolean oli;
  private BaseBytes arg1;
  private BaseBytes arg2;

  private BaseBytes result = BaseBytes.fromBytes32(Bytes32.ZERO);
  private BaseTheta aBytes = BaseTheta.fromBytes32(Bytes32.ZERO);
  private BaseTheta bBytes = BaseTheta.fromBytes32(Bytes32.ZERO);
  private BaseTheta qBytes = BaseTheta.fromBytes32(Bytes32.ZERO);
  private BaseTheta rBytes = BaseTheta.fromBytes32(Bytes32.ZERO);
  private BaseTheta hBytes = BaseTheta.fromBytes32(Bytes32.ZERO);
  private BaseTheta dBytes = BaseTheta.fromBytes32(Bytes32.ZERO);
  private final boolean[] cmp1 = new boolean[8];
  private final boolean[] cmp2 = new boolean[8];
  private Boolean[] msb1 = new Boolean[8];
  private Boolean[] msb2 = new Boolean[8];

  public ModOperation(OpCode opCode, Bytes32 arg1, Bytes32 arg2) {
    rawArg1 = arg1;
    rawArg2 = arg2;

    this.opCode = opCode;
    this.oli = arg2.isZero();
  }

  private void compute() {
    arg1 = BaseBytes.fromBytes32(rawArg1);
    arg2 = BaseBytes.fromBytes32(rawArg2);

    Arrays.fill(msb1, false);
    Arrays.fill(msb2, false);

    if (!oli) {
      result = getRes(opCode, rawArg1, rawArg2);

      UInt256 a = absoluteValueIfSignedInst(rawArg1);
      aBytes = BaseTheta.fromBytes32(a);

      UInt256 b = absoluteValueIfSignedInst(rawArg2);
      bBytes = BaseTheta.fromBytes32(b);

      UInt256 q = a.divide(b);
      qBytes = BaseTheta.fromBytes32(q);

      UInt256 r = a.mod(b);
      rBytes = BaseTheta.fromBytes32(r);

      dBytes = BaseTheta.fromBytes32(Bytes32.ZERO);
      this.setCmp12();
      this.setDeltas();
      this.setAlphaBetasH012();

      UnsignedByte msb1 = UnsignedByte.of(arg1.getHigh().get(0));
      UnsignedByte msb2 = UnsignedByte.of(arg2.getHigh().get(0));

      this.msb1 = byteBits(msb1);
      this.msb2 = byteBits(msb2);
    }
  }

  private static BaseBytes getRes(OpCode op, Bytes32 arg1, Bytes32 arg2) {
    return switch (op) {
      case DIV -> BaseBytes.fromBytes32(UInt256.fromBytes(arg1).divide(UInt256.fromBytes(arg2)));
      case SDIV -> BaseBytes.fromBytes32(UInt256.fromBytes(arg1).sdiv0(UInt256.fromBytes(arg2)));
      case MOD -> BaseBytes.fromBytes32(UInt256.fromBytes(arg1).mod(UInt256.fromBytes(arg2)));
      case SMOD -> BaseBytes.fromBytes32(UInt256.fromBytes(arg1).smod0(UInt256.fromBytes(arg2)));
      default -> throw new IllegalArgumentException("Modular arithmetic was given wrong opcode");
    };
  }

  private UInt256 absoluteValueIfSignedInst(Bytes32 arg) {
    if (this.isSigned()) {
      Bytes argBytes = Bytes.of(arg.toArray());
      return UInt256.valueOf(argBytes.toBigInteger().abs());
    }
    return UInt256.fromBytes(arg);
  }

  private UInt256 bVar(int k) {
    checkElementIndex(k, 4);
    return UInt256.fromBytes(bBytes.get(k));
  }

  private UInt256 qVar(int k) {
    checkElementIndex(k, 4);
    return UInt256.fromBytes(qBytes.get(k));
  }

  private UInt256 rVar(int k) {
    checkElementIndex(k, 4);
    return UInt256.fromBytes(rBytes.get(k));
  }

  private UInt256 hVar(int k) {
    checkElementIndex(k, 3);
    return UInt256.fromBytes(hBytes.get(k));
  }

  private void setCmp12() {
    for (int k = 0; k < 4; k++) {
      cmp1[k] = bVar(k).compareTo(rVar(k)) > 0;
      cmp2[k] = bVar(k).compareTo(rVar(k)) == 0;
    }
  }

  private void setDeltas() {
    for (int k = 0; k < 4; k++) {
      UInt256 delta;
      if (cmp1[k]) {
        delta = bVar(k).subtract(rVar(k)).subtract(UInt256.ONE);
      } else {
        delta = rVar(k).subtract(bVar(k));
      }
      dBytes.set(k, delta.slice(24, 8));
    }
  }

  private void setAlphaBetasH012() {
    UInt256 theta = UInt256.ONE;
    UInt256 thetaSquared = UInt256.ONE;

    theta = theta.shiftLeft(64);
    thetaSquared = thetaSquared.shiftLeft(128);

    UInt256 sum = bVar(0).multiply(qVar(1)).add(bVar(1).multiply(qVar(0)));
    hBytes = BaseTheta.fromBytes32(sum);

    // alpha
    cmp2[4] = sum.compareTo(thetaSquared) >= 0;

    sum =
        bVar(0)
            .multiply(qVar(3))
            .add(bVar(1).multiply(qVar(2)))
            .add(bVar(2).multiply(qVar(1)))
            .add(bVar(3).multiply(qVar(0)));

    if (sum.bitLength() > 64) {
      throw new RuntimeException("b[0]q[3] + b[1]q[2] + b[2]q[1] + b[3]q[0] >= (1 << 64)");
    }

    hBytes.set(2, sum.slice(24, 8));

    sum = qVar(0).multiply(bVar(0));
    sum = sum.add(hVar(0).multiply(theta));
    sum = sum.add(UInt256.fromBytes(rBytes.getLow()));

    UInt256 beta = sum.divide(thetaSquared);
    if (beta.compareTo(UInt256.valueOf(2)) > 0) {
      throw new RuntimeException("b[0]q[0] + theta.h[0] + rLo = [beta|...] with beta > 2");
    }

    UInt64 betaUInt64 = UInt64.valueOf(beta.toUnsignedBigInteger());
    cmp2[5] = betaUInt64.mod(UInt64.valueOf(2)).compareTo(UInt64.ONE) == 0; // beta_0
    cmp2[6] = betaUInt64.divide(UInt64.valueOf(2)).compareTo(UInt64.ONE) == 0; // beta_1

    BigInteger sumInt = sum.mod(thetaSquared).toUnsignedBigInteger();
    BigInteger aLo = aBytes.getLow().toUnsignedBigInteger();
    if (sumInt.compareTo(aLo) != 0) {
      throw new RuntimeException("b[0]q[0] + theta.h[0] + rLo = [beta|xxx] and xxx != aLo");
    }
  }

  boolean isSigned() {
    return opCode == OpCode.SDIV || opCode == OpCode.SMOD;
  }

  boolean isDiv() {
    return opCode == OpCode.DIV || opCode == OpCode.SDIV;
  }

  int numberOfRows() {
    return oli ? 1 : NB_ROWS_MOD;
  }

  public void trace(Trace.Mod trace, int stamp) {
    this.compute();

    for (short ct = 0; ct < this.numberOfRows(); ct++) {
      final int accLength = ct + 1;
      trace
          .stamp(stamp)
          .oli(oli)
          .mli(!oli)
          .ct(ct)
          .inst(opCode.unsignedByteValue())
          .isSdiv(opCode == OpCode.SDIV)
          .isDiv(opCode == OpCode.DIV)
          .isSmod(opCode == OpCode.SMOD)
          .isMod(opCode == OpCode.MOD)
          .signed(this.isSigned())
          .arg1Hi(arg1.getHigh())
          .arg1Lo(arg1.getLow())
          .arg2Hi(arg2.getHigh())
          .arg2Lo(arg2.getLow())
          .resHi(result.getHigh())
          .resLo(result.getLow())
          .acc12(arg1.getBytes32().slice(8, ct + 1))
          .acc13(arg1.getBytes32().slice(0, ct + 1))
          .acc22(arg2.getBytes32().slice(8, ct + 1))
          .acc23(arg2.getBytes32().slice(0, ct + 1))
          .accB0(bBytes.get(0).slice(0, accLength))
          .accB1(bBytes.get(1).slice(0, accLength))
          .accB2(bBytes.get(2).slice(0, accLength))
          .accB3(bBytes.get(3).slice(0, accLength))
          .accR0(rBytes.get(0).slice(0, accLength))
          .accR1(rBytes.get(1).slice(0, accLength))
          .accR2(rBytes.get(2).slice(0, accLength))
          .accR3(rBytes.get(3).slice(0, accLength))
          .accQ0(qBytes.get(0).slice(0, accLength))
          .accQ1(qBytes.get(1).slice(0, accLength))
          .accQ2(qBytes.get(2).slice(0, accLength))
          .accQ3(qBytes.get(3).slice(0, accLength))
          .accDelta0(dBytes.get(0).slice(0, accLength))
          .accDelta1(dBytes.get(1).slice(0, accLength))
          .accDelta2(dBytes.get(2).slice(0, accLength))
          .accDelta3(dBytes.get(3).slice(0, accLength))
          .byte22(UnsignedByte.of(arg2.getByte(ct + 8)))
          .byte23(UnsignedByte.of(arg2.getByte(ct)))
          .byte12(UnsignedByte.of(arg1.getByte(ct + 8)))
          .byte13(UnsignedByte.of(arg1.getByte(ct)))
          .byteB0(UnsignedByte.of(bBytes.get(0).get(ct)))
          .byteB1(UnsignedByte.of(bBytes.get(1).get(ct)))
          .byteB2(UnsignedByte.of(bBytes.get(2).get(ct)))
          .byteB3(UnsignedByte.of(bBytes.get(3).get(ct)))
          .byteR0(UnsignedByte.of(rBytes.get(0).get(ct)))
          .byteR1(UnsignedByte.of(rBytes.get(1).get(ct)))
          .byteR2(UnsignedByte.of(rBytes.get(2).get(ct)))
          .byteR3(UnsignedByte.of(rBytes.get(3).get(ct)))
          .byteQ0(UnsignedByte.of(qBytes.get(0).get(ct)))
          .byteQ1(UnsignedByte.of(qBytes.get(1).get(ct)))
          .byteQ2(UnsignedByte.of(qBytes.get(2).get(ct)))
          .byteQ3(UnsignedByte.of(qBytes.get(3).get(ct)))
          .byteDelta0(UnsignedByte.of(dBytes.get(0).get(ct)))
          .byteDelta1(UnsignedByte.of(dBytes.get(1).get(ct)))
          .byteDelta2(UnsignedByte.of(dBytes.get(2).get(ct)))
          .byteDelta3(UnsignedByte.of(dBytes.get(3).get(ct)))
          .byteH0(UnsignedByte.of(hBytes.get(0).get(ct)))
          .byteH1(UnsignedByte.of(hBytes.get(1).get(ct)))
          .byteH2(UnsignedByte.of(hBytes.get(2).get(ct)))
          .accH0(Bytes.wrap(hBytes.get(0)).slice(0, ct + 1))
          .accH1(Bytes.wrap(hBytes.get(1)).slice(0, ct + 1))
          .accH2(Bytes.wrap(hBytes.get(2)).slice(0, ct + 1))
          .cmp1(cmp1[ct])
          .cmp2(cmp2[ct])
          .msb1(msb1[ct])
          .msb2(msb2[ct])
          .validateRow();
    }
  }

  @Override
  protected int computeLineCount() {
    return this.numberOfRows();
  }
}
