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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc;

/**
 * {@link ReturnAtParameter} is an enum which describes the types of <b>return at</b> memory range.
 * For <b>MODEXP</b> the interpretation is as follows:
 *
 * <p>- {@link #EMPTY} is clear
 *
 * <p>- {@link #PARTIAL} represents a fraction of the mbs (modulus byte size)
 *
 * <p>- {@link #FULL} represents the whole return data range size (mbs)
 *
 * <p>- {@link #LARGE} is larger than mbs
 */
public enum ReturnAtParameter {
  EMPTY,
  PARTIAL,
  FULL,
  LARGE;
}
