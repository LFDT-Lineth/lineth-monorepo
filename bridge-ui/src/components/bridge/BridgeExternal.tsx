import Link from "next/link";
import { MdArrowOutward } from "react-icons/md";

export function BridgeExternal() {
  return (
    <Link
      href="https://linea.build/apps?types=bridge"
      passHref
      target="_blank"
      rel="noopener noreferrer"
      className="flex items-center justify-center gap-2 p-2"
    >
      <span>Bridge using Third-Party bridges</span>
      <MdArrowOutward className="text-secondary" />
    </Link>
  );
}
