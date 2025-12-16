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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.modexp;

import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;

import java.math.BigInteger;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallMemoryContents;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

public class MemoryContents implements PrecompileCallMemoryContents {

  @Override
  public void switchVariant() {
    variant = !variant;
  }

  public final ByteSizeParameter bbs;
  public final ByteSizeParameter ebs;
  public final ByteSizeParameter mbs;
  public final CallDataSizeParameter cds;

  public boolean variant = false;

  public MemoryContents(
      ByteSizeParameter bbs,
      ByteSizeParameter ebs,
      ByteSizeParameter mbs,
      CallDataSizeParameter cds) {
    this.bbs = bbs;
    this.ebs = ebs;
    this.mbs = mbs;
    this.cds = cds;
  }

  public int gasCost() {
    return gasCost(this.bbsShort(), this.ebsShort(), this.mbsShort(), expn());
  }

  private int gasCost(short bbs, short ebs, short mbs, String Exponent) {

    int maxBbsMbs = Math.max(bbs, mbs);
    int fOfMax = f(maxBbsMbs);

    String leadingWord =
        (ebs <= WORD_SIZE) ? Exponent.substring(0, 2 * ebs) : Exponent.substring(0, 2 * WORD_SIZE);
    BigInteger leadingWordBigInt = new BigInteger(leadingWord, 16);
    checkState(leadingWordBigInt.signum() >= 0, "Leading word must be non-negative");
    int floorOfLog = leadingWordBigInt.bitLength() == 0 ? 0 : leadingWordBigInt.bitLength() - 1;
    int lEprime = (ebs <= WORD_SIZE) ? floorOfLog : floorOfLog + 8 * (ebs - WORD_SIZE);
    int maxLEprimeOne = Math.max(lEprime, 1);

    return Math.max(200, (fOfMax * maxLEprimeOne) / 3);
  }

  private int f(int x) {
    int ceil = (x + 7) / 8;
    return ceil * ceil;
  }

  public int memorySize() {
    return 3 * WORD_SIZE + this.bbsShort() + this.ebsShort() + this.mbsShort();
  }

  /**
   * Constructs a well-formed a byte string of the form
   *
   * <p><b>[ bbs | ebs | mbs | BASE | EXPN | MDLS ]</b>
   *
   * <p>where the base <b>BASE</b>, exponent <b>EXPN</b> and modulus <b>MDLS</b> measure <b>bbs</b>,
   * <b>ebs</b> and <b>mbs</b> bytes respectively, and <b>bbs</b>, <b>ebs</b> and <b>mbs</b> are
   * <b>32</b>-byte integers (required to be ≤ 512 = 0x0200 by the arithmetization.)
   *
   * @return
   */
  public BytecodeCompiler memoryContents(ChainConfig chainConfig) {

    // starting at index 2 eliminates the 0x prefix
    String byteSizes = this.bbs().substring(2) + this.ebs().substring(2) + this.mbs().substring(2);

    // every byte is represented by two hexadecimal characters, whence the factor of 2
    String memoryContents =
        byteSizes
            + base().substring(0, 2 * this.bbsShort())
            + expn().substring(0, 2 * this.ebsShort())
            + mdls().substring(0, 2 * this.mbsShort());

    return BytecodeCompiler.newProgram(chainConfig).immediate(Bytes.fromHexString(memoryContents));
  }

  private String base() {
    return variant ? BASE_512_a : BASE_512_b;
  }

  private String expn() {
    return variant ? EXPN_512_a : EXPN_512_b;
  }

  private String mdls() {
    return variant ? MDLS_512_a : MDLS_512_b;
  }

  private short bbsShort() {
    return switch (bbs) {
      case ZERO -> 0;
      case ONE -> 1;
      case SHORT -> 0x1f;
      case MODERATE -> (short) (variant ? 0x01a3 : 0x0101);
      case MAX -> 0x0200;
    };
  }

  private short ebsShort() {
    return switch (ebs) {
      case ZERO -> 0x00;
      case ONE -> 0x01;
      case SHORT -> 0x12;
      case MODERATE -> (short) (variant ? 0xf1 : 0x012a);
      case MAX -> 0x0200;
    };
  }

  private short mbsShort() {
    return switch (mbs) {
      case ZERO -> 0x00;
      case ONE -> 0x01;
      case SHORT -> 0x0d;
      case MODERATE -> (short) (variant ? 0x016f : 0xd1);
      case MAX -> 0x0200;
    };
  }

  private String bbs() {
    return Bytes32.leftPad(Bytes.ofUnsignedShort(bbsShort())).toString();
  }

  private String ebs() {
    return Bytes32.leftPad(Bytes.ofUnsignedShort(ebsShort())).toString();
  }

  private String mbs() {
    return Bytes32.leftPad(Bytes.ofUnsignedShort(mbsShort())).toString();
  }

  private static final String BASE_512_a =
      "15d1ba637430de62c88abfddb0e8d6abc6f5660d2a4293b4184981ef6b9f178701abdb3f9b392b300c1457d8b8b4ce57c2473f301ac199431fd06b5c914438a9eb2315e2f0f0686c2b57e5cca793667dd45935364e59ed920ec3a40d37799f68f49e097f763ca64a8038fe3061b09a6051867eb779e72eb9b9ecc468788ecb29266c1c02ebaafa98bd2c1d69166548ac84d6bfa06a4ca606540e90231bffb53e48ffe7981a43129f8e54bd74dffa2e4024566078bd0e8afe1f1caf44e66b233828b87057fb4d6d8d83e7c682fab9b5c9cfc82142e6f150a41ffdbfa7163bdb210d2274a20e055604d6704228fcc6a05da1bf9487d26c9d791fdd2d48a934328995acbf32eba94c0edbcd8cf8688c45685e1c9f3188d143c9c2ed456c8e68f480fb851f587d3eee03635e4e3cf34e521192513157da5968d776528391f246e650a7f0e8683df8560e6cdbc9e15c562f57ae1b12e17eaaae1181462e702d5ffa9a9fb2d0dc5e02a2ca8d2dcc76dbb5a768fcac1f32a36447b11206e6024a7f72ab81b98738395376cda72fcbad77be565a757703f6dbf0c6a68d9af9f8f0d6f6002e53e995908361792e16b5624a25ecd5c799763b33acd4e5ebec1f8028c1cc903800e781f35b339df589d0f324933e23af337eedaed4273c0a842b41f71691670c02a3f0fd2355690574c5a38237e426da6bb6bd0f0797e502b498e8870d6032";
  private static final String EXPN_512_a =
      "bce258eac6ba5f6ebde23fa3a88c322230f7a104249495389dc9587519f028353ba6ec5cecc8d07d7184749f24894ffb8c41ac83a8ad1409fffe530c72c16d1a91fb7597a2d9796581e77f9199e2595328f543fffdc20db91889ecd20ea260ba6af38466f418630b74f57a032720922508590047aa7292fafd0587d94065df031e3e7ef123c347cb16c822646f4d564b724cb98643b91d1cf56d55672263345f6caea42ca3c25a69a54de58d9a45fef0cb80d650c1e549e6e7ff867023117300f553e2ad3484763e281c6229240d44ceae10177712e02a57f32f2cd9a7f8d0ef5fdeb79ea629af3614c8ce0fcdedb4c3d6ab6c1afeac40105c34eb4057b22342df320e21a619835e792d8e2701b759157d4ef0b7872fad66372481c124118ff54db223a476ffb69842f90d5b66ad1dfb4fc86e6c8ffdd6a324051c0ec69c1d99d38d5ff8a12f1e1b724833bdcd9171c30f191b99992233f9a35476d78efecf4e99ddf05c83dd6bdf43c8bd5244657c3493d6035c713fa93976960dc62526506c91593c8edc7f8d10bdc041da70e4cc5eff830cb65b1d2cffef2a147696bcccae23c2a2c07dff40e671b0d16fcda339ca0a00b71124a52978370025fe5d67949b1f9ca20230e3b83b74345c77f835626cbce252eb48a6eb89f16f8cd2f3b36ef84d41f3d1b97e9a4e3a0edee1d41f9b8ca11abbd1b436a713c4c8ef17f38764ad";
  private static final String MDLS_512_a =
      "e92a1304e5e9dfdcb0cb08acddcc25691f84581794b48daace8202718341e071cc98b84d4e0a7102958b5a4b85e9487cf6bd8c03e1cc70eadf5dee58e2caf2d738c6bcce89e59024f16a481bc32252cbbf17183c899b52182b01645500a9a56c06b2807c259100072500eacb12c9aecda64ce58d6b4ef7d335aaeffd1c794440d3fe7d8eba6d5b9cfbd7b4178dabf2045fd1cde509b7c8d9bcb77e3dbfb9ecd8fdaa2db10f24a765485cf455ae221637ef859577e6d5cf18e5fe11c736037b44dda3da3c37bd116d0c775c41f74993dcd503cc23d0b178c22d5e38e776f4c72b4391ddd15b3dc95dd3525cdd58227d86f5e2ca5b661fa18fd9556df478d2ac5719bbd8657e0555799f531f85a07ba174e8232d123a132e65bfa1e622bfa77034b45ebb57359a38721da39365bfec4a889f0b0d13e2971019cf6f14ae2d959212502cfef1e69d8238b4972c898895f2783db45e5410f15a648a02de3ce40e264d24ee409aa2296e0cf43a749cb39d6342d68b1e85aca442a4f4af67295ee06a90d74f10ef19a2698def6deecf19784c8e511165aeec3009c6effd8e9aee1cf7b9bb23e4266ba62eab3ac2f3796cecbdf800dae6f74c0c9abcf227b4bdb1e5a10f6dba2284934fe7593f8058d5a74113baba19b6a59cd371dc0669ac48af4093f02d10d947239fceaf124860b4d123878b77564ea491b543c89417c9c3f2d39527";

  private static final String BASE_512_b =
      "4c594c0a14b26df4357980b0e3daf7b4c7e5c85c703f529a0a5d7a3bda9854d738b54bad23c6e9c4674b503d17ad60dda0fc68789aebfadf64e1c0b4f5e0917edbace443548bfb65755828da15e17b0c672c05d0de0f0d516ef2b1b23842fdcd96a181eff693851ef41cfbdab3eb1664cee9d0ad7fc78a70aa112f71f4a3311eda30a1c1adc62251154382b555df4b22aabef45d8dfb417262bb5da9a45ba5d6bcb1542f24cddb58b1fc56102a57f296fe125bf5ead5b115b2e7317f7950e869fa19e7becd516015428f0e67da4aaca0ccd7cfe6e00072f9b63830ffac60e13cf8e4b31db0f2464f35c9f1650f1edcc3ac424a9e7f3509e270a606428209d7522f3a7e35ebef7cb267c87cf8805c48eba6cca87ec44e81112ad25ef72d487be71cb6a021eb7ded009d3f7536a6f47d9362fdf22f0b01c9071f0ae05b3afc04574dce57f9219d821296f5067c5814bd95a22cf1b4d4aec3bceaea8ccb6abb318228306e9c6433795241796a844403dea79be8e5df88fb54134f73ffddfbae3b6e995fb582c5638a17cd7c7a957dc0ad36ec5e04815ec293f544bb13ad73270eafb5220c02173d8ff78ea774887ef2a119f7625d4866d131a05a9c4df2537a56bd623ea338f5b2aeb026c395318d7957a29558a4844a0a7ca31ad77ab0dc77a5d31d54df598a2ac0fad9db855ab299c0546ca2545729ae41584b82b55d6b061f9c";
  private static final String EXPN_512_b =
      "0003e59f9eb9de451161f0f2a5bfebcce1422179e69258468b74a98fcca79f2a156eae7aaa5418b2bc45b33d643cee08b1228ebf08ea29995019689ffad602473c52276d91d04abe3bdeb0f48324da2ddd43e6dcdde860c6ca9dd9d8232b0b4db931b2391a86b20400aaa96e085b6b90b8a562a7fa6a052a389660e99169ca065e12b939b0647e0ae6c762d61e0739ee06f862abbd5fe72f46092fb15505114194fc82435cacb1500a1b6a677200101809e5e701e7bd2385853183e3d5b2167a5f006d801940c620ec7d8763084272a76a23b8c401adee3efa424fa1702f1b936fa0d6017328339f54e1aa64919aaaff6333fe148c60ad57de2f62c2d05b2b73aef9f18088cb3141d7be05739723303b1761aaa9459507f1cf5118fc306e0854bf42f9ded226417f7ccae38e4222239236d4f33edff6e53012f4e0b0a8316b966aa75908dcb40a703c2688cffb33a940851556dfc08b151461726c88293b9f77199c5c9e4dec70fbf3ef0d84e2328ab48e15fb20cc0aa8dba18b2ddf901de27c4af3d74bcbd1ff606fe0434218c6e3966657bd0f375f8aad5ed178887e3d37159d109cf30ee961a20090d17e33075c9da626402a04901c7364abc222dcd883f65a72e649972a23f8b55c79515e8f5b8c0149fc816c91214fda785bce80042c0170c5c0a1467a569a61f380eb47e0b299d2848bb2b7ae22cfe472a4e5a3614fc6";
  private static final String MDLS_512_b =
      "52c17a8a52b76004c769c98ae510914b17c516f5d9b1fecf0a3b4b89c0f4c415ef31699855362601b4991b2b3ced962c7c23e8ea958ffc28e59711d270cebf7709de66cb29f8e6dcf7161745529444073dbce14d5a99dea405da9472d66d26abacba626e1c797c155a8119058afbdfd2923cb91cd18c37ebfc736d0c0457041c24822031bea6b09e3b8e9b7f1f724121be221d399ef7f2070b16b3b37b953bc19ab46cafb13a742b185e1a126f22d89373a8b09c57de4d7c5a556e6bc9de527134b7b1b7fd6ad883889f96f3253b2b90de4cf82dc35d1b839fd0ca308cd7e4d950d2890a6af5c7f7fbff9a11892cdd90f058e2e719ec91d990c9f53e19607f7a7a105be37083be8e6ec78fed09170399a58963dc5f2d0457f1b311036eba270b44e88bbf5f3b0704d79e37e4ed9a901416a8495f502c39bdfe5aded946ae55f59b6285fc05b8e0ea50710e50cbf896001cb1fdc0c8bbbc3629f8a901dc3d3c3d5aff0112b2adae83119692f12b4c1f36e1b0628d1522804c15360ea9413a702079412a29f60c578052f7cf02a86b28a9d8e95785fa6d7d2fb2964e8d25489ff0077fa5776ac32bef08bf84a7f172ab21ac8817b79bf83a565209820193cd307451dc6f5a01f29e795e27b4d82a87d8a6eeeab3fa06c914b0e05c240acf9ac51a7f863ec8fb246d78b576ffa4426ff31dbe08dd8d81ce2fe69c191c8a13486e51";
}
