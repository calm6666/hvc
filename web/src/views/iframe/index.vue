<template>
  <iframe :src="frameSrc" class="w-full h-full border-none" style="min-height: calc(100vh - 120px)" />
</template>
<script lang="ts" setup>
  import { computed } from 'vue';
  import { useRoute } from 'vue-router';
  const route = useRoute();
  const frameSrc = computed(() => (route.meta?.frameSrc as string) || '');
</script>
