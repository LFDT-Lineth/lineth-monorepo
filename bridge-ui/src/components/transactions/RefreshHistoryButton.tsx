import { Button } from "../ui";

export default function RefreshHistoryButton({
  fetchHistory,
  isLoading,
}: {
  fetchHistory: () => void;
  isLoading: boolean;
}) {
  return (
    <div className="flex justify-end">
      <Button
        id="reload-history-btn"
        variant="link"
        size="sm"
        className="font-normal normal-case text-secondary no-underline opacity-60 hover:text-secondary hover:opacity-100"
        onClick={fetchHistory}
      >
        Reload history
        {isLoading && <span className="loading loading-spinner loading-xs" />}
      </Button>
    </div>
  );
}
