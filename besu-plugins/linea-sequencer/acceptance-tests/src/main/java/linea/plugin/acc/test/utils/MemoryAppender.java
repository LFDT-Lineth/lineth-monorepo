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
package linea.plugin.acc.test.utils;

import java.io.ByteArrayOutputStream;
import java.io.Serializable;
import java.nio.charset.StandardCharsets;
import lombok.SneakyThrows;
import org.apache.logging.log4j.core.Appender;
import org.apache.logging.log4j.core.Core;
import org.apache.logging.log4j.core.Layout;
import org.apache.logging.log4j.core.LogEvent;
import org.apache.logging.log4j.core.appender.AbstractAppender;
import org.apache.logging.log4j.core.config.Property;
import org.apache.logging.log4j.core.config.plugins.Plugin;
import org.apache.logging.log4j.core.config.plugins.PluginAttribute;
import org.apache.logging.log4j.core.config.plugins.PluginElement;
import org.apache.logging.log4j.core.config.plugins.PluginFactory;

@Plugin(name = "Memory", category = Core.CATEGORY_NAME, elementType = Appender.ELEMENT_TYPE)
public class MemoryAppender extends AbstractAppender {
  private static final ByteArrayOutputStream collectingOutput = new ByteArrayOutputStream();

  protected MemoryAppender(String name, Layout<? extends Serializable> layout) {
    super(name, null, layout, true, Property.EMPTY_ARRAY);
  }

  @PluginFactory
  public static MemoryAppender createAppender(
      @PluginAttribute("name") String name,
      @PluginElement("Layout") Layout<? extends Serializable> layout) {
    return new MemoryAppender(name, layout);
  }

  @SneakyThrows
  @Override
  public void append(LogEvent event) {
    collectingOutput.write(getLayout().toByteArray(event));
  }

  public static void reset() {
    collectingOutput.reset();
  }

  public static String getLog() {
    return collectingOutput.toString(StandardCharsets.UTF_8);
  }
}
