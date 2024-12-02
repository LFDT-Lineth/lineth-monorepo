import Link from "next/link";

type FooterLinksProps = {
  toggleMenu?: () => void;
};

export const FooterLinks = ({ toggleMenu }: FooterLinksProps) => (
  <div className="space-y-2 py-4">
    <Link
      className="flex items-center hover:text-secondary"
      href="https://support.linea.build/bridging/how-to-bridge-to-linea"
      passHref
      target="_blank"
      rel="noopener noreferrer"
      onClick={toggleMenu}
    >
      Contact Support
    </Link>
    <Link
      className="flex items-center hover:text-secondary"
      href="https://linea.build/privacy-policy"
      passHref
      target="_blank"
      rel="noopener noreferrer"
      onClick={toggleMenu}
    >
      Privacy Policy
    </Link>
    <Link
      className="flex items-center hover:text-secondary"
      href="https://linea.build/terms-of-service"
      passHref
      target="_blank"
      rel="noopener noreferrer"
      onClick={toggleMenu}
    >
      Terms of service
    </Link>
  </div>
);
