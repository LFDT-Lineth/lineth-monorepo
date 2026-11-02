//! The KZG trusted-setup context of the guest crypto bindings. It lives in a test binary of its
//! own, so the context starts unbuilt: no point evaluation of another test has built it lazily.

const std = @import("std");
const gc = @import("guest_crypto");

test "the embedded trusted setup builds on request, ahead of the first point evaluation" {
    try std.testing.expect(!gc.kzgContextBuilt());
    try std.testing.expect(gc.initKzgContext());
    try std.testing.expect(gc.kzgContextBuilt());
}
