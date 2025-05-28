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
package net.consensys.linea.extradata;

public class LineaExtraDataException extends RuntimeException {
  public enum ErrorType {
    INVALID_ARGUMENT(-32602),
    FAILED_CALLING_SET_MIN_GAS_PRICE(-32000),
    FAILED_CALLING_SET_EXTRA_DATA(-32000);

    private final int code;

    ErrorType(int code) {
      this.code = code;
    }

    public int getCode() {
      return code;
    }
  }

  private final ErrorType errorType;

  public LineaExtraDataException(final ErrorType errorType, final String message) {
    super(message);
    this.errorType = errorType;
  }

  public ErrorType getErrorType() {
    return errorType;
  }

  @Override
  public String toString() {
    return "errorType=" + errorType + ", message=" + getMessage();
  }
}
