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
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecpairing.MemoryContents.G2_POINT_5;

public enum LargePoint {
  // valid points
  INFINITY,
  VALID_LARGE_POINT,
  // invalid points
  // RAND,
  RE_X_NOT_IN_FIELD,
  IM_X_NOT_IN_FIELD,
  RE_Y_NOT_IN_FIELD,
  IM_Y_NOT_IN_FIELD,
  NOT_ON_CURVE,
  NOT_IN_SUBGROUP;

  public boolean isValid() {
    return this == INFINITY || this == VALID_LARGE_POINT;
  }

  public boolean isInfinity() {
    return this == INFINITY;
  }

  // various hex strings are taken from Ivo's comment
  //
  public String hexString() {
    final String result =
        switch (this) {
          case INFINITY -> LARGE_POINT_AT_INFINITY;
          case VALID_LARGE_POINT -> G2_POINT_5;
            // case RAND -> RANDOM_HEX_STRING.substring(31, 31 + 4 * WORD_HEX_SIZE);
          case RE_X_NOT_IN_FIELD -> IN_RANGE_SAVE_FOR_LARGE_RE_X;
          case IM_X_NOT_IN_FIELD -> IN_RANGE_SAVE_FOR_LARGE_IM_X;
          case RE_Y_NOT_IN_FIELD -> IN_RANGE_SAVE_FOR_LARGE_RE_Y;
          case IM_Y_NOT_IN_FIELD -> IN_RANGE_SAVE_FOR_LARGE_IM_Y;
          case NOT_ON_CURVE -> IN_RANGE_BUT_NOT_ON_CURVE;
          case NOT_IN_SUBGROUP -> ON_CURVE_BUT_NOT_ON_SUBGROUP;
          default -> throw new RuntimeException("Invalid point candidate");
        };
    checkState(result.length() == 4 * WORD_HEX_SIZE, "Invalid hex string length");

    return result;
  }

  public static final String LARGE_POINT_AT_INFINITY = "00".repeat(4 * WORD_SIZE);
  public static final String IN_RANGE_SAVE_FOR_LARGE_RE_X =
      "268294ac35af168840b758c749a19a7b1ffcb3df81e0047f2a4a3a68549fe8b9"
          + "facbb907b18555b9fa34efaa266ab946e1c7eadfe9d8f0cd821e7448e8a8053a"
          + "2fe08b1ff6a7fbe4cc3f0bfc309ae33bca35e9b29de0b2b3a73a829f3b9393cf"
          + "1b38210130d62850704dfeeee59e982308d9c976e583ad20a4289145cfcf3b1d";
  public static final String IN_RANGE_SAVE_FOR_LARGE_IM_X =
      "4dbf7d5e29252a3f871fe6ec7bbb736a82c6a92e1f383988fbf1993fd254535c"
          + "28747a6a24cce55f56d4847a08e45f2b19a575097915bfb39048be362f605ff8"
          + "031b86886f00782d83503db96caf1bb0ed6f68a907464fbb5b0de864b80c1ffd"
          + "302ca42cef622575199921b4f82e95009d09aba8e18c2240b9d084ac8bfd27de";
  public static final String IN_RANGE_SAVE_FOR_LARGE_RE_Y =
      "0fefff1654529948ddff9422011555b212236211766ce7674f9de0867248bec0"
          + "101dce57329a75b870c3ed82911561dddc38337d9de49c1a201e42097074d358"
          + "0b9a97e4531fb7c625ba34bf2939bc22a8ad8b2e03d0bd4cc9def5fdf96f30dc"
          + "5786a896704be805b5301789420715b9e6c4608d6bbf593b8c2bdc27b504eb91";
  public static final String IN_RANGE_SAVE_FOR_LARGE_IM_Y =
      "2ab1c49ae7628ad0ef94a9e181a47f7ebbf697173af1587ee152684b98b3b0d5"
          + "0efd3c694ab7d704b3f241402397b340a509ba863775d7b8ee1e8c99eddec530"
          + "5b2a592225a2ce715a205171f503ae70024f4842b4888407d67e0c580d3459d3"
          + "06433c581782ec0072e2185740924ac1fa2cf74905b40a901d3a7dfc7edb10e0";
  public static final String IN_RANGE_BUT_NOT_ON_CURVE =
      "1d3df5be6084324da6333a6ad1367091ca9fbceb70179ec484543a58b8cb5d63"
          + "119606e6d3ea97cea4eff54433f5c7dbc026b8d0670ddfbe6441e31225028d31"
          + "49fe60975e8c78b7b31a6ed16a338ac8b28cf6a065cfd2ca47e9402882518ba0"
          + "1b9a36ea373fe2c5b713557042ce6deb2907d34e12be595f9bbe84c144de86ef";
  public static final String ON_CURVE_BUT_NOT_ON_SUBGROUP =
      "15ce93f1b1c4946dd6cfbb3d287d9c9a1cdedb264bda7aada0844416d8a47a63"
          + "07192b9fd0e2a32e3e1caa8e59462b757326d48f641924e6a1d00d66478913eb"
          + "06e1f5e20f68f6dfa8a91a3bea048df66d9eaf56cc7f11215401f7e05027e0c6"
          + "0fa65a9b48ba018361ed081e3b9e958451de5d9e8ae0bd251833ebb4b2fafc96";
}
