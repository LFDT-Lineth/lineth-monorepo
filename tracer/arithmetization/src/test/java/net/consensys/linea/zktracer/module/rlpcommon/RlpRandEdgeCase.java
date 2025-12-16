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

package net.consensys.linea.zktracer.module.rlpcommon;

import java.math.BigInteger;
import java.util.Random;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class RlpRandEdgeCase extends TracerTestBase {
  /**
   * NOTE: Do not make this static as it will introduce non-deterministic behaviour into the testing
   * process.
   */
  private final Random RAND = new Random(666);

  public BigInteger randBigInt(boolean onlyFourteenByte) {
    final int selectorBound = onlyFourteenByte ? 4 : 5;
    int selector = RAND.nextInt(0, selectorBound);

    return switch (selector) {
      case 0 -> BigInteger.ZERO;
      case 1 -> BigInteger.valueOf(RAND.nextInt(1, 128));
      case 2 -> BigInteger.valueOf(RAND.nextInt(128, 256));
      case 3 -> new BigInteger(14 * 8, RAND);
      case 4 -> new BigInteger(32 * 8, RAND);
      default -> throw new IllegalStateException("Unexpected value: " + selector);
    };
  }

  public Bytes randData(boolean nonEmpty) {
    final int maxDataSize = 1000;
    int selectorOrigin = 0;
    if (nonEmpty) {
      selectorOrigin += 1;
    }
    int selector = RAND.nextInt(selectorOrigin, 6);
    return switch (selector) {
      case 0 -> Bytes.EMPTY;
      case 1 -> Bytes.of(0x0);
      case 2 -> Bytes.minimalBytes(RAND.nextLong(1, 128));
      case 3 -> Bytes.minimalBytes(RAND.nextLong(128, 256));
      case 4 -> Bytes.random(RAND.nextInt(1, 56), RAND);
      case 5 -> Bytes.random(RAND.nextInt(56, maxDataSize), RAND);
      default -> throw new IllegalStateException("Unexpected value: " + selector);
    };
  }

  public Long randLong() {
    int selector = RAND.nextInt(0, 4);
    return switch (selector) {
      case 0 -> 0L;
      case 1 -> RAND.nextLong(1, 128);
      case 2 -> RAND.nextLong(128, 256);
      case 3 -> RAND.nextLong(256, 0xfffffffffffffffL - 2);
      default -> throw new IllegalStateException("Unexpected value: " + selector);
    };
  }
}
