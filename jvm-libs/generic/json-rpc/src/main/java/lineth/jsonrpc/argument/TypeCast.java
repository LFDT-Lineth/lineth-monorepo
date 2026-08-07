package lineth.jsonrpc.argument;

public class TypeCast {

  public static int safeIntCast(Object value, String argumentName) {
    return safeCast(Integer.class, value, argumentName);
  }

  public static Integer safeOptionalIntCast(Object value, String argumentName) {
    return safeCastOptional(Integer.class, value, argumentName);
  }

  public static long safeLongCast(Object value, String argumentName) {
    return safeCast(Long.class, value, argumentName);
  }

  public static Long safeOptionalLongCast(Object value, String argumentName) {
    return safeCastOptional(Long.class, value, argumentName);
  }

  public static String safeStringCast(Object value, String argumentName) {
    return safeCast(String.class, value, argumentName);
  }

  public static String safeOptionalStringCast(Object value, String argumentName) {
    return safeCastOptional(String.class, value, argumentName);
  }

  public static <T> T safeCast(Class<T> clazz, Object value, String argumentName) {
    return safeCast(clazz, value, argumentName, false);
  }

  public static <T> T safeCastOptional(Class<T> clazz, Object value, String argumentName) {
    return safeCast(clazz, value, argumentName, true);
  }

  @SuppressWarnings("unchecked")
  public static <T> T safeCast(
      Class<T> clazz, Object value, String argumentName, boolean nullable) {
    if (value == null) {
      if (nullable) {
        return null;
      }
      throw new IllegalArgumentException("Required argument " + argumentName + " is null!");
    }

    if (value instanceof Integer && clazz == Long.class) {
      // caveat: allow Integers to be cast to Long.
      return (T) (Long.valueOf(((Integer) value).longValue()));
    }
    if (value instanceof Integer && clazz == int.class) {
      // caveat: allow Integers to be cast to int
      return (T) value;
    }
    if (value instanceof Integer && clazz == long.class) {
      // caveat: allow Integers to be cast to long
      return (T) value;
    }

    if (value instanceof Long && clazz == long.class) {
      // caveat: allow Long to be cast to long
      return (T) value;
    }

    if (!clazz.isAssignableFrom(value.getClass())) {
      throw buildCastError(argumentName, clazz, value.getClass());
    }

    try {
      return (T) value;
    } catch (ClassCastException e) {
      throw buildCastError(argumentName, clazz, value.getClass());
    }
  }

  private static <T, V> IllegalArgumentException buildCastError(
      String argumentName, Class<T> expectedClazz, Class<V> actualClazz) {
    return new IllegalArgumentException(
        String.format(
            "Invalid argument %s: expected type %s but got %s instead.",
            argumentName, expectedClazz.getName(), actualClazz.getName()));
  }
}
