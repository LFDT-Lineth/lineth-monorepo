import { cn } from "@/utils/cn";
import Link from "next/link";
import { usePathname } from "next/navigation";

type MenuItemProps = {
  title: string;
  href: string;
  external: boolean;
  Icon: React.ComponentType<React.SVGProps<SVGSVGElement>>;
  toggleMenu?: () => void;
  border?: boolean;
};

export const MenuItem = ({ title, href, external, Icon, toggleMenu, border }: MenuItemProps) => {
  const pathname = usePathname();
  return (
    <li key={title}>
      <Link
        href={href}
        passHref={external}
        target={external ? "_blank" : undefined}
        rel={external ? "noopener noreferrer" : undefined}
        className={cn("flex items-center gap-2 py-3 font-normal text-lg", {
          "text-secondary": pathname === href,
          "border-r-2 border-secondary": pathname === href && border,
        })}
        onClick={toggleMenu}
      >
        <Icon width={30} height={30} />
        <span>{title}</span>
      </Link>
    </li>
  );
};
