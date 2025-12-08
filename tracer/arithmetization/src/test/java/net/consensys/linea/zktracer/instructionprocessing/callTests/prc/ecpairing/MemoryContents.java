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
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecpairing.LargePoint.LARGE_POINT_AT_INFINITY;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecpairing.SmallPoint.SMALL_POINT_AT_INFINITY;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallMemoryContents;
import org.apache.tuweni.bytes.Bytes;

/**
 * Memory contents for the <b>ECPAIRING</b> precompile take on the following form:
 *
 * <p><b>[ λ∞ | ∞∞ | -P | ∞ρ | P | λ∞ | Q | R | ∞ρ | CENTER_PAIR | ∞ρ | S | ff .. ff ]</b>
 *
 * <p>where <b>CENTER_PAIR = τθ</b> and:
 *
 * <ul>
 *   <li><b>∞</b> represents the point at infinity one either curve
 *   <li><b>λ</b> is a nontrivial and valid C1 point
 *   <li><b>ρ</b> is a nontrivial and valid G2 point
 *   <li><b>P, Q, R, S</b> are nontrivial pairs of points such that
 *       <ul>
 *         <li><b>"-P"</b> (non standard notation) negates the C1 point in <b>P</b>
 *         <li>for {@link #variant} ≡ <code>true</code> <b>e(P)∙e(Q)∙e(R) ≡ 1</b>
 *         <li>for {@link #variant} ≡ <code>false</code> <b>e(P)∙e(Q)∙e(R)∙e(S) ≡ 1</b>
 *       </ul>
 *   <li><b>τ</b> is a {@link SmallPoint} which will vary
 *   <li><b>θ</b> is a {@link LargePoint} which will vary
 * </ul>
 *
 * <b>Note.</b> The <b>CENTER_PAIR</b> is the only component in this setup which may cause failure
 * in the precompile call.
 *
 * <p><b>Note.</b> The trailing <b>ff .. ff</b> is where we will place the <b>returnAt</b> memory
 * range.
 */
public class MemoryContents implements PrecompileCallMemoryContents {
  public static final int TOTAL_NUMBER_OF_PAIRS_OF_POINTS = 12;
  public static final int SIZE_OF_PAIR_OF_POINTS = 192;
  private final String RETURN_DATA_STRIP = "ff".repeat(WORD_SIZE);

  public final SmallPoint small;
  public final LargePoint large;

  public boolean centerPairIsValid() {
    return small.isValid() && large.isValid();
  }

  public boolean centerPairIsInconsequential() {
    return centerPairIsValid() && (small.isInfinity() || large.isInfinity());
  }

  public MemoryContents(SmallPoint small, LargePoint large) {
    this.small = small;
    this.large = large;
  }

  boolean variant = false;

  @Override
  public void switchVariant() {
    variant = !variant;
  }

  @Override
  public BytecodeCompiler memoryContents(ChainConfig chainConfig) {
    Bytes memoryContentsBytes =
        Bytes.fromHexString(leftPairs() + CenterPair() + rightPairs() + RETURN_DATA_STRIP);
    checkState(
        memoryContentsBytes.size()
            == TOTAL_NUMBER_OF_PAIRS_OF_POINTS * SIZE_OF_PAIR_OF_POINTS + WORD_SIZE,
        "ECPAIRING memory contents size is incorrect");

    BytecodeCompiler memoryContents = BytecodeCompiler.newProgram(chainConfig);
    return memoryContents.immediate(memoryContentsBytes);
  }

  private String leftPairs() {
    return (C1_POINT_1 + LARGE_POINT_AT_INFINITY)
        + (SMALL_POINT_AT_INFINITY + LARGE_POINT_AT_INFINITY)
        + NEGATIVE_P
        + (SMALL_POINT_AT_INFINITY + G2_POINT_1)
        + (variant ? TRIPLE_P : QUADRUPLE_P)
        + (C1_POINT_2 + LARGE_POINT_AT_INFINITY)
        + (variant ? TRIPLE_R : QUADRUPLE_R)
        + (variant ? TRIPLE_Q : QUADRUPLE_Q)
        + (SMALL_POINT_AT_INFINITY + G2_POINT_2);
  }

  private String CenterPair() {
    return small.hexString() + large.hexString();
  }

  private String rightPairs() {
    return (SMALL_POINT_AT_INFINITY + G2_POINT_3)
        + (variant ? C1_POINT_3 + G2_POINT_4 : QUADRUPLE_S);
  }

  // All values below are obtained from Ivo's comment
  // https://github.com/Consensys/linea-tracer/issues/822#issuecomment-2260511164

  private static final String C1_POINT_1 =
      "26d7d8759964ac70b4d5cdf698ad5f70da246752481ea37da637551a60a2a57f"
          + "13991eda70bd3bd91c43e7c93f8f89c949bd271ba8f9f5a38bce7248f1f6056b";
  private static final String C1_POINT_2 =
      "1760ca14c35b8978c10ba226b4654e4c925218417ec23731c29da60f481c2c0a"
          + "16206149ae732094afbc9921444e04cae6e093a0e73f3212a5e9f93a04f7f07a";
  private static final String C1_POINT_3 =
      "214eb4ed76fa9ea509001fa6d4a1ddc86ac42da639fba6e4956b4045dd74fa26"
          + "0a847e7fee1a9b1c6166724ec7eea284fd71506e31371674164f860ac641b0e4";
  public static final String C1_POINT_4 =
      "1cbcc5ae2ad1e062ffbfe7858b0f962d24656075da7f168779afb5167da8e946"
          + "1504db20d014e62edf9b2105d8fc6c62919351144cfdd8c6e12c3290d9903a79";
  private static final String C1_POINT_5 =
      "117e59d40acc2608b961101994f76516f4cd4204af85de7d499c2b9043e0d771"
          + "2c76b5677261cc2851a41b75f03497ef99522c1c29a4f4d2aba921997cec664a";

  private static final String G2_POINT_1 =
      "13eb8555f322c3885846df8a505693a0012493a30c34196a529f964a684c0cb2"
          + "18335a998f3db61d3d0b63cd1371789a9a8a5ed4fb1a4adaa20ab9573251a9d0"
          + "20494259608bfb4cd04716ba62e1350ce90d00d8af00a5170f46f59ae72d060c"
          + "257a64fbc5a9cf9c3f7be349d09efa099305fe61f364c31c082ef6a81c815c1d";
  private static final String G2_POINT_2 =
      "2f1e9fe1d767c3ee1f801d43e4238a28cb4f94d3e644b2c43e236a503eade386"
          + "14675488459758a6e2f0bebcf5cd4c70c0c6d9733575c999a09418ae773ae6b5"
          + "0ae82edaac30d3ff28ea0c4d8e43e39622b47a19beb334a4216994e3a98a796a"
          + "237392d458d5e3a479ebb4feac87c4371150ea86ee61f3268f5b65bca02daa4a";
  private static final String G2_POINT_3 =
      "210c2d4972a76beee46732b60ed998e2e60769847350553f274523a7a9b23d17"
          + "0dd0cf966d5a0ef6d560e7dc949b48839e14e26bb8e22d08029b17e1f916de8b"
          + "22a8f09fd8c34454c77a7aeb7d7d6e00e160cdf03700c1e503b7e0cabf9aba3c"
          + "2090ce9e959dcf086d024dd626111301fd559aab1b4a475bb4c59cd7f598af83";
  private static final String G2_POINT_4 =
      "15746a21d15b2ffb960a9cb93426fc05f20921467c23f89494966d13e1507e87"
          + "29670f195cd081b64b28dae6420ad919a7ca8a5c3a0ae5c313420cd4aca339af"
          + "056e6142247149ea3505c5adbd6f3d1af9f51d50fdd2cdcd637221ee9ef80a3c"
          + "025f9828f38707d5cb94e8905be933d4ae03e7f084e250a2c65f0c856035a2a4";
  public static final String G2_POINT_5 =
      "22007e1404f2d2f0a9b676095daef5b4d49be05df224ef98a2cd00da756900d9"
          + "1d8281748ef6cd4dd40149ce6f2afbcc26f3d2499cb484b0a4c21872f9816171"
          + "191fc3a7eb19e1f750c5c3dc7a008c8d35dc08feb1de5dd24f293bc2fd84a739"
          + "16dca8731c250e792423469ab23a2f7d4891d55d43da021f6ce572f268e2cab9";

  private static final String NEGATIVE_P =
      "05dcb6449ff95e1a04c3132ce3be82a897811d2087e082e0399985449942a45b"
          + "0cb5122006e9b7ceb5307fa4015b132b3945bb972c83459f598659fc4b5a9d32"
          + "127a664dd11342beb666506dac296731e404de80a25e05f40b2405c4c00c28fc"
          + "2bd236cb7a7b0e0543e6b6e0d7308576aeeec4dea2f740654854215d7813826f"
          + "1b28f411c2931b52b2ad62de524be4eaac555dfed67d59e2d0f6c4607b23526b"
          + "181c4319cc974dd174c5918ac1892326badb2603a04bc8f565221c06eec8a126";

  private static final String TRIPLE_P =
      "01395d002b3ca9180fb924650ef0656ead838fd027d487fed681de0d674c30da"
          + "097c3a9a072f9c85edf7a36812f8ee05e2cc73140749dcd7d29ceb34a8412188"
          + "2bd3295ff81c577fe772543783411c36f463676d9692ca4250588fbad0b44dc7"
          + "07d8d8329e62324af8091e3a4ffe5a57cb8664d1f5f6838c55261177118e9313"
          + "230f1851ba0d3d7d36c8603c7118c86bd2b6a7a1610c4af9e907cb702beff1d8"
          + "12843e703009c1c1a2f1088dcf4d91e9ed43189aa6327cae9a68be22a1aee5cb";
  private static final String TRIPLE_Q =
      "05dcb6449ff95e1a04c3132ce3be82a897811d2087e082e0399985449942a45b"
          + "0cb5122006e9b7ceb5307fa4015b132b3945bb972c83459f598659fc4b5a9d32"
          + "12618811f3e9fa06644d43cfe3f69c6c17a738128de60f8f3ebb4266bab29be6"
          + "00a5a6c2ec01c4d1374078ae1bbea91dea8e938c1275226a1ce51db5e7de53d1"
          + "2da43ecc11a0095a72454bb08fb4d1116facadcab482a1107ae67a12bb3c19f2"
          + "1e2f128bf79945a370324b82c36c1e63509b122c023bd8163495526bb030a216";
  private static final String TRIPLE_R =
      "1296d042f33ccbb814746e187aa20af49bd503356de4846abee08da9e32ae2ac"
          + "0b980019d2af83b353aa8c2efda45f16ce523b99452118be7ae5dd1e92e0e4ec"
          + "012cd1b1242354b35cd8d2493c487b52411d111c80e8cfb97080e2db1af5f705"
          + "20ca232ed2582feeca2d56a589eec30c27075a44aced8382d87cd43c011aeeb0"
          + "2df7d0d9ae47467ca500b528f38ac38433885d6a59db6ecd7d27d37145e75360"
          + "1e4d8d4d8878cad4de8dbb31ec11d1ecd7b40617c46bb7189ccab201b9bdbaab";

  private static final String QUADRUPLE_P =
      "01395d002b3ca9180fb924650ef0656ead838fd027d487fed681de0d674c30da"
          + "097c3a9a072f9c85edf7a36812f8ee05e2cc73140749dcd7d29ceb34a8412188"
          + "2bd3295ff81c577fe772543783411c36f463676d9692ca4250588fbad0b44dc7"
          + "07d8d8329e62324af8091e3a4ffe5a57cb8664d1f5f6838c55261177118e9313"
          + "230f1851ba0d3d7d36c8603c7118c86bd2b6a7a1610c4af9e907cb702beff1d8"
          + "12843e703009c1c1a2f1088dcf4d91e9ed43189aa6327cae9a68be22a1aee5cb";
  private static final String QUADRUPLE_Q =
      "05dcb6449ff95e1a04c3132ce3be82a897811d2087e082e0399985449942a45b"
          + "0cb5122006e9b7ceb5307fa4015b132b3945bb972c83459f598659fc4b5a9d32"
          + "12618811f3e9fa06644d43cfe3f69c6c17a738128de60f8f3ebb4266bab29be6"
          + "00a5a6c2ec01c4d1374078ae1bbea91dea8e938c1275226a1ce51db5e7de53d1"
          + "2da43ecc11a0095a72454bb08fb4d1116facadcab482a1107ae67a12bb3c19f2"
          + "1e2f128bf79945a370324b82c36c1e63509b122c023bd8163495526bb030a216";
  private static final String QUADRUPLE_R =
      "1296d042f33ccbb814746e187aa20af49bd503356de4846abee08da9e32ae2ac"
          + "0b980019d2af83b353aa8c2efda45f16ce523b99452118be7ae5dd1e92e0e4ec"
          + "0cdc14ed1231209df350fbf71c359fa338955c8e4b10678b33237e54473e7b0f"
          + "2c5be308b741fee6607eea4980779483339372c80494a43189ab6613f2ac6b00"
          + "0a56a2a107cc154cade228f3da75a714a7de0738b9ebd455f3f73c4c3c43136b"
          + "1d98bf24e00f830334bdf3334135c5aa55b6fd4e88e87b1aee72fb1550970879";
  private static final String QUADRUPLE_S =
      "0baab17525d29e5d7c34a5cdb6558e8427f5c79206e009b4b1e8b91a4c827f1c"
          + "18cb7444434d59d126ccc48244daa9a3709140bfa58bff81751d42b647211a91"
          + "27f28ec95a4937c625a4fa28551ba8b7010511313699acb72d5eb0a0a208df2b"
          + "2e04f5f992061adf169ef7423bc2f8748ca8fe437036e44a5a24b32bdbae8754"
          + "056d39d4c664d7bf3417673a870f94749311be2f4e4c5fdfe6563b1593fcc00e"
          + "18b849ea2b045d9b94d2e1d8daa843ef2c98577bb7c98d6004eb8982f561fd0f";

  @Override
  public String toString() {
    return "MemoryContents{" + "small=" + small + ", large=" + large + '}';
  }
}
