<script setup lang="ts">
import SchemaForm from '~/components/SchemaForm.vue';


const schema = JSON.parse(`{
  "name": "handler",
  "description": "Runs a SQL query against a file using DuckDB",
  "parameters": {
    "type": "object",
    "properties": {
      "path": {
        "type": "string",
        "description": "The path to the database (optional).  If none is provided or the path is invalid, an in-memony database will be used"
      },
      "query": { "type": "string", "description": "The SQL query provided" },
      "bindings": {
        "type": "array",
        "items": {},
        "description": "An optional list of variables to be bound into the query. Defaults to [].",
        "default": []
      }
    },
    "required": ["path", "query"]
  }
}`);
</script>

<template>
  <UCard>
    <UContainer>
      <h1>The PSAE App</h1>
      <p>A full explanation will go here</p>
    </UContainer>
  </UCard>
  </template>