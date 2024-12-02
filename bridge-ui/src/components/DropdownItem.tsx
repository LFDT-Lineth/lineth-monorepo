import Image from "next/image";
import Link from "next/link";
import { MdCallMade } from "react-icons/md";
import { cn } from "@/utils/cn";

export type DropdownItemProps = {
  title: string;
  onClick?: () => void;
  iconPath?: string;
  externalLink?: string;
  className?: string;
};

export default function DropdownItem({ title, iconPath, onClick, externalLink, className }: DropdownItemProps) {
  if (externalLink) {
    return (
      <li key={`dropdown-item-${title}`} className="w-full">
        <Link
          href={externalLink}
          passHref
          target="_blank"
          rel="noopener noreferrer"
          className={cn(
            "btn btn-md flex justify-start font-normal rounded-none border-none bg-backgroundColor hover:bg-primary-light",
            className,
          )}
        >
          {iconPath && (
            <Image src={iconPath} alt={title} width={18} height={18} style={{ width: "18px", height: "auto" }} />
          )}
          {title}
          <MdCallMade />
        </Link>
      </li>
    );
  }

  return (
    <li key={`dropdown-item-${title}`} className="w-full">
      <button
        className={cn(
          "btn btn-md flex justify-start font-normal border-none bg-backgroundColor hover:bg-primary-light rounded-none",
          className,
        )}
        onClick={onClick}
      >
        {iconPath && (
          <Image src={iconPath} alt={title} width={18} height={18} style={{ width: "18px", height: "auto" }} />
        )}
        {title}
      </button>
    </li>
  );
}
