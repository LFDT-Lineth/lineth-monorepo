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

package net.consensys.linea.zktracer.module.ext.calculator;

import static net.consensys.linea.zktracer.module.Util.getBit;
import static net.consensys.linea.zktracer.module.Util.getOverflow;
import static net.consensys.linea.zktracer.module.Util.multiplyRange;

import net.consensys.linea.zktracer.bytestheta.BytesArray;
import org.apache.tuweni.units.bigints.UInt256;

public class OverflowJCalculator {

  static boolean[] computeOverflowJ(
      BytesArray qBytes,
      BytesArray cBytes,
      BytesArray rBytes,
      BytesArray iBytes,
      UInt256 sigma,
      UInt256 tau) {
    boolean[] overflow = new boolean[8];

    long phi = calculatePhi(qBytes, cBytes, rBytes, iBytes);
    overflow[0] = getBit(phi, 0);
    overflow[1] = getBit(phi, 1);

    long psi = calculatePsi(phi, qBytes, cBytes, rBytes, iBytes, sigma);
    overflow[2] = getBit(psi, 0);
    overflow[3] = getBit(psi, 1);
    overflow[4] = getBit(psi, 2);

    long chi = calculateChi(psi, qBytes, cBytes, iBytes, tau);
    overflow[5] = getBit(chi, 0);
    overflow[6] = getBit(chi, 1);
    overflow[7] = getBit(chi, 2);

    return overflow;
  }

  private static long calculatePhi(
      BytesArray qBytes, BytesArray cBytes, BytesArray rBytes, BytesArray iBytes) {
    UInt256 prodPhi = multiplyRange(qBytes.getBytesRange(0, 0), cBytes.getBytesRange(0, 0));
    UInt256 sumPhi =
        prodPhi.add(UInt256.valueOf(iBytes.get(0).toUnsignedBigInteger().shiftLeft(64)));
    sumPhi = sumPhi.add((UInt256.valueOf(rBytes.get(1).toUnsignedBigInteger().shiftLeft(64))));
    sumPhi = sumPhi.add(UInt256.fromBytes(rBytes.get(0)));
    return getOverflow(sumPhi, 2, "phi out of range");
  }

  private static long calculatePsi(
      long phi,
      BytesArray qBytes,
      BytesArray cBytes,
      BytesArray rBytes,
      BytesArray iBytes,
      UInt256 sigma) {

    var sumPsi = UInt256.valueOf(phi);
    sumPsi = sumPsi.add(UInt256.fromBytes(iBytes.get(1)));
    sumPsi = sumPsi.add(UInt256.valueOf(sigma.toUnsignedBigInteger().shiftLeft(64)));
    var prodPsi = multiplyRange(cBytes.getBytesRange(0, 2), qBytes.getBytesRange(0, 2));
    sumPsi = sumPsi.add(UInt256.fromBytes(prodPsi));
    sumPsi = sumPsi.add(UInt256.valueOf(iBytes.get(2).toUnsignedBigInteger().shiftLeft(64)));
    sumPsi = sumPsi.add(UInt256.valueOf(rBytes.get(3).toUnsignedBigInteger().shiftLeft(64)));
    sumPsi = sumPsi.add(UInt256.fromBytes(rBytes.get(2)));
    return getOverflow(UInt256.fromBytes(sumPsi.toBytes()), 4, "psi out of range");
  }

  private static long calculateChi(
      long psi, BytesArray qBytes, BytesArray cBytes, BytesArray iBytes, UInt256 tau) {

    var sumChi = UInt256.valueOf(psi);
    sumChi = sumChi.add(UInt256.fromBytes(iBytes.get(3)));
    sumChi = sumChi.add(UInt256.valueOf(tau.toUnsignedBigInteger().shiftLeft(64)));
    var prodChi = multiplyRange(cBytes.getBytesRange(0, 3), qBytes.getBytesRange(1, 4));
    sumChi = sumChi.add(prodChi);
    sumChi = sumChi.add(UInt256.valueOf(iBytes.get(4).toUnsignedBigInteger().shiftLeft(64)));
    return getOverflow(sumChi, 4, "chi out of range");
  }
}
