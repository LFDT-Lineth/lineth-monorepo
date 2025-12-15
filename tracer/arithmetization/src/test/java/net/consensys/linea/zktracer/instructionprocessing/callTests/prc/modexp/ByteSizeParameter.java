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
 * Used to describe one of the three byte size parameters of a <b>MODEXP</b> call:
 *
 * <p>- <b>bbs</b>: base byte size
 *
 * <p>- <b>ebs</b>: exponent byte size
 *
 * <p>- <b>mbs</b>: modulus byte size
 */
public enum ByteSizeParameter {
  ZERO,
  ONE,
  /**
   * {@link #SHORT} stands for a {@link ByteSizeParameter} that is shorter than <b>32</b> bytes.
   * This is interesting in particular for the exponent byte size (<b>ebs</b>): pricing necessitates
   * extracting the leading word from the exponent, which is poses extra difficulties when the
   * exponent is {@link #SHORT}.
   */
  SHORT,
  MODERATE,
  MAX;
}
