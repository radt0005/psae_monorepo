import { S3Client } from "bun";

let cached: S3Client | null = null;

/**
 * Object storage client. Bun has S3 built in (`Bun.s3`); we configure
 * it once with the runtime config so dev (MinIO) and prod (S3) work
 * identically.
 */
export function useS3() {
  if (cached) return cached;
  const config = useRuntimeConfig();
  cached = new S3Client({
    endpoint: config.s3Endpoint as string,
    region: config.s3Region as string,
    accessKeyId: config.s3AccessKeyId as string,
    secretAccessKey: config.s3SecretAccessKey as string,
    bucket: config.s3Bucket as string,
  });
  return cached;
}

export function s3Bucket() {
  return useRuntimeConfig().s3Bucket as string;
}
