/**
 * Expands role definitions into `{ role, addressWithRole }` assignments. Generic over the role and address string
 * types so viem callers keep `Hex` / `Address` and the ethers-only scripts can keep plain strings.
 */
export function generateRoleAssignments<RoleT extends string, AddressT extends string>(
  roles: readonly RoleT[],
  defaultAddress: AddressT,
  overrides: { role: RoleT; addresses: AddressT[] }[],
): { role: RoleT; addressWithRole: AddressT }[] {
  const roleAssignments: { role: RoleT; addressWithRole: AddressT }[] = [];

  const overridesMap = new Map<RoleT, AddressT[]>();
  for (const override of overrides) {
    overridesMap.set(override.role, override.addresses);
  }

  const allRolesSet = new Set<RoleT>(roles);
  for (const override of overrides) {
    allRolesSet.add(override.role);
  }

  for (const role of allRolesSet) {
    if (overridesMap.has(role)) {
      const addresses = overridesMap.get(role);

      if (addresses && addresses.length > 0) {
        for (const addressWithRole of addresses) {
          roleAssignments.push({ role, addressWithRole });
        }
      }
    } else {
      roleAssignments.push({ role, addressWithRole: defaultAddress });
    }
  }

  return roleAssignments;
}
