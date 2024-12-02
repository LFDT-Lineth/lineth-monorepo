import FromChainDropdown from "./dropdowns/FromChainDropdown";

export function FromChain() {
  return (
    <div className="mb-4 flex items-center justify-between">
      <span>From this network</span>
      <FromChainDropdown />
    </div>
  );
}
