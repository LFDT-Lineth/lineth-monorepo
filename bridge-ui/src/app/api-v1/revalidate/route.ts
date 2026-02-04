import { revalidateTag } from "next/cache";

// This route performs revalidation and must be dynamic
export const dynamic = "force-dynamic";

interface RevalidateRequestBody {
  secret: string;
  tag: string;
}

export async function POST(req: Request): Promise<Response> {
  const body: RevalidateRequestBody = await req.json();
  const secret = process.env.REVALIDATE_SECRET;

  if (body.secret !== secret) {
    return new Response(JSON.stringify({ message: "Invalid token" }), {
      status: 401,
    });
  }

  const { tag } = body;
  if (!tag) {
    return new Response(JSON.stringify({ message: "Tag is required" }), {
      status: 400,
    });
  }

  try {
    const validTags = ["nav-data"];
    if (validTags.includes(tag)) {
      revalidateTag(tag, "max");
      return new Response(JSON.stringify({ message: `Revalidated tag: ${tag}` }), { status: 200 });
    } else {
      return new Response(JSON.stringify({ message: `Tag not recognized: ${tag}` }), { status: 400 });
    }
  } catch (err) {
    console.error("Error during revalidation:", err);
    return new Response(JSON.stringify({ message: "Failed to revalidate" }), {
      status: 500,
    });
  }
}
