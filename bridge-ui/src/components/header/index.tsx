import styles from "./header.module.scss";
import Link from "next/link";
import { DesktopNavigation } from "./desktop-navigation";
import { MobileNavigation } from "./mobile-navigation";
import LineaBridgeLogo from "@/assets/logos/linea-bridge.svg";
import { MENUS } from "./data";
import { Theme } from "@/types";

const Header = ({ theme = Theme.default }: { theme?: Theme }) => {
  return (
    <header className={`${styles.header} ${styles[theme]}`}>
      <div>
        <Link className={styles["link-to-home"]} href="/" aria-label="Go to homepage">
          <LineaBridgeLogo className={styles.logo} />
        </Link>
      </div>
      <MobileNavigation menus={MENUS} theme={theme} />
      <DesktopNavigation menus={MENUS} theme={theme} />
    </header>
  );
};

export default Header;
