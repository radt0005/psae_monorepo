import { pgTable, text, timestamp, boolean } from "drizzle-orm/pg-core";

/**
 * Better Auth `user` table — column names match Better Auth's expected
 * schema so the Drizzle adapter works without overrides.
 */
export const users = pgTable("user", {
  id: text("id").primaryKey(),
  email: text("email").notNull().unique(),
  emailVerified: boolean("emailVerified").notNull().default(false),
  name: text("name"),
  image: text("image"),
  role: text("role").notNull().default("user"),
  createdAt: timestamp("createdAt").notNull().defaultNow(),
  updatedAt: timestamp("updatedAt").notNull().defaultNow(),
});

export type User = typeof users.$inferSelect;
export type NewUser = typeof users.$inferInsert;
