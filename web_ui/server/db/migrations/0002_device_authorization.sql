-- Better Auth device authorization plugin (RFC 8628).
-- Generated via: npx better-auth generate
-- Re-run that command after any plugin changes to get the canonical SQL.
CREATE TABLE "device_code" (
	"id" text PRIMARY KEY NOT NULL,
	"clientId" text NOT NULL,
	"userId" text,
	"deviceCode" text NOT NULL,
	"userCode" text NOT NULL,
	"status" text NOT NULL DEFAULT 'pending',
	"expiresAt" timestamp NOT NULL,
	"lastPolledAt" timestamp,
	"createdAt" timestamp NOT NULL,
	CONSTRAINT "device_code_deviceCode_unique" UNIQUE("deviceCode"),
	CONSTRAINT "device_code_userCode_unique" UNIQUE("userCode")
);
