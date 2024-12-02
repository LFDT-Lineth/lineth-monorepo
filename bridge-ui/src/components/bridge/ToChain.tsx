import ToChainDropdown from "./dropdowns/ToChainDropdown";

export function ToChain() {
  return (
    <div className="mb-4 flex items-center justify-between">
      <span>To this network</span>
      <ToChainDropdown />
    </div>
  );
}
