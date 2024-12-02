import { BsDashLg } from "react-icons/bs";
import { MdInfo } from "react-icons/md";
import { Tooltip } from "@/components/ui";

interface FeeLineProps {
  label: string;
  value: string | undefined;
  tooltip?: string;
  tooltipClassName?: string;
}

export const FeeLine: React.FC<FeeLineProps> = ({ label, value, tooltip, tooltipClassName }) => (
  <div className="flex justify-between">
    <div className="flex items-center gap-2">
      <span>{label}:</span>
      {tooltip && (
        <Tooltip text={tooltip} className={tooltipClassName}>
          <MdInfo className="text-icon" />
        </Tooltip>
      )}
    </div>
    <span>{value ? value : <BsDashLg />}</span>
  </div>
);
