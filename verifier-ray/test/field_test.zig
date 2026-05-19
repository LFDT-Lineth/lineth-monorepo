const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;

test "koalabear element reduces and adds modulo the field" {
    const a = field.Element.init(field.modulus - 1);
    const b = field.Element.init(2);
    try std.testing.expect(a.add(b).eql(field.Element.one()));
}

test "koalabear element multiplication uses the field modulus" {
    const a = field.Element.init(field.modulus - 1);
    try std.testing.expect(a.mul(a).eql(field.Element.one()));
}

test "extension lift stores base element in the first limb" {
    const lifted = ext.Ext.lift(field.Element.init(17));
    try std.testing.expect(lifted.limbs[0].eql(field.Element.init(17)));
    try std.testing.expect(lifted.limbs[1].isZero());
    try std.testing.expect(lifted.limbs[2].isZero());
    try std.testing.expect(lifted.limbs[3].isZero());
}
