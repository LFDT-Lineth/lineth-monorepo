import { z } from "zod";

// Zod schemas - validate required fields, strip extras (don't fail on extra fields)
export const RawDiscoursePostSchema = z.object({
  id: z.number(),
  username: z.string(),
  cooked: z.string(),
  post_url: z.string(),
  created_at: z.string(),
});

export const RawDiscourseProposalSchema = z.object({
  id: z.number(),
  slug: z.string(),
  title: z.string(),
  created_at: z.string(),
  post_stream: z.object({
    posts: z.array(RawDiscoursePostSchema),
  }),
});

export const RawDiscourseProposalListSchema = z.object({
  topic_list: z.object({
    topics: z.array(
      z.object({
        id: z.number(),
        slug: z.string(),
      }),
    ),
  }),
});

// TypeScript interfaces derived from schemas
export type RawDiscoursePost = z.infer<typeof RawDiscoursePostSchema>;
export type RawDiscourseProposal = z.infer<typeof RawDiscourseProposalSchema>;
export type RawDiscourseProposalList = z.infer<typeof RawDiscourseProposalListSchema>;
