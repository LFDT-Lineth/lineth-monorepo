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

package net.consensys.linea.zktracer.json;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.JsonSerializer;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.module.SimpleModule;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import lombok.Getter;
import lombok.SneakyThrows;

/** A wrapper class handling Jackson's {@link ObjectMapper} configuration. */
public class JsonConverter {

  @Getter private final ObjectMapper objectMapper;

  private final boolean prettyPrintEnabled;

  private JsonConverter(Builder builder) {
    this.objectMapper = builder.objectMapper;
    this.prettyPrintEnabled = builder.prettyPrintEnabled;
  }

  /**
   * Serializes an object to a JSON string.
   *
   * @param object the object to be serialized
   * @return a JSON string representing the object's data
   */
  @SneakyThrows(JsonProcessingException.class)
  public String toJson(Object object) {
    if (prettyPrintEnabled) {
      return objectMapper.writerWithDefaultPrettyPrinter().writeValueAsString(object);
    }

    return objectMapper.writeValueAsString(object);
  }

  /**
   * Deserializes a JSON string to a specified type.
   *
   * @param jsonString JSON string to be deserialized
   * @param clazz class type of the type for deserialization
   * @param <T> the deserialization type
   * @return an instance of the deserialized type
   */
  @SneakyThrows(JsonProcessingException.class)
  public <T> T fromJson(String jsonString, Class<T> clazz) {
    return objectMapper.readValue(jsonString, clazz);
  }

  /**
   * Deserializes a JSON string to a specified type.
   *
   * @param jsonString JSON string to be deserialized
   * @param clazz class type of the type for deserialization
   * @param <T> the deserialization type
   * @return an instance of the deserialized type
   */
  @SneakyThrows(JsonProcessingException.class)
  public <T> T fromJson(String jsonString) {
    return objectMapper.readValue(jsonString, new TypeReference<>() {});
  }

  /**
   * A factory for the {@link Builder} instance.
   *
   * @return an instance of {@link Builder}.
   */
  public static Builder builder() {
    return new Builder();
  }

  /** A builder for {@link JsonConverter}. */
  public static class Builder {
    private ObjectMapper objectMapper;
    private final SimpleModule module;
    private boolean prettyPrintEnabled;

    private Builder() {
      this.objectMapper = new ObjectMapper();
      this.module = new SimpleModule();
    }

    /**
     * Configures a custom jackson serializer for a specific type.
     *
     * @param type the class type targeted for custom serialization
     * @param serializer the serializer implementation handling the specified type
     * @param <T> the type targeted for custom serialization
     * @return the current instance of the builder
     */
    public <T> Builder addCustomSerializer(Class<T> type, JsonSerializer<T> serializer) {
      module.addSerializer(type, serializer);
      return this;
    }

    public Builder enableYaml() {
      objectMapper = new ObjectMapper(new YAMLFactory());
      return this;
    }

    public Builder enablePrettyPrint() {
      this.prettyPrintEnabled = true;
      return this;
    }

    /**
     * Builds an instance of {@link JsonConverter}.
     *
     * @return an instance of {@link JsonConverter}.
     */
    public JsonConverter build() {
      objectMapper
          .configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false)
          .registerModule(module);

      return new JsonConverter(this);
    }
  }
}
