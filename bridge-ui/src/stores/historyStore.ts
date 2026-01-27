import { createJSONStorage, persist } from "zustand/middleware";
import { createWithEqualityFn } from "zustand/traditional";
import { shallow } from "zustand/vanilla/shallow";

import { config } from "@/config";
import { BridgeTransaction, TransactionStatus } from "@/types";
import { getCompleteTxStoreKeyForTx } from "@/utils/history";

export type HistoryState = {
  isLoading: boolean;
  completeTxHistory: Record<string, BridgeTransaction>;
};

type HistoryActions = {
  setIsLoading: (isLoading: boolean) => void;
  setCompleteTx: (transaction: BridgeTransaction) => void;
  getCompleteTx: (key: string) => BridgeTransaction | undefined;
  deleteCompleteTx: (key: string) => void;
};

export type HistoryActionsForCompleteTxCaching = Pick<
  HistoryActions,
  "setCompleteTx" | "getCompleteTx" | "deleteCompleteTx"
>;

export type HistoryStore = HistoryState & HistoryActions;

export const defaultInitState: HistoryState = {
  isLoading: false,
  // history: {},
  completeTxHistory: {},
};

export const useHistoryStore = createWithEqualityFn<HistoryStore>()(
  persist(
    (set, get) => ({
      ...defaultInitState,
      setIsLoading: (isLoading) => set({ isLoading }),
      setCompleteTx: (transaction) =>
        set((state) => {
          if (transaction.status !== TransactionStatus.COMPLETED) return state;
          const key = getCompleteTxStoreKeyForTx(transaction);
          return {
            completeTxHistory: {
              ...state.completeTxHistory,
              [key]: transaction,
            },
          };
        }),
      getCompleteTx: (key) => {
        const { completeTxHistory } = get();
        return completeTxHistory[key];
      },
      deleteCompleteTx: (key) =>
        set((state) => {
          // eslint-disable-next-line @typescript-eslint/no-unused-vars
          const { [key]: _, ...remainingHistory } = state.completeTxHistory;
          return {
            completeTxHistory: remainingHistory,
          };
        }),
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
