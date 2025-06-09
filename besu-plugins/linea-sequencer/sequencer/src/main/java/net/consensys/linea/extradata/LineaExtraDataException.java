/*
 * Copyright Consensys Software Inc.
 *
 * This file is dual-licensed under either the MIT license or Apache License 2.0.
 * See the LICENSE-MIT and LICENSE-APACHE files in the repository root for details.
 *
 * SPDX-License-Identifier: MIT OR Apache-2.0
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
