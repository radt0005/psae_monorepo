import { validate as isUuid, v7 } from "uuid";
import amqplib from "amqplib";

export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig();
  const url = config.rabbitmqUrl as string;
  const queue = config.rabbitmqQueue as string;

  const body = await readBody(event);
  const userId: string | undefined = body.userId;
  const pipeline: string | undefined = body.pipeline;

  if (!userId || !isUuid(userId)) {
    throw createError({ statusCode: 400, statusMessage: "Invalid userId" });
  }
  if (!pipeline || typeof pipeline !== "string") {
    throw createError({
      statusCode: 400,
      statusMessage: "Missing pipeline YAML",
    });
  }

  const runId = v7();
  const message = { user_id: userId, run_id: runId, content: pipeline };

  const conn = await amqplib.connect(url);
  try {
    const channel = await conn.createChannel();
    await channel.assertQueue(queue, { durable: true });
    const success = channel.sendToQueue(
      queue,
      Buffer.from(JSON.stringify(message)),
      { persistent: true },
    );
    await channel.close();
    return { success, id: runId };
  } finally {
    await conn.close();
  }
});
