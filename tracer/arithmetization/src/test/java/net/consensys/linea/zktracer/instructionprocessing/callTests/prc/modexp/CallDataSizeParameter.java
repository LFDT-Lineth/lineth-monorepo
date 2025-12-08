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

/**
 * {@link CallDataSizeParameter} represents different call data sizes for <b>CALL</b>'s to the
 * <b>MODEXP</b> precompile. The interpretation goes as follows:
 *
 * <p>- {@link #EMPTY} for empty call data (cds ≡ 0)
 *
 * <p>- {@link #BBS} for call data containing only <b>bbs</b> (cds ≡ 32)
 *
 * <p>- {@link #EBS} for call data containing everything up to and including <b>ebs</b> (cds ≡ 64)
 *
 * <p>- {@link #MBS} for call data containing everything up to and including <b>mbs</b> (cds ≡ 96)
 *
 * <p>- {@link #BASE} for call data containing the byte sizes and the <b>BASE</b> (cds ≡ 96 + bbs)
 *
 * <p>- {@link #EXPONENT} for call data containing everything up to the <b>EXPONENT</b> (cds ≡ 96 +
 * bbs + ebs)
 *
 * <p>- {@link #MODULUS_PART} for call data containing the byte sizes and parts of the
 * <b>MODULUS</b> (cds ≡ 96 + bbs + bbs + x) with 0 < x < mbs
 *
 * <p>- {@link #MODULUS_FULL} for well-formed call data <b>MODULUS</b> (cds ≡ 96 + bbs + bbs + mbs)
 *
 * <p>- {@link #LARGE} for call data larger than required (cds > 96 + bbs + bbs + mbs)
 */
public enum CallDataSizeParameter {
  EMPTY,
  BBS,
  EBS,
  MBS,
  BASE,
  EXPONENT,
  MODULUS_PART,
  MODULUS_FULL,
  LARGE;
}
