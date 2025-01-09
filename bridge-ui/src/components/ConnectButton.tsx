import { useAppKit } from "@reown/appkit/react";
import { cn } from "@/utils/cn";
import { Button } from "./ui";

type ConnectButtonProps = {
  fullWidth?: boolean;
};

export default function ConnectButton({ fullWidth }: ConnectButtonProps) {
  const { open } = useAppKit();
  return (
    <Button
      id="wallet-connect-btn"
      variant="primary"
      size="md"
      className={cn("text-lg font-normal", {
        "w-full": fullWidth,
      })}
      onClick={() => open()}
    >
      Connect Wallet
    </Button>
  );
}
