declare const Env: {
    get: (key: string) => string | undefined;
  };


interface RecordModel {
    id: string
    collectionId: string
    collectionName: string
    created: string
    updated: string
    [key: string]: any
  }
  
  interface AuthRecord {
    id: string
    email: string
    [key: string]: any
  }
  
  interface RequestContext {
    authRecord?: AuthRecord
  }
  
  interface ResponseContext {
    record: RecordModel
  }

export function afterCreate(request: any, response: any) {
    const webhookUrl = Env.get("RABBITMQ_WEBHOOK_URL")
    if (!webhookUrl) {
      console.log("Missing RABBITMQ_WEBHOOK_URL env variable.")
      return
    }
  
    const payload = {
      run_id: response.record.id,
      user_id: request.authRecord ? request.authRecord.id : null,
      content: response.record.content,
    }
  
    fetch(webhookUrl, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    })
  }
  
