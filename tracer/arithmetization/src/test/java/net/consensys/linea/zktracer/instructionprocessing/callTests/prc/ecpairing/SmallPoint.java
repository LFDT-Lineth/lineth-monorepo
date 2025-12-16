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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecpairing;

import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecadd.MemoryContents.WORD_HEX_SIZE;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecpairing.MemoryContents.C1_POINT_4;

public enum SmallPoint {
  // valid points
  INFINITY,
  VALID_SMALL_POINT,
  // invalid points
  // RAND,
  X_NOT_IN_FIELD,
  Y_NOT_IN_FIELD,
  NOT_ON_CURVE;

  public boolean isValid() {
    return this == INFINITY || this == VALID_SMALL_POINT;
  }

  public boolean isInfinity() {
    return this == INFINITY;
  }

  public String hexString() {
    final String result =
        switch (this) {
          case INFINITY -> SMALL_POINT_AT_INFINITY;
          case VALID_SMALL_POINT -> C1_POINT_4;
          case X_NOT_IN_FIELD -> IN_RANGE_SAVE_FOR_LARGE_X;
          case Y_NOT_IN_FIELD -> IN_RANGE_SAVE_FOR_LARGE_Y;
          case NOT_ON_CURVE -> IN_RANGE_BUT_NOT_ON_CURVE;
          // case RAND -> RANDOM_HEX_STRING.substring(131, 131 + 2 * WORD_HEX_SIZE);
          default -> throw new IllegalArgumentException("Invalid point candidate");
        };
    checkState(result.length() == 2 * WORD_HEX_SIZE, "Invalid hex string length");

    return result;
  }

  // All values below are obtained from Ivo's comment
  // https://github.com/Consensys/linea-tracer/issues/822#issuecomment-2260511164

  public static final String SMALL_POINT_AT_INFINITY = "00".repeat(2 * WORD_SIZE);
  public static final String IN_RANGE_SAVE_FOR_LARGE_X =
      "b3a1cca4c86cdc017597bb5e39705666199eccabc367f8c6aa5e713921c0886e"
          + "2018c3b9c5993f6e66a53edf1524775bd337cd82931b44760bdb4e0f557fcf55";
  public static final String IN_RANGE_SAVE_FOR_LARGE_Y =
      "29c5b47fbe82856ac08cfc5e72ee76f9ca909a4360ceafdbb058792b4dea2380"
          + "ced846e53d3564f9d059532c94207b64cba68393988bb0bd26711c828f68cd64";
  public static final String IN_RANGE_BUT_NOT_ON_CURVE =
      "1dac6eed25388228c5542dfc2c0b30bd5afbb4fb091791052ad8ca6a8e1c94fb"
          + "0b35150d9fe2b8cff4e81fd7445b6d529f68ca798d16618adf0f1d319e772987";

  public static final String RANDOM_HEX_STRING =
      "5b715e0b8ee221775e11207b0e08b43ac5c2005a980303465dd90d321499242d6245977f7b7b2cbfe4275637425e058731b66ab18907df88f3e76e207ede232238443a4e0f7dd7f72381ecb6005f4ca18ed70e5773c511568ad10dbf079f4f4b39e0265c45f23e3d8aaa6bd1119a63663af56960cf29c5af84b419a5f3c61fe1acba71b7b83528c3d35053516b4ed779638c2b3c82b07f0755d488e635a84d925eab59be05280155242d2481493d0ab85bbe1386328b54893149b2c02c2ac61a9397b12318adcc280249e8b8e2eb8ca8b06c41abd89efa24ef836b38a686e5242cd98fa00fec1500ac439697e38b79293c580bf731f273dd8b3e16a0c37db08b14436f9c0d009a81fca876bf0e08fd60cbcde26f2caeea76e135015d48d20813d15979b635a3173ecc3fae9c6a1f3edc38537a12327a1a79f38d8bba974ec7279504762c2a86f21015e20e1ff21c601896c232b1fdc998df42f71cd6863c3c7eb7fe8867fc531a817873892e6793f42ad6a37f3148cb34dd8bc05b83296ea9c7b5199dbb498cc11dea8444823f51c553725516adb00d63b20aa09ba2c445aee03484765c95b8b1969dc0688bec46594b43b3d4636f79da7fd0d03256249d8ae9a1b3ba040a242dfc11b93994503afbb9e7dcec5982c8e75346cbb0fc07afd992e5659a570ebedbcdd4bdd5f52db9b823c5a2a974ba97fc036f3cbaf7aeaeebe0";
}
