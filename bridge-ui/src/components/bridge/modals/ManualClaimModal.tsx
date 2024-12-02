import Link from "next/link";
import { Button } from "../../ui";
import { MdCallMade } from "react-icons/md";

type ManualClaimModalProps = {
  handleNoClose: () => void;
  handleYesClose: () => void;
};

const ManualClaimModal: React.FC<ManualClaimModalProps> = ({ handleNoClose, handleYesClose }) => {
  return (
    <div className="flex flex-col items-center justify-center gap-8 px-8">
      <h2 className="text-2xl">Manual claim on destination</h2>
      <div className="space-y-5 text-center">
        <div className="text-sm">
          Activating Manual claim means you will need to claim the message on the destination chain with ETH with a
          second transaction, once the first transaction has completed.
        </div>
        <div className="text-sm">Are you sure you want to enable manual claim?</div>
        <div className="flex items-center justify-center gap-4">
          <Button onClick={handleYesClose}>Yes</Button>
          <Button variant="outline" size="md" className="border-primary px-4" onClick={handleNoClose}>
            No
          </Button>
          <Link
            href="https://docs.linea.build/developers/guides/bridge/how-to-bridge-eth"
            rel="noopener noreferrer"
            target="_blank"
            className="hover:border-b-1 border-b-1 btn btn-ghost btn-sm rounded-none border-b-primary p-0 font-normal hover:border-b-primary hover:bg-transparent"
          >
            More Info
            <MdCallMade />
          </Link>
        </div>
      </div>
    </div>
  );
};

export default ManualClaimModal;
