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

import java.math.BigInteger;
import java.util.List;

public class BlsTestUtils {

  // Valid G1 point - 128 bytes (256 hex chars)
  static final String VALID_G1_POINT =
      "0000000000000000000000000000000012196c5a43d69224d8713389285f26b98f86ee910ab3dd668e413738282003cc5b7357af9a7af54bb713d62255e80f56"
          + "0000000000000000000000000000000006ba8102bfbeea4416b710c73e8cce3032c31c6269c44906f8ac4f7874ce99fb17559992486528963884ce429a992fee";

  // Invalid G1 point (not on curve)
  static final String INVALID_G1_POINT_NOT_ON_CURVE =
      "00000000000000000000000000000000177b39d2b8d31753ee35033df55a1f891be9196aec9cd8f512e9069d21a8bdbf693bd2e826e792cd12cb554287adf4ca"
          + "0000000000000000000000000000000003c0f5770509862f754fc474cb163c41790d844f52939e2dec87b97c2a707831a4043ab47014d501f67862e95842ba5a";

  // G1 point that's on curve but not in subgroup
  static final String G1_POINT_NOT_IN_SUBGROUP =
      "00000000000000000000000000000000054a4326bbddbdbbca126659e6686984046d2fa49270742e5b6d9017734acf2801f370eebe7af29dfc8d50483609dc00"
          + "000000000000000000000000000000001713e9ef64254fe96d874d16e33636f186e30d7e476db9f49a16698b771f10e0f8f08e5d8dba621b887c0d257cbd8eac";

  static final List<String> SMALL_POINTS =
      List.of(VALID_G1_POINT, INVALID_G1_POINT_NOT_ON_CURVE, G1_POINT_NOT_IN_SUBGROUP);

  // Valid G2 point - 256 bytes (512 hex chars)
  static final String VALID_G2_POINT =
      "00000000000000000000000000000000124aca13d9ead2e5194eb097360743fc996551a5f339d644ded3571c5588a1fedf3f26ecdca73845241e47337e8ad990"
          + "000000000000000000000000000000000299bfd77515b688335e58acb31f7e0e6416840989cb08775287f90f7e6c921438b7b476cfa387742fcdc43bcecfe45f"
          + "00000000000000000000000000000000032e78350f525d673e75a3430048a7931d21264ac1b2c8dc58aee07e77790dfc9afb530b004145f0040c48bce128135e"
          + "0000000000000000000000000000000015963bcbd8fa50808bdce4f8de40eb9706c1a41ada22f0e469ecceb3e0b0fa3404ccdcc66a286b5a9e221c4a088a9145";

  // Invalid G2 point (not on curve)
  static final String INVALID_G2_POINT_NOT_ON_CURVE =
      "000000000000000000000000000000000b2c619263417e8f6cffa2e53261cb8cf5fbbabb9e6f4188aeaabe50d434a0489b6cccd2b65b4d1393a26911021baffa"
          + "00000000000000000000000000000000007bcd4156af7ebe5e2f6ac63db859c9f42d5f11682792a0de2ec1db76648c0c98fdd8a82cf640bdcd309901afd4f570"
          + "00000000000000000000000000000000153a9002d117a518b2c1786f9e8b95b00e936f3f15302a27a16d7f2f8fc48ca834c0cf4fce456e96d72f01f252f4d084"
          + "000000000000000000000000000000001091fc53100190db07ec2057727859e65da996f6792ac5602cb9dfbc3ed4a5a67d6b82bd82112075ef8afc4155db2621";

  // G2 point that's on curve but not in subgroup
  static final String G2_POINT_NOT_IN_SUBGROUP =
      "000000000000000000000000000000000380f5c0d9ae49e3904c5ae7ad83043158d68fa721b06b561e714b71a2c48c2307b5258892f999a882bed3549a286b7f"
          + "0000000000000000000000000000000004886f7f17a8e9918b4bfa8ebe450b0216ed5e1fa103dfc671332dc38b04ed3105526fb0dda7e032b6fb67debf9f0bc5"
          + "0000000000000000000000000000000018146b7ed1ecf2a4f2d1f75bb6e9ddbb9796bb03576686346995566cf3b3831ec5462e61028355504fc90f877408ac17"
          + "0000000000000000000000000000000003da9de8dcd94d7793b19e45a5521b1bc42f1a6d693139d03bb26402678ee6a635a4d50eaddfd326e446ed0330fa67fb";

  static final List<String> LARGE_POINTS =
      List.of(VALID_G2_POINT, INVALID_G2_POINT_NOT_ON_CURVE, G2_POINT_NOT_IN_SUBGROUP);

  static final BigInteger BLS_PRIME =
      new BigInteger(
          "1a0111ea397fe69a4b1ba7b6434bacd764774b84f38512bf6730d2a0f6b0f6241eabfffeb153ffffb9feffffffffaaab",
          16);

  // Fp elements testing data
  static final List<String> leadSuccess = List.of("00".repeat(16));
  static final List<String> leadFailure =
      List.of(
          "ff".repeat(16),
          "10" + "00".repeat(15),
          "00".repeat(13) + "eeff00",
          "00".repeat(15) + "01");
  static final List<String> tailSuccess =
      List.of(
          BLS_PRIME.subtract(BigInteger.ONE).toString(16),
          "00".repeat(45) + "aabbcc",
          "00".repeat(47) + "01",
          "00".repeat(48));
  static final List<String> tailFailure =
      List.of(
          "ff".repeat(48),
          BLS_PRIME.toString(16),
          BLS_PRIME.add(BigInteger.valueOf(123)).toString(16));
}
