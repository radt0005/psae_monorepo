"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.afterCreate = afterCreate;
function afterCreate(request, response) {
    const webhookUrl = Env.get("RABBITMQ_WEBHOOK_URL");
    if (!webhookUrl) {
        console.log("Missing RABBITMQ_WEBHOOK_URL env variable.");
        return;
    }
    const payload = {
        run_id: response.record.id,
        user_id: request.authRecord ? request.authRecord.id : null,
        content: response.record.content,
    };
    fetch(webhookUrl, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
    });
}
