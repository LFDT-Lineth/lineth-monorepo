//! The KZG trusted-setup context of the guest crypto bindings. It lives in a test binary of its
//! own, so the context starts unbuilt.

const std = @import("std");
const gc = @import("guest_crypto");

test "the embedded trusted setup builds on request, ahead of its first use" {
    try std.testing.expect(!gc.kzgContextBuilt());
    try std.testing.expect(gc.initKzgContext());
    try std.testing.expect(gc.kzgContextBuilt());
}
