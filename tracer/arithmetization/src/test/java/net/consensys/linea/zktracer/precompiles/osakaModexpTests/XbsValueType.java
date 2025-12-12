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
package net.consensys.linea.zktracer.precompiles.osakaModexpTests;

import static net.consensys.linea.zktracer.Trace.EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;

import java.util.List;
import java.util.stream.Stream;

public enum XbsValueType {
  UNCONDITIONALLY_VALID,
  CONDITIONALLY_VALID,
  INVALID;

  static final String GIBBERISH =
      "89f5c8bec8831d083e3563926d699a6cbcf09a6524770f3a3caf7e1b1c5d2a615edbca1cc12228b72b215bee100848a685854348b64be4ceb6921cc0860db363a0a0790ef02b669a7710ad3ba01133a683505705471f4a274f1c37cb4a8e216a45b25130388db08f80e42ecf5223f87f80c4b6616583ce29d250367a1151b7491c05af77d3e4cc72bfe72d052d8a577216457d914cdeee1c8898dd5da5be329e0f5fa11552b8e18fff1d90263f9d6192b7715b00c43d78d3127d216862b19c2f445b56a915ff2ccb30c90cbfa19e8b83e3bf4716d8b36359fdc2554ec93e5a74094c7eb4e4ae99a6258eff6f02c2bc064420a245cc07ffbe48d35e40dc51ddb6c099f60a78c99f1019c9713bf0377d8bda681e127916fe28c4955a607b0a294208ee5b15f83f196a2c524e3cd5d0927c3320d1ee1f82eec4fed1935cdf7b542ea24f22dbcb6fb46b6a7005285cdf372c8194e0e46167958e9e4b5c24f357e19aced1a4b4497400d7793e8b12ff5b0c7dc9bb80db6c67cf875dd34acb08bdf3fab04ba769f1b1afa45559d0135a43f7d5a3aaa0d98507602a10ae5af6a84e3e269c9ca33aeec0db21175e6d6578f000e2b3d81fabe3654fa1cbc233d858261c72c9a015f38b898191e14ae8a5b6609bd2711d0e6de5c1fd7938205c9ec5902deab73eff5b7629df6dc3538a67dd16a7db46748130b3b99f1279f9cac3780783551db705a0fe345bf2f07266ad6a91cdc2fb3ba293c3e411924736544ed9f82c9977b38af5dab5f55a69d9724e15a390cb1c95ef423fd329e95c5f5df25279b54a0adba4aa3cc500a83372aa10b601b148ae4eb47cfd411d9a25bd9776fc4caac3e8750aa6e30141ed890dcd95b1103e503c219954ae177eac08c12396adea872cf2406bb13c0a76ebbcd8db3604e874fc3a5d9203056c06d50d50b4e219654fe40a5bc8b0cd165744df101e4944df119e7c04411ecf575c2eb2c502a6fee31f4b357a0da84b0cde8f11d8b8ed51ef8b253447b60454fbcfb845ccea90ea993235f6e01288251d09d6e01be5e74e2ff8643d6515d65f228926d5ba3bb09d11a9f9c8c35b307ed1fcd6d9a04b997e51b9d742d812e71e0471f8fbd3a5559b68f39a1f8eb4d4796dd1ebc0c7af86355f14dd22d8ae92a372e39cdb2a0f5248b66af416d11ef558a3dc376b6356377bf7c6b885a33330d5e1040f8d459dd4af9dd68bdb86e65ece9609a42ddb536f969d28300cd856112a02ce0a719df9423d5d541a66f5c7c261271128a27812f5d643226427da6bd513fee16b12b60e89755f3d2b3f3d4c4ec032c62ad4686d775e7fac33f1c9f5e9d81fd6b2700ea9e72046b36f0529df5539f0c480e7be22e428f2681b87715e656b1cce33e055fa69b4331f4939f8c2ec277fd3dc5dcb738f4221639ed805cff4a7e862750a08299a02748284ef00b14aab01ea7b9e1abb6df750f82b46a12da0a9fb8ce71c7348f5bbf3bd22123c929a957f708d779a7a6124859d89a82e65002a972d918dec4d90903d4384fd49424ca1c5dbdb706975c606280614f41b5f6f7cb69bbca1b2096a0b1173edbb15a991aa05c12f60ae6fa83385283cac7134d08f25777f66ef3ef79a26916323544c768c6a0547f10ac55bc6afe65ca68f5902cc7177324dff1534ba433623bbdfa9163922eace311e4731c40dfaee1b39e201d8be1379062fa93f18ad93a01145cb613cd797c8ab6411a35d984419e65890578f2f2ae322d508b4752d43d547b574a5e4d2674981655fe831a087f7b63363bce6df82e4e05a067b7f10d6da745b600f5512d34b360345b4228c3c39ba367d5f98a8be5efa69b58f5a367ba2b45e9b05b006692922f9e732cdbc790c57d91c9f388965bf1661ebc672bbbb5f567942879958af43e0af3af4fdfb418ca5f380f9c33a20784414d8b342d65bfcc8348755ab775fca767916398d877d6f92b1a166c3628229be6e6bbc7518a0a8bb99d565b077195e2862dc7b8f9242974d87045168666fb1c45f0edc36a429fb9caca29f32ab94e0ead29dcdac237b8d6443df6f9347f70a361e06a3ad93a36f581f8a3c85d342294ec9b4827137fb1fe712c2bd5c7dd02c4315368e87d68f9d0ddccdbd6c5c912bd1dd969b6d4c25b2dd63256cd2a10b1421208aa439ee2d470242ec565d096e7b102e5e824877bbe282e3db78c4159c860e266e67de822bdc9f702bea509aa6760397b4f3d44c4ba0ee85efa213c60207db57546c8b25eee4ac89950f0035498538c4e8b59a21259510dd7bd1d6f2bc6e7dd9c0a6e5253acb0a6debfa42506a1e343484c1263500feb27d705f25618b6ec5009a33ff3b9fec45a030e2c93ec119b175ac45d2140022f70861fcd448b5c58402f6f5977c899dbf0e306f335b6ce1cd88ca23af858f40eebdd2e78f5630cc2d6b46ce33c3da3fcdc2dfd4235c6c79b44fc38b944a1e682a78726da859ed197989834c3f00a7af98def4e00a7415f5f1947906c7aad16dd9510f5266c3dd8e2527e12b7e216d65fbdb3789022410dae423641432db71cb5ef0e9affcaf0a4c6d281a54c6fc35d02283d54277dc4e4eb570e3e2a6848e7b61d14a6aa82da10e288d7139e3700c5ac4717c7d76fe76e9ce157bb475b4247d6b43049b7584de4524aa1b2a2fd7e250bba3c31f35a7005cb03c2a82b1bbed5cb1a3538011ae75cdc1ded0b00a89478ddb50abfad6475d102d671d5b88fd22dde31c666341fdaaac449ec8ee1158fec7ddae8ff580e8becdd705e8b644797904327f7d94dc542fbb36e3089d00c77677855e967f5bc1d02e48dac95c57935419a84a0b6f471f21cc48a58e6f0137e95dfb809f7edd0c9ce8b25fc469adfaf83dbd622d8a734312046a08c216718eec46284605d0a0cc12a94536f0998f0467c60ed8bbdfc90c1b02620061ab5baa62e14cdbf64c2f69b59c5845a55e08537c5ea0fa0c3d164a29fc35e2e7f6b0715301fb0c97bde2a638de9c7496e1e2e9d5fa735b8f028c71362295486e65c46297d19e020a8fd667ecd4ce0a2e50ad029c9a37a03516296d18437592641ed6a175a0e1f449f6afa2e2783661c9b1f2a9bfd27bc33c56a035215c6d680e5d7d72ccbd6def5759561492f391f81c3e462ec4362f0a88a9e0b5a69d65af53170278fded98a949f4b79d7dd4b188487ef53edd8235d8199234020d43818a5bd438468ff8e92b2e12cca751a0683de8cffb150190a61c01249f2d2dcfc50d985d318a3e9704c59842842d91b0a294dab265fc5c1fc461513a9dfaaae688fbb53e3f327bd91becd76bb1b859c9716f854650d388f7a9799a56fdbfcadcf8cd6de6c910e4d47b979bf64e3f8150b880865d8d052ef7cf96179796315cf771be3593df6cef82e3c55621f4947492cde5fa94d7c91d1435c3ae157532009f28d179a9072023ac098d793fc49a0a310fc94200857533fec56e23d35ccd253d41176b22662f5190d5a5493c4d1cf64e887be4298b8ff2f3e487c04ece4e991b98b52fa8e5a8294f7d1a4c5cdd0a1e03f1c9dc0b0aac02fc91d31041930220c23e9fdb8c0cc628361f6aed6f9b608e1a096218338fd70b90ff603d7ac1663c351469b0e73b519d99afbbab6cd5b22a51253638c6c84394891e2bed7216a5f72b9404e3db713e28f98049e3f7aaf54cbd7761126b00d2f07289e5c5fe304e2c400732784633c051a579f2af3f7d65fb47b53ee6965964bf4c36ada6b89623a43daee0adb6d0b06698a1cca88b7a297f556d7e65cf6f3023c62ebc95a01339a9df7aed4d5fac1b9fe693db67108d1e179f3ab49cca7bc31aa247e7b6c4d1971494fa3d5394f658a09fce6dbc76bcc160731abf603e9d03adf60d642558613cfc49c04f425b391bb8c3bb30a4fd12354e1989bb6d770f03d022fe402df650e701e53e6c66403c8e6efd9a0f7ff891b0fa7edb5b9523d903f1722223906d879516833c4e285c237a66aec7635f0dc269a5a840e00b400c6be5a709f4cf74955848ab607db46d5e2b83b6e1ef1f3ab1a3b82e6ce5baf34dc7af612d98b3b11c5873533b11a30a59feb1e73d8a7db76992fb601e52ca4e42438dffad37cd5da6699f219e5d03265443d10225784ba40aa293da0df2b0ddc8fe912aef2e8b106a9a09ed6a32d2f2dcff77a00f3bdec816b610d154b8c6dd341fb15b3bc8275a24a94e15c206c62e60848eb34563c74be772c5df6cf1a4243e742defd7314b1652f62ce8e4643e5d7cf849ad63c1e2e218eb19e6e38bcf0fbf3db73a63157aadc3628c2711891bd3f8f0f6b29f9d762deaf1867874413345bced165dbb8ed4274f055ddd4a5d8d6535646967";

  static final String VALID_XBS_ZERO =
      "0000000000000000000000000000000000000000000000000000000000000000";
  static final String VALID_XBS_ONE =
      "0000000000000000000000000000000000000000000000000000000000000001";
  static final String VALID_XBS_WORD =
      "0000000000000000000000000000000000000000000000000000000000000020";
  static final String VALID_XBS_RAND =
      "000000000000000000000000000000000000000000000000000000000000031e";
  static final String VALID_XBS_MAX =
      "0000000000000000000000000000000000000000000000000000000000000400";

  static final String CONDITIONALLY_VALID_XBS_MIN =
      "0000000000000000000000000000000000000000000000000000000000000401";
  static final String CONDITIONALLY_VALID_XBS_MAX =
      "00000000000000000000000000000000000000000000000000000000000004ff";

  static final String INVALID_LEAD_LIMB_ZERO = "00000000000000000000000000000000";
  static final String INVALID_LEAD_LIMB_RAND = "deadbeef00123400ffffffff00c0ffee";
  static final String INVALID_LEAD_LIMB_MAX = "ffffffffffffffffffffffffffffffff";

  static final String INVALID_TAIL_LIMB_ZERO = "00000000000000000000000000000000";
  static final String INVALID_TAIL_LIMB_401 = "00000000000000000000000000000401";
  static final String INVALID_TAIL_LIMB_500 = "00000000000000000000000000000500";
  static final String INVALID_TAIL_LIMB_RAND = "0000aa00ff000000beef000000004321";
  static final String INVALID_TAIL_LIMB_MAX = "ffffffffffffffffffffffffffffffff";

  /**
   * Byte sizes (<b>xbs</b>) are unconditionally valid in OSAKA if they are ≤ 1024 ≡ 400 after
   * trimming. The {@link #unconditionallyValidByteSizes} are those that are valid without resorting
   * to any trimming.
   */
  static final List<String> unconditionallyValidByteSizes =
      List.of(VALID_XBS_ZERO, VALID_XBS_ONE, VALID_XBS_WORD, VALID_XBS_RAND, VALID_XBS_MAX);

  /**
   * Byte sizes (<b>xbs</b>) are conditionally valid in OSAKA if they are ≤ 1024 ≡ 400 only after
   * trimming, which means we can accept byte sizes of the form <b>4??</b> as long as <b>cds</b> is
   *
   * <ul>
   *   <li>32 - 1 for {@link #CONDITIONALLY_VALID} bbs
   *   <li>64 - 1 for {@link #CONDITIONALLY_VALID} ebs
   *   <li>96 - 1 for {@link #CONDITIONALLY_VALID} mbs
   * </ul>
   *
   * <b>Note.</b> 1024 is <b>EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND</b>.
   */
  static final List<String> conditionallyValidByteSizes =
      List.of(CONDITIONALLY_VALID_XBS_MIN, CONDITIONALLY_VALID_XBS_MAX);

  static final List<String> headLimbsForInvalidXbses =
      List.of(INVALID_LEAD_LIMB_RAND, INVALID_LEAD_LIMB_MAX);

  static final List<String> tailLimbsForInvalidXbses =
      List.of(
          INVALID_TAIL_LIMB_ZERO,
          INVALID_TAIL_LIMB_401,
          INVALID_TAIL_LIMB_500,
          INVALID_TAIL_LIMB_RAND,
          INVALID_TAIL_LIMB_MAX);

  static final List<String> nonzeroTailLimbsForInvalidByteSizes =
      List.of(
          INVALID_TAIL_LIMB_401,
          INVALID_TAIL_LIMB_500,
          INVALID_TAIL_LIMB_RAND,
          INVALID_TAIL_LIMB_MAX);

  static final List<String> smallInvalidByteSizes =
      nonzeroTailLimbsForInvalidByteSizes.stream()
          .map(tail -> INVALID_LEAD_LIMB_ZERO + tail)
          .toList();
  static final List<String> largeInvalidByteSizes =
      headLimbsForInvalidXbses.stream()
          .flatMap(head -> tailLimbsForInvalidXbses.stream().map(tail -> head + tail))
          .toList();
  static final List<String> invalidByteSizes =
      Stream.concat(smallInvalidByteSizes.stream(), largeInvalidByteSizes.stream()).toList();

  static final List<String> getListOfInputs(XbsValueType xbsValueType) {

    return switch (xbsValueType) {
      case CONDITIONALLY_VALID -> conditionallyValidByteSizes;
      case UNCONDITIONALLY_VALID -> unconditionallyValidByteSizes;
      case INVALID -> invalidByteSizes;
    };
  }

  record BbsEbsMbsScenario(
      XbsValueType bbsValueType, XbsValueType ebsValueType, XbsValueType mbsValueType) {

    /**
     * {@link BbsEbsMbsScenario#callDataSize()} returns a very short call data size when one of the
     * <b>xbs</b> {@link XbsValueType} fields is only {@link XbsValueType#CONDITIONALLY_VALID}.
     *
     * @return
     */
    public int callDataSize() {

      if (bbsValueType == CONDITIONALLY_VALID) return WORD_SIZE - 1;
      if (ebsValueType == CONDITIONALLY_VALID) return 2 * WORD_SIZE - 1;
      if (mbsValueType == CONDITIONALLY_VALID) return 3 * WORD_SIZE - 1;

      return 3 * WORD_SIZE + 3 * EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND;
    }

    public String memoryContents(String bbs, String ebs, String mbs) {
      return bbs + ebs + mbs + GIBBERISH;
    }
  }
}
