import Image from "next/image";
import { MdOutlineClose } from "react-icons/md";
import { Button } from "../ui";
import { SocialLinks, FooterLinks } from "./footer";
import { Menu } from "./menu";

type MobileMenuProps = {
  toggleMenu: () => void;
};

export default function MobileMenu({ toggleMenu }: MobileMenuProps) {
  return (
    <div className="fixed inset-0 z-[200] flex flex-col bg-backgroundColor px-8 py-4 md:hidden">
      <div className="flex items-center justify-between">
        <Image
          src={"/images/logo/linea.svg"}
          alt="Linea logo"
          width={0}
          height={0}
          priority
          style={{ width: "auto", height: "auto" }}
        />
        <Button onClick={toggleMenu} className="btn-circle btn-ghost">
          <MdOutlineClose size="2em" />
        </Button>
      </div>
      <div className="mt-5 flex-1 overflow-y-auto">
        <Menu toggleMenu={toggleMenu} />
      </div>
      <div>
        <FooterLinks toggleMenu={toggleMenu} />
        <SocialLinks />
      </div>
    </div>
  );
}
