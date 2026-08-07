package lineth.jsonrpc.argument;

import static lineth.jsonrpc.argument.TypeCast.safeCast;
import static lineth.jsonrpc.argument.TypeCast.safeCastOptional;
import static lineth.jsonrpc.argument.TypeCast.safeIntCast;
import static lineth.jsonrpc.argument.TypeCast.safeLongCast;
import static lineth.jsonrpc.argument.TypeCast.safeOptionalIntCast;
import static lineth.jsonrpc.argument.TypeCast.safeOptionalLongCast;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.assertj.core.api.Assertions;
import org.junit.jupiter.api.Test;

class TypeCastTest {
  static {
    Assertions.setMaxStackTraceElementsDisplayed(90);
  }

  @Test
  void testSafeIntCast() {
    assertThat(safeIntCast(Integer.valueOf(10), "accountId")).isEqualTo(10);

    assertThatThrownBy(() -> safeIntCast(null, "accountId"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("accountId");

    assertThatThrownBy(() -> safeIntCast(Long.valueOf(10), "accountId"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("accountId");

    assertThatThrownBy(() -> safeIntCast(new Object(), "accountId"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("accountId");

    assertThatThrownBy(() -> safeIntCast(Double.valueOf(0.0), "accountId"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("accountId");
  }

  @Test
  void testSafeOptionalIntCast() {
    assertThat(safeOptionalIntCast(Integer.valueOf(10), "accountId")).isEqualTo(10);

    assertThat(safeOptionalIntCast(null, "accountId")).isNull();

    assertThatThrownBy(() -> safeOptionalIntCast(Long.valueOf(10), "accountId"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("accountId");

    assertThatThrownBy(() -> safeOptionalIntCast(new Object(), "accountId"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("accountId");

    assertThatThrownBy(() -> safeOptionalIntCast(Double.valueOf(0.0), "accountId"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("accountId");
  }

  @Test
  void testSafeLongCast() {
    assertThat(safeLongCast(Integer.valueOf(10), "accountId")).isEqualTo(10L);
    assertThat(safeLongCast(Long.valueOf(10), "accountId")).isEqualTo(10L);

    assertThatThrownBy(() -> safeLongCast(new Object(), "accountId"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("accountId");

    assertThatThrownBy(() -> safeLongCast(Double.valueOf(0.0), "accountId"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("accountId");
  }

  @Test
  void testSafeOptionalLongCast() {
    assertThat(safeOptionalLongCast(Integer.valueOf(10), "accountId")).isEqualTo(10L);
    assertThat(safeOptionalLongCast(Long.valueOf(10), "accountId")).isEqualTo(10L);

    assertThat(safeOptionalLongCast(null, "accountId")).isNull();

    assertThatThrownBy(() -> safeOptionalLongCast(new Object(), "accountId"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("accountId");

    assertThatThrownBy(() -> safeOptionalLongCast(Double.valueOf(0.0), "accountId"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("accountId");
  }

  @Test
  void testSafeCastOptional_nullField() {
    assertThat(safeCastOptional(String.class, null, "address")).isNull();
  }

  @Test
  void testSafeCast_String() {
    assertThat(safeCast(String.class, "0xAAFF", "address")).isEqualTo("0xAAFF");
    assertThat(safeCastOptional(String.class, null, "address")).isNull();

    assertThatThrownBy(() -> safeCast(String.class, new String[] {"s1", "s2"}, "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");

    assertThatThrownBy(() -> safeCast(String.class, 10, "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");
  }

  @Test
  void testSafeCast_Integer() {
    assertThat(safeCast(Integer.class, 100, "address")).isEqualTo(100);
    assertThat(safeCastOptional(Integer.class, null, "address")).isNull();

    assertThatThrownBy(() -> safeCast(Integer.class, new Integer[] {1, 2}, "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");

    assertThatThrownBy(() -> safeCast(Integer.class, "10", "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");

    assertThatThrownBy(() -> safeCast(Integer.class, 1L, "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");

    assertThatThrownBy(() -> safeCast(Integer.class, 1.1, "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");
  }

  @Test
  void testSafeCast_Long() {
    assertThat(safeCast(Long.class, 100L, "address")).isEqualTo(100L);
    assertThat(safeCast(Long.class, 100, "address")).isEqualTo(100L);
    assertThat(safeCastOptional(Integer.class, null, "address")).isNull();

    assertThatThrownBy(() -> safeCast(Long.class, new Long[] {1L, 2L}, "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");

    assertThatThrownBy(() -> safeCast(Long.class, "10", "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");

    assertThatThrownBy(() -> safeCast(Long.class, 1.1, "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");
  }

  @Test
  void testSafeCast_Double() {
    assertThat(safeCast(Double.class, 10.2, "address")).isEqualTo(10.2);
    assertThat(safeCastOptional(Integer.class, null, "address")).isNull();

    assertThatThrownBy(() -> safeCast(Double.class, new Double[] {1.1, 2.2}, "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");

    assertThatThrownBy(() -> safeCast(Double.class, "10", "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");

    assertThatThrownBy(() -> safeCast(Double.class, 1, "address"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("address");
  }

  @Test
  @SuppressWarnings("unchecked")
  void testSafeCast_SuperTypes() {
    var arg0 = new LinkedHashMap<String, Object>();
    arg0.put("K1", "V1");
    arg0.put("K2", List.of("a", "b"));

    assertThat(safeCast(Map.class, arg0, "jsonObject"))
        .isEqualTo(Map.of("K1", "V1", "K2", List.of("a", "b")));

    assertThatThrownBy(() -> safeCast(ArrayList.class, arg0, "jsonObject"))
        .isInstanceOf(IllegalArgumentException.class)
        .hasMessageContaining("jsonObject");
  }
}
