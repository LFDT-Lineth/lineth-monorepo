//! Root of the `check-r5` build step: exports the guest-shaped entry points so
//! that the dummy verifier is compiled for the R5 guest target.

const api = @import("api_surface.zig");

export fn dummy_verify_image(img: [*]u8, img_len: usize, encoded_base: usize) u8 {
    return api.verifyImage(img, img_len, encoded_base);
}

export fn dummy_verify_pair_image(img: [*]u8, img_len: usize, encoded_base: usize, out: *[api.statement_len]u8) u8 {
    return api.verifyPairImage(img, img_len, encoded_base, out);
}
