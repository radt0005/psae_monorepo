// file: server/utils/rabbitmq.ts
import amqplib from 'amqplib';

const RABBITMQ_URL = process.env.RABBITMQ_URL || 'amqp://localhost';
const QUEUE_NAME = 'user_submissions';

let connection: amqplib.Connection | amqplib.ChannelModel | null = null;
let channel: amqplib.Channel | null = null;

export async function connectRabbitMQ() {
  if (!connection || !channel) {
    connection = await amqplib.connect(RABBITMQ_URL);
    channel = await connection.createChannel();
    await channel.assertQueue(QUEUE_NAME, { durable: true });
  }
  return channel!;
}

export async function publishJob(userId: string, runId: string, content: string | object) {
  const ch = await connectRabbitMQ();

  const message = {
    user_id: userId,
    run_id: runId,
    content
  };

  const sent = ch.sendToQueue(
    QUEUE_NAME,
    Buffer.from(JSON.stringify(message)),
    { persistent: true }
  );

  if (!sent) {
    throw new Error('Failed to enqueue job');
  }
}
