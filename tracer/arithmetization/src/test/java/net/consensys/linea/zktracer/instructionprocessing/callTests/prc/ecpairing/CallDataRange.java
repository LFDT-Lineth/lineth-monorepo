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

import static com.google.common.base.Preconditions.checkArgument;

public class CallDataRange {

  private final int firstPoint;
  private final int finalPoint;
  public boolean isEmpty = false;

  public CallDataRange(int firstPoint, int finalPoint) {
    checkArgument(
        finalPoint >= firstPoint, "final point must be greater than or equal to first point");
    this.firstPoint = firstPoint;
    this.finalPoint = finalPoint;
  }

  public CallDataRange() {
    this.firstPoint = 0;
    this.finalPoint = 0;
    this.isEmpty = true;
  }

  public boolean isEmpty() {
    return isEmpty;
  }

  public int firstPoint() {
    return firstPoint;
  }

  public int finalPoint() {
    return finalPoint;
  }

  public int numberOfPairsOfPoints() {
    return isEmpty() ? 0 : (finalPoint() - firstPoint() + 1);
  }

  @Override
  public String toString() {
    return "{" + "first point=" + firstPoint + ", final point=" + finalPoint + '}';
  }
}
