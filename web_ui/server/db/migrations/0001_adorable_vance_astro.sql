CREATE TYPE "public"."run_status" AS ENUM('queued', 'running', 'succeeded', 'failed', 'canceled');--> statement-breakpoint
CREATE TABLE "run_files" (
	"id" text PRIMARY KEY NOT NULL,
	"run_id" text NOT NULL,
	"block_id" text,
	"name" text NOT NULL,
	"mime_type" text,
	"size_bytes" bigint,
	"s3_key" text NOT NULL,
	"created_at" timestamp DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "run_logs" (
	"id" text PRIMARY KEY NOT NULL,
	"run_id" text NOT NULL,
	"block_id" text,
	"stdout" text,
	"stderr" text,
	"created_at" timestamp DEFAULT now() NOT NULL
);
--> statement-breakpoint
CREATE TABLE "run_shares" (
	"run_id" text NOT NULL,
	"user_id" text NOT NULL,
	"permission" "share_permission" DEFAULT 'view' NOT NULL,
	"created_at" timestamp DEFAULT now() NOT NULL,
	CONSTRAINT "run_shares_run_id_user_id_pk" PRIMARY KEY("run_id","user_id")
);
--> statement-breakpoint
CREATE TABLE "runs" (
	"id" text PRIMARY KEY NOT NULL,
	"pipeline_id" text,
	"owner_id" text NOT NULL,
	"name" text,
	"yaml" text NOT NULL,
	"status" "run_status" DEFAULT 'queued' NOT NULL,
	"error" text,
	"visibility" "visibility" DEFAULT 'private' NOT NULL,
	"started_at" timestamp,
	"finished_at" timestamp,
	"created_at" timestamp DEFAULT now() NOT NULL,
	"updated_at" timestamp DEFAULT now() NOT NULL
);
--> statement-breakpoint
ALTER TABLE "run_files" ADD CONSTRAINT "run_files_run_id_runs_id_fk" FOREIGN KEY ("run_id") REFERENCES "public"."runs"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "run_logs" ADD CONSTRAINT "run_logs_run_id_runs_id_fk" FOREIGN KEY ("run_id") REFERENCES "public"."runs"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "run_shares" ADD CONSTRAINT "run_shares_run_id_runs_id_fk" FOREIGN KEY ("run_id") REFERENCES "public"."runs"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "run_shares" ADD CONSTRAINT "run_shares_user_id_user_id_fk" FOREIGN KEY ("user_id") REFERENCES "public"."user"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "runs" ADD CONSTRAINT "runs_pipeline_id_pipelines_id_fk" FOREIGN KEY ("pipeline_id") REFERENCES "public"."pipelines"("id") ON DELETE set null ON UPDATE no action;--> statement-breakpoint
ALTER TABLE "runs" ADD CONSTRAINT "runs_owner_id_user_id_fk" FOREIGN KEY ("owner_id") REFERENCES "public"."user"("id") ON DELETE cascade ON UPDATE no action;--> statement-breakpoint
CREATE INDEX "run_files_run_idx" ON "run_files" USING btree ("run_id");--> statement-breakpoint
CREATE INDEX "run_logs_run_idx" ON "run_logs" USING btree ("run_id");--> statement-breakpoint
CREATE INDEX "run_shares_user_idx" ON "run_shares" USING btree ("user_id");--> statement-breakpoint
CREATE INDEX "runs_owner_idx" ON "runs" USING btree ("owner_id");--> statement-breakpoint
CREATE INDEX "runs_pipeline_idx" ON "runs" USING btree ("pipeline_id");--> statement-breakpoint
CREATE INDEX "runs_status_idx" ON "runs" USING btree ("status");