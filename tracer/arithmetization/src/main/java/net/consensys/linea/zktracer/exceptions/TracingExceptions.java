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

package net.consensys.linea.zktracer.exceptions;

import java.io.PrintStream;
import java.io.PrintWriter;
import java.io.StringWriter;
import java.util.List;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;

/** Gather all and any exception that happened during tracing under a common umbrella. */
@Getter
@RequiredArgsConstructor
@Slf4j
public class TracingExceptions extends RuntimeException {
  private final List<Exception> tracingExceptions;

  @Override
  public String getMessage() {
    final StringBuilder msg = new StringBuilder("Exceptions triggered while tracing:\n");
    for (final Exception e : tracingExceptions) {
      msg.append("  - ").append(e.getClass());
      if (e.getMessage() != null) {
        msg.append(": ").append(e.getMessage());
      }
      msg.append("\n");
    }
    log.error("First exception that was caught while tracing:", tracingExceptions.getFirst());
    return msg.toString();
  }

  @Override
  public void printStackTrace(PrintStream s) {
    for (final Exception e : this.tracingExceptions) {
      e.printStackTrace(s);
    }
  }

  @Override
  public String toString() {
    StringWriter stringWriter = new StringWriter();
    PrintWriter s = new PrintWriter(stringWriter);
    for (final Exception e : this.tracingExceptions) {
      s.append("\n");
      e.printStackTrace(s);
    }
    return stringWriter.toString();
  }
}
