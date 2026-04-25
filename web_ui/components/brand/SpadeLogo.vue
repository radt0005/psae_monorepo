<script setup lang="ts">
import { computed } from "vue";
/**
 * The SVG mark from ../website/templates/base.html lines 23-26 / 53-56.
 * Default 28×34 to match the marketing-site nav. Pass `dark` for the
 * white-on-dark footer variant.
 */
const props = withDefaults(
  defineProps<{
    width?: number | string;
    height?: number | string;
    dark?: boolean;
    title?: string;
  }>(),
  { width: 28, height: 34, dark: false, title: "Spade logo" },
);

const fill = computed(() => (props.dark ? "#ffffff" : "#1a1a1a"));
</script>

<template>
  <svg
    viewBox="0 0 100 120"
    :width="width"
    :height="height"
    role="img"
    :aria-label="title"
    xmlns="http://www.w3.org/2000/svg"
  >
    <path
      d="M50 10 C50 10, 15 45, 15 62 C15 78, 28 88, 42 82 C46 80, 49 76, 50 72 C51 76, 54 80, 58 82 C72 88, 85 78, 85 62 C85 45, 50 10, 50 10 Z"
      :fill="fill"
    />
    <path
      d="M42 82 C38 90, 35 98, 30 105 L70 105 C65 98, 62 90, 58 82 C54 86, 46 86, 42 82 Z"
      :fill="fill"
    />
  </svg>
</template>
