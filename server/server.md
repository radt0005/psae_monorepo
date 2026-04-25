# Scheduler Server 

This folder should contain a scheduling server for the spade system.  

This system builds on the core Go library (`../core`).  Unlike the CLI (`../cli`), this uses the multiple pipeline scheduler for scheduling execution across multiple worker nodes. The worker is not yet implemented, but that will be next.  That will be created in `../runner` but that's currently an empty Go module.  

This system must communicate with the user interface for ingesting jobs, and with the worker nodes so they can complete the jobs. We will discuss the two connections in sequence

## Talking to the User Interface
This connection is based on the inbox/outbox pattern in the Postgresql database. We pair the outbox table with `LISTEN`/`NOTIFY` so the scheduler doesn't poll constantly. Web UI does `INSERT … NOTIFY scheduler_inbox`; scheduler runs `LISTEN scheduler_inbox` and pulls a batch only when notified.  This should be sufficient for the communication between the UI and scheduler. 

The reverse connection should follow a similar pattern, with the Web UI listening to the changes on the Job status.  

## Talking to the Workers

This should use RabbitMQ as a Work Queue. This gives us automatic retries and fair scheduling for free; the scheduler just has to take care of which block has to be next and push them into the queue.  This makes it much easier, but there should 