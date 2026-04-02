import { describe, it, expect } from "@jest/globals";

import { UnsupportedOperationError } from "../UnsupportedOperationError";

describe("UnsupportedOperationError", () => {
  it("should format message with operation only when no context is provided", () => {
    const error = new UnsupportedOperationError("someOperation");
    expect(error.name).toBe("UnsupportedOperationError");
    expect(error.message).toBe("someOperation is not supported");
  });

  it("should include context in message when context is provided", () => {
    const error = new UnsupportedOperationError("someOperation", "use alternativeMethod instead");
    expect(error.name).toBe("UnsupportedOperationError");
    expect(error.message).toBe("someOperation is not supported (use alternativeMethod instead)");
  });
});
