<template>
  <div class="node-box">
    <div class="node-header">
      {{ nodeName }}
    </div>
    <div class="node-body">
      <div v-if="pods.length === 0" class="no-pods">No pods running</div>
      <div v-for="pod in pods" :key="pod.name" class="pod-box">
        {{ pod.name }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  nodeName: string;
  pods: { name: string }[];
}>();
</script>

<style scoped>
.node-box {
  border: 2px solid #333;
  border-radius: 8px;
  width: 300px;
  margin: 10px;
  padding: 10px;
  background-color: #f8f9fa;
  box-shadow: 2px 2px 8px rgba(0, 0, 0, 0.2);
}

.node-header {
  font-weight: bold;
  background-color: #007bff;
  color: white;
  padding: 10px;
  text-align: center;
  border-radius: 4px;
  font-size: 1.2em;
}

.node-body {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  padding: 10px;
  justify-content: center;
}

.pod-box {
  background-color: #28a745;
  color: white;
  padding: 5px 10px;
  border-radius: 4px;
  text-align: center;
  flex: 1 1 40%;
  min-width: 80px;
  font-size: 0.9em;
}

.no-pods {
  font-size: 0.9em;
  color: #666;
  font-style: italic;
}
</style>
