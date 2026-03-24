<template>
    <vue-form-json-schema
      v-model="model"
      :schema="schema"
      :ui-schema="uiSchema"
    >
  </vue-form-json-schema>
  <UContainer>
    <pre>

        {{ schema }}
    </pre>
  </UContainer>
</template>

<script>
  export default {
    data() {
      return {
        // An object which holds the form values
        model: {},
        // A valid JSON Schema object
        schema: JSON.parse(`{
  "$schema": "http://json-schema.org/draft-04/schema#",
  "type": "object",
  "title": "Newsletter Subscription",
  "properties": {
    "name": {
      "type": "string",
      "minLength": 8,
      "maxLength": 80,
      "attrs": {
        "placeholder": "Full Name",
        "title": "Please enter your full name"
      }
    },
    "email": {
      "type": "string",
      "maxLength": 120,
      "attrs": {
        "type": "email",
        "placeholder": "Email"
      }
    },
    "lists": {
      "type": "string",
      "enum": ["Daily New", "Promotion"]
    }
  },
  "required": ["name", "email", "lists"]
}`),
        // Array of HTML elements or Vue components
        uiSchema: [],
      };
    }
  };
</script>