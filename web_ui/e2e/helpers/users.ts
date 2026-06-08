/**
 * Test user factory. Emails are made unique per run so reruns against a
 * persistent dev database don't collide on the `user_email_unique` constraint.
 */
export interface TestUser {
  email: string;
  password: string;
  name: string;
}

let counter = 0;

export function makeUser(prefix = "e2e"): TestUser {
  counter += 1;
  // Date.now keeps it unique across runs; counter keeps it unique within a run.
  const stamp = `${Date.now().toString(36)}-${counter}`;
  return {
    email: `${prefix}+${stamp}@example.com`,
    password: "Sup3r-Secret-Pw!",
    name: `${prefix} ${stamp}`,
  };
}
