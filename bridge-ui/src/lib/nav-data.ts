import { sanitizeAbsoluteHttpUrl } from "@/lib/urls";
import { LinkBlock } from "@/types";

function sanitizeNavUrl(url?: string): string | undefined {
  if (!url) {
    return undefined;
  }

  if (url.startsWith("/")) {
    return `https://linea.build${url}`;
  }

  return sanitizeAbsoluteHttpUrl(url);
}

export function formatNavData(data: LinkBlock[]) {
  const transform = (items: LinkBlock[]) =>
    items.reduce((acc: LinkBlock[], item: LinkBlock) => {
      const newItem = {
        ...item,
        url: sanitizeNavUrl(item.url),
        active: item.__id === "26O3hVvXLdQwEueLMdQ6Xj",
        ...(item.submenusLeft && {
          submenusLeft: transform(item.submenusLeft),
        }),
        ...(item.submenusRight && {
          submenusRight: transform([item.submenusRight])[0],
        }),
      };
      acc.push(newItem);
      return acc;
    }, []);

  return transform(data);
}

export async function getNavData() {
  try {
    const response = await fetch("https://linea.build/nav-data.json", {
      method: "GET",
      next: {
        tags: ["nav-data"],
        revalidate: 86400,
      },
    });

    if (!response.ok) {
      throw new Error(`Error fetching nav data: ${response.statusText}`);
    }

    const data = await response.json();
    return formatNavData(data);
  } catch (error) {
    console.error("Error fetching nav data:", error);
    throw error;
  }
}
