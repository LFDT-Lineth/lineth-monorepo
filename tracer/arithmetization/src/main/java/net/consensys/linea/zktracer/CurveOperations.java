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

package net.consensys.linea.zktracer;

import java.math.BigInteger;
import java.util.Optional;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.bytes.MutableBytes;
import org.hyperledger.besu.crypto.SECPPublicKey;
import org.hyperledger.besu.crypto.SECPSignature;
import org.hyperledger.besu.crypto.SignatureAlgorithm;
import org.hyperledger.besu.crypto.SignatureAlgorithmFactory;
import org.hyperledger.besu.crypto.altbn128.AltBn128Fq2Point;
import org.hyperledger.besu.crypto.altbn128.AltBn128Point;
import org.hyperledger.besu.crypto.altbn128.Fq;
import org.hyperledger.besu.crypto.altbn128.Fq2;

public class CurveOperations {
  private static final EWord P =
      EWord.ofHexString("0x30644e72e131a029b85045b68181585d97816a916871ca8d3c208c16d87cfd47");
  private static final BigInteger Q =
      new BigInteger("30644E72E131A029B85045B68181585D2833E84879B9709143E1F593F0000001", 16);

  private static final int V_BASE = 27;
  private static final SignatureAlgorithm signatureAlgorithm =
      SignatureAlgorithmFactory.getInstance();

  public static BigInteger extractParameter(final Bytes input) {
    if (input.isEmpty()) {
      throw new IllegalArgumentException("EC_DATA input cannot be empty");
    }
    return new BigInteger(1, input.toArray());
  }

  public static boolean isOnC1(Bytes xBytes) {
    final BigInteger pX = extractParameter(xBytes.slice(0, 32));
    final BigInteger pY = extractParameter(xBytes.slice(32, 32));

    if (pX.compareTo(Fq.FIELD_MODULUS) >= 0 || pY.compareTo(Fq.FIELD_MODULUS) >= 0) {
      return false;
    }

    final AltBn128Point p = new AltBn128Point(Fq.create(pX), Fq.create(pY));

    return p.isOnCurve();
  }

  public static boolean isOnG2(Bytes xBytes) {
    final BigInteger pXIm = extractParameter(xBytes.slice(0, 32));
    final BigInteger pXRe = extractParameter(xBytes.slice(32, 32));
    final BigInteger pYIm = extractParameter(xBytes.slice(64, 32));
    final BigInteger pYRe = extractParameter(xBytes.slice(96, 32));

    final Fq2 pX = Fq2.create(pXRe, pXIm);
    final Fq2 pY = Fq2.create(pYRe, pYIm);

    final AltBn128Fq2Point p2 = new AltBn128Fq2Point(pX, pY);
    return p2.isOnCurve() && p2.isInGroup();
  }

  public static boolean ecRecoverSuccessful(final Bytes input) {
    final int size = input.size();
    final Bytes d = size >= 128 ? input : Bytes.wrap(input, MutableBytes.create(128 - size));
    final Bytes32 h = Bytes32.wrap(d, 0);
    // Note that the Yellow Paper defines v as the next 32 bytes (so 32..63). Yet, v is a simple
    // byte in ECDSARECOVER and the Yellow Paper is not very clear on this mismatch, but it appears
    // it is simply the last byte of those 32 bytes that needs to be used. It does appear we need
    // to check the rest of the bytes are zero though.
    if (!d.slice(32, 31).isZero()) {
      return false;
    }

    final int recId = d.get(63) - V_BASE;
    final BigInteger r = d.slice(64, 32).toUnsignedBigInteger();
    final BigInteger s = d.slice(96, 32).toUnsignedBigInteger();

    final SECPSignature signature;
    try {
      signature = signatureAlgorithm.createSignature(r, s, (byte) recId);
    } catch (final IllegalArgumentException e) {
      return false;
    }

    // SECP256K1#PublicKey#recoverFromSignature throws an Illegal argument exception
    // when it is unable to recover the key. There is not a straightforward way to
    // check the arguments ahead of time to determine if the fail will happen and
    // the library needs to be updated.
    try {
      final Optional<SECPPublicKey> recovered =
          signatureAlgorithm.recoverPublicKeyFromSignature(h, signature);
      return recovered.isPresent();
    } catch (final IllegalArgumentException e) {
      return false;
    }
  }
}
