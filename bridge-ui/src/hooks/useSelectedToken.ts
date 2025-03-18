import { useTokenStore } from "@/stores";

const useSelectedToken = () => {
  return useTokenStore((state) => state.selectedToken);
};

export default useSelectedToken;
