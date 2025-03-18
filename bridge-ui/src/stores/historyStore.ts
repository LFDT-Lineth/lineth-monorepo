import { createWithEqualityFn } from "zustand/traditional";
import { shallow } from "zustand/vanilla/shallow";
import { createJSONStorage, persist } from "zustand/middleware";
import { config } from "@/config";
import { isEmptyObject, BridgeTransaction } from "@/utils";

export type HistoryState = {
  isLoading: boolean;
  history: Record<
    string,
    { transactions: BridgeTransaction[]; lastL1FetchedBlockNumber: bigint; lastL2FetchedBlockNumber: bigint }
  >;
};

export type HistoryActions = {
  setIsLoading: (isLoading: boolean) => void;
  setTransactions: (
    key: string,
    transactions: BridgeTransaction[],
    lastL1FetchedBlockNumber?: bigint,
    lastL2FetchedBlockNumber?: bigint,
  ) => void;
  getTransactionsByKey: (key: string) => BridgeTransaction[];
  getFromBlockNumbers: (key: string) => { l1FromBlock: bigint; l2FromBlock: bigint };
};

export type HistoryStore = HistoryState & HistoryActions;

export const defaultInitState: HistoryState = {
  history: {},
  isLoading: false,
};

export const useHistoryStore = createWithEqualityFn<HistoryStore>()(
  persist(
    (set, get) => ({
      ...defaultInitState,
      setIsLoading: (isLoading) => set({ isLoading }),
      setTransactions: (key, transactions, lastL1FetchedBlockNumber, lastL2FetchedBlockNumber) =>
        set((state) => ({
          history: {
            ...state.history,
            [key]: {
              transactions,
              lastL1FetchedBlockNumber: lastL1FetchedBlockNumber || 0n,
              lastL2FetchedBlockNumber: lastL2FetchedBlockNumber || 0n,
            },
          },
        })),
      getTransactionsByKey: (key) => {
        const { history } = get();
        return history[key]?.transactions ?? [];
      },
      getFromBlockNumbers: (key) => {
        const { history } = get();

        if (isEmptyObject(history) || !history[key]) {
          return {
            l1FromBlock: 0n,
            l2FromBlock: 0n,
          };
        }

        return {
          l1FromBlock: history[key].lastL1FetchedBlockNumber,
          l2FromBlock: history[key].lastL2FetchedBlockNumber,
        };
      },
    }),
    {
      name: "history-storage",
      version: config.storage.minVersion,
      storage: createJSONStorage(() => localStorage, {
        replacer: (_, value) => {
          if (typeof value === "bigint") value = { __type: "bigint", value: value.toString() };
          if (value instanceof Map) value = { __type: "Map", value: Array.from(value.entries()) };
          return value;
        },
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        reviver: (_, value: any) => {
          if (value?.__type === "bigint") value = BigInt(value.value);
          if (value?.__type === "Map") value = new Map(value.value);
          return value;
        },
      }),
      migrate() {
        return defaultInitState;
      },
    },
  ),
  shallow,
);
