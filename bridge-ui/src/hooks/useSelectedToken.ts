import { useTokenStore } from "@/stores/tokenStoreProvider";

const useSelectedToken = () => {
  return useTokenStore((state) => state.selectedToken);
};

export default useSelectedToken;
